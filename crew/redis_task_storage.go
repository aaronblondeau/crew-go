package crew

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/google/uuid"
	goredislib "github.com/redis/go-redis/v9"
)

// RedisTaskStorage stores tasks in the filesystem as JSON files.
type RedisTaskStorage struct {
	Client  *goredislib.Client
	RedSync *redsync.Redsync
}

// NewRedisTaskStorage creates a new RedisTaskStorage.
func NewRedisTaskStorage(Addr string, Password string, DB int) *RedisTaskStorage {
	client := goredislib.NewClient(&goredislib.Options{
		Addr:     Addr,
		Password: Password,
		DB:       DB,
	})
	pingCmd := client.Ping(context.Background())
	pingErr := pingCmd.Err()
	if pingErr != nil {
		panic(pingErr)
	}

	pool := goredis.NewPool(client)
	rs := redsync.New(pool)

	storage := RedisTaskStorage{
		Client:  client,
		RedSync: rs,
	}
	return &storage
}

// TaskKey returns the key for a task.
func (storage *RedisTaskStorage) TaskKey(taskId string) string {
	return "go-crew/tasks/" + taskId
}

// TaskMutexKey returns the key for a task's lock.
func (storage *RedisTaskStorage) TaskMutexKey(taskId string) string {
	return "go-crew/tasks/" + taskId + "/mutex"
}

// TaskGroupKey returns the key for a task group.
func (storage *RedisTaskStorage) TaskGroupKey(taskGroupId string) string {
	return storage.TaskGroupsPrefix() + taskGroupId
}

func (storage *RedisTaskStorage) TaskGroupsPrefix() string {
	// IMPORTANT - tasks and groups should have differing root paths so that we can SCAN groups without getting tasks
	return "go-crew/task-groups/"
}

func (storage *RedisTaskStorage) GetExpiration() time.Duration {
	taskExpiration := time.Duration(0)

	// Disabling this for now, tasks are more cleanly removed from db with the Delete method
	// taskExpirationEnv := os.Getenv("CREW_TASK_EXPIRATION")
	// if taskExpirationEnv != "" {
	// 	taskExpirationEnvParsed, taskExpirationErr := time.ParseDuration(taskExpirationEnv)
	// 	if taskExpirationErr == nil {
	// 		taskExpiration = taskExpirationEnvParsed
	// 	}
	// }

	return taskExpiration
}

func (storage *RedisTaskStorage) GetLockExpiration() time.Duration {
	taskLockExpiration := time.Duration(10 * time.Minute)

	taskLockExpirationEnv := os.Getenv("CREW_TASK_LOCK_EXPIRATION")
	if taskLockExpirationEnv != "" {
		taskLockExpirationEnvParsed, taskLockExpirationErr := time.ParseDuration(taskLockExpirationEnv)
		if taskLockExpirationErr == nil {
			taskLockExpiration = taskLockExpirationEnvParsed
		}
	}

	return taskLockExpiration
}

// SaveTask saves a task.
func (storage *RedisTaskStorage) SaveTask(task *Task, create bool) (err error) {
	if task.Id == "" {
		task.Id = uuid.New().String()
	}
	key := storage.TaskKey(task.Id)

	taskJson, jsonErr := json.Marshal(task)
	if jsonErr != nil {
		return jsonErr
	}
	taskJsonStr := string(taskJson)

	canWrite := true
	if !create {
		// If !create then we are doing a save of a task that should exist.
		// If it doesn't exist then we shouldn't write it because it was deleted.
		exists, existsError := storage.FindTask(task.Id)
		if (exists == nil) || (existsError != nil) {
			canWrite = false
		}
	}

	if canWrite {
		redisSetErr := storage.Client.Set(context.Background(), key, taskJsonStr, storage.GetExpiration()).Err()
		if redisSetErr != nil {
			return redisSetErr
		}

		if create {
			// Add task to taskGroup index
			tasksIdxErr := storage.Client.LPush(context.Background(), storage.TaskGroupKey(task.TaskGroupId)+"/tasks", task.Id).Err()
			if tasksIdxErr != nil {
				return tasksIdxErr
			}

			// Add task to task key index
			if task.Key != "" {
				tasksKeyIdxErr := storage.Client.LPush(context.Background(), "go-crew/keys/"+task.Key, task.Id).Err()
				if tasksKeyIdxErr != nil {
					return tasksKeyIdxErr
				}
			}

			// Add task to workgroup index
			if task.Workgroup != "" {
				tasksWorkgroupIdxErr := storage.Client.LPush(context.Background(), "go-crew/workgroups/"+task.Workgroup, task.Id).Err()
				if tasksWorkgroupIdxErr != nil {
					return tasksWorkgroupIdxErr
				}
			}

			// Add task to parent's children list
			if len(task.ParentIds) > 0 {
				for _, parentId := range task.ParentIds {
					tasksParentIdxErr := storage.Client.LPush(context.Background(), storage.TaskKey(parentId)+"/children", task.Id).Err()
					if tasksParentIdxErr != nil {
						return tasksParentIdxErr
					}
				}
			}
		}
		return nil
	} else {
		return errors.New("cannot overwrite existing task")
	}
}

func (storage *RedisTaskStorage) FindTaskAtPath(path string) (task *Task, err error) {
	taskData, readTaskErr := storage.Client.Get(context.Background(), path).Bytes()
	if readTaskErr != nil {
		return nil, readTaskErr
	}
	inflatedTask := NewTask()
	taskParseError := json.Unmarshal(taskData, &inflatedTask)
	if taskParseError != nil {
		return nil, taskParseError
	}
	return inflatedTask, nil
}

// FindTask finds a task by task group id and task id.
func (storage *RedisTaskStorage) FindTask(taskId string) (task *Task, err error) {
	key := storage.TaskKey(taskId)
	return storage.FindTaskAtPath(key)
}

func (storage *RedisTaskStorage) TryLockTask(taskId string) (unlocker func() error, err error) {
	// TODO - make lock duration configurable
	mux := storage.RedSync.NewMutex(storage.TaskMutexKey(taskId), redsync.WithExpiry(storage.GetLockExpiration()))
	err = mux.Lock()
	if err != nil {
		return nil, err
	}

	unlocker = func() error {
		_, unlockErr := mux.Unlock()
		return unlockErr
	}
	return
}

// Delete task deletes a task by task id.
func (storage *RedisTaskStorage) DeleteTask(taskId string) (err error) {
	key := storage.TaskKey(taskId)

	task, findErr := storage.FindTask(taskId)
	if findErr != nil {
		return findErr
	}

	// Remove from task group index
	storage.Client.LRem(context.Background(), storage.TaskGroupKey(task.TaskGroupId)+"/tasks", 0, task.Id)

	// Remove from task key index
	if task.Key != "" {
		storage.Client.LRem(context.Background(), "go-crew/keys/"+task.Key, 0, task.Id)
	}
	// NOTE, redis removes empty lists automatically

	// Remove from workgroup index
	if task.Workgroup != "" {
		storage.Client.LRem(context.Background(), "go-crew/workgroups/"+task.Workgroup, 0, task.Id)
	}
	// NOTE, redis removes empty lists automatically

	// Remove from parents' children list
	if len(task.ParentIds) > 0 {
		for _, parentId := range task.ParentIds {
			storage.Client.LRem(context.Background(), storage.TaskKey(parentId)+"/children", 0, task.Id)
		}
	}

	// Remove own children list
	storage.Client.Del(context.Background(), key+"/children")

	// Remove task itself
	redisErr := storage.Client.Del(context.Background(), key).Err()

	return redisErr
}

// SaveTaskGroup doesn't do anything for memory storage.
func (storage *RedisTaskStorage) SaveTaskGroup(taskGroup *TaskGroup, create bool) (err error) {
	if taskGroup.Id == "" {
		taskGroup.Id = uuid.New().String()
	}

	groupJson, jsonErr := json.Marshal(taskGroup)
	if jsonErr != nil {
		return jsonErr
	}
	groupJsonStr := string(groupJson)

	key := storage.TaskGroupKey(taskGroup.Id)

	redisErr := storage.Client.Set(context.Background(), key, groupJsonStr, storage.GetExpiration()).Err()
	return redisErr
}

func (storage *RedisTaskStorage) FindTaskGroupAtPath(path string) (taskGroup *TaskGroup, err error) {
	taskGroupData, readTaskErr := storage.Client.Get(context.Background(), path).Bytes()
	if readTaskErr != nil {
		return nil, readTaskErr
	}
	inflatedTaskGroup := NewTaskGroup("", "")
	taskGroupParseError := json.Unmarshal(taskGroupData, &inflatedTaskGroup)
	if taskGroupParseError != nil {
		return nil, taskGroupParseError
	}
	return inflatedTaskGroup, nil
}

// FindTaskGroup finds a task group by task group id.
func (storage *RedisTaskStorage) FindTaskGroup(taskGroupId string) (taskGroup *TaskGroup, err error) {
	key := storage.TaskGroupKey(taskGroupId)
	return storage.FindTaskGroupAtPath(key)
}

// All TaskGroups returns all task groups.
func (storage *RedisTaskStorage) AllTaskGroups() (taskGroups []*TaskGroup, err error) {
	ctx := context.Background()
	iter := storage.Client.Scan(ctx, 0, storage.TaskGroupsPrefix()+"*", 0).Iterator()
	taskGroups = make([]*TaskGroup, 0)

	for iter.Next(ctx) {
		path := iter.Val()
		// Ignore if path ends in /tasks
		if !strings.HasSuffix(path, "/tasks") {
			taskGroup, taskGroupErr := storage.FindTaskGroupAtPath(path)
			if taskGroupErr != nil {
				return nil, taskGroupErr
			}

			taskGroups = append(taskGroups, taskGroup)
		}
	}
	return
}

func (storage *RedisTaskStorage) AllTasksInList(path string) (tasks []*Task, err error) {
	// Get all tasks ids from the group list
	taskIds, taskIdsErr := storage.Client.LRange(context.Background(), path, 0, -1).Result()
	tasks = make([]*Task, 0)
	if taskIdsErr == nil {
		for _, taskId := range taskIds {
			task, taskErr := storage.FindTask(taskId)
			if taskErr == nil {
				tasks = append(tasks, task)
			}
		}
	}
	return
}

// All AllTasksInGroup returns all tasks within a group.
func (storage *RedisTaskStorage) AllTasksInGroup(taskGroupId string) (tasks []*Task, err error) {
	return storage.AllTasksInList(storage.TaskGroupKey(taskGroupId) + "/tasks")
}

// GetTaskChildren returns the children of a task.
func (storage *RedisTaskStorage) GetTaskChildren(taskId string) (tasks []*Task, err error) {
	return storage.AllTasksInList(storage.TaskKey(taskId) + "/children")
}

// GetTaskParents returns the parents of a task.
func (storage *RedisTaskStorage) GetTaskParents(taskId string) (tasks []*Task, err error) {
	task, taskErr := storage.FindTask(taskId)
	if taskErr != nil {
		return nil, taskErr
	}
	tasks = make([]*Task, 0)
	if len(task.ParentIds) > 0 {
		for _, taskId := range task.ParentIds {
			task, taskErr := storage.FindTask(taskId)
			if taskErr == nil {
				tasks = append(tasks, task)
			}
		}
		return tasks, nil
	}
	return
}

func (storage *RedisTaskStorage) GetTasksInWorkgroup(workgroup string) (tasks []*Task, err error) {
	return storage.AllTasksInList("go-crew/workgroups/" + workgroup)
}

func (storage *RedisTaskStorage) GetTasksWithKey(key string) (tasks []*Task, err error) {
	return storage.AllTasksInList("go-crew/keys/" + key)
}

// DeleteTaskGroup deletes a task group by task group id.
func (storage *RedisTaskStorage) DeleteTaskGroup(taskGroupId string) (err error) {
	// Delete all tasks in group
	groupTasks, groupTasksErr := storage.AllTasksInGroup(taskGroupId)
	if groupTasksErr != nil {
		return groupTasksErr
	}
	for _, task := range groupTasks {
		storage.DeleteTask(task.Id)
	}

	// Delete tasks list
	storage.Client.Del(context.Background(), storage.TaskGroupKey(taskGroupId)+"/tasks")

	// Delete group itself
	storage.Client.Del(context.Background(), storage.TaskGroupKey(taskGroupId))

	return nil
}
