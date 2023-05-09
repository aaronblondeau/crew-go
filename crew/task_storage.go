package crew

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
)

// TaskStorage defines the methods required for implementing crew's task storage interface.
type TaskStorage interface {
	SaveTask(task *Task) (err error)
	DeleteTask(task *Task) (err error)
	SaveTaskGroup(group *TaskGroup) (err error)
	DeleteTaskGroup(group *TaskGroup) (err error)
	Bootstrap(shouldOperate bool, client TaskClient, throttler *Throttler) (pool *TaskPool, err error)
}

// Redis Storage

// RedisTaskStorage stores tasks in the filesystem as JSON files.
type RedisTaskStorage struct {
	Pool   map[string]*Task
	Client *redis.Client
}

// TaskKey returns the key for a task.
func (storage *RedisTaskStorage) TaskKey(task *Task) string {
	// IMPORTANT - tasks and groups should have differing root paths so that we can SCAN groups without getting tasks
	return "go-crew/group-tasks/" + task.Address
}

// TaskGroupKey returns the key for a task group.
func (storage *RedisTaskStorage) TaskGroupKey(group *TaskGroup) string {
	// IMPORTANT - tasks and groups should have differing root paths so that we can SCAN groups without getting tasks
	return "go-crew/groups/" + group.Address
}

// TaskGroupTasksPrefix returns the SCAN prefix to use to search for all tasks within a group.
func (storage *RedisTaskStorage) TaskGroupTasksPrefix(group *TaskGroup) string {
	return "go-crew/group-tasks/" + group.Address + "*"
}

// TaskGroupsPrefix returns the SCAN prefix to use to search for all groups.
func (storage *RedisTaskStorage) TaskGroupsPrefix() string {
	return "go-crew/groups/*"
}

// SaveTask saves a task to redis.
func (storage *RedisTaskStorage) SaveTask(task *Task) (err error) {
	if task.Operator.IsDeleting {
		// Avoid re-creating a task that is getting deleted
		return nil
	}
	taskJson, jsonErr := json.Marshal(task)
	if jsonErr != nil {
		return jsonErr
	}
	taskJsonStr := string(taskJson)

	ctx := context.Background()
	key := storage.TaskKey(task)
	redisErr := storage.Client.Set(ctx, key, taskJsonStr, 0).Err()
	return redisErr
}

// DeleteTask deletes a task from redis.
func (storage *RedisTaskStorage) DeleteTask(task *Task) (err error) {
	key := storage.TaskKey(task)
	ctx := context.Background()
	redisErr := storage.Client.Del(ctx, key).Err()
	return redisErr
}

// SaveTaskGroup saves a task group to redis.
func (storage *RedisTaskStorage) SaveTaskGroup(group *TaskGroup) (err error) {
	if group.IsDeleting {
		// Avoid re-creating a group that is getting deleted
		return nil
	}

	groupJson, jsonErr := json.Marshal(group)
	if jsonErr != nil {
		return jsonErr
	}
	groupJsonStr := string(groupJson)

	ctx := context.Background()
	key := storage.TaskGroupKey(group)
	redisErr := storage.Client.Set(ctx, key, groupJsonStr, 0).Err()
	return redisErr
}

// DeleteTaskGroup deletes a task group from redis.
func (storage *RedisTaskStorage) DeleteTaskGroup(group *TaskGroup) (err error) {
	key := storage.TaskGroupKey(group)
	ctx := context.Background()
	// Delete the group
	redisErr := storage.Client.Del(ctx, key).Err()
	if redisErr != nil {
		return redisErr
	}

	// Delete all tasks in the group
	iter := storage.Client.Scan(ctx, 0, storage.TaskGroupTasksPrefix(group), 0).Iterator()
	for iter.Next(ctx) {
		taskKey := iter.Val()
		fmt.Println("~~ Deleting task group child task", taskKey)
		redisErr = storage.Client.Del(ctx, taskKey).Err()
		if redisErr != nil {
			return redisErr
		}
	}
	return nil
}

// Bootstrap loads all task groups and tasks from the filesystem.
func (storage *RedisTaskStorage) Bootstrap(shouldOperate bool, client TaskClient, throttler *Throttler) (pool *TaskPool, err error) {
	pool = NewTaskPool(storage, throttler)

	ctx := context.Background()
	iter := storage.Client.Scan(ctx, 0, storage.TaskGroupsPrefix(), 0).Iterator()

	for iter.Next(ctx) {
		// Load each group
		groupKey := iter.Val()
		fmt.Println("~~ Loading group", groupKey)

		groupData, readGroupErr := storage.Client.Get(ctx, groupKey).Bytes()
		if readGroupErr != nil {
			fmt.Println("~~ Skipping group - failed to read group key", readGroupErr)
			continue
		}

		group := NewTaskGroup("", "")
		groupParseError := json.Unmarshal(groupData, &group)
		if groupParseError != nil {
			fmt.Println("~~ Skipping group - failed to parse group value", groupParseError)
			continue
		}
		pool.CreateGroup(group)

		// Load each group's tasks
		tasksIter := storage.Client.Scan(ctx, 0, storage.TaskGroupTasksPrefix(group), 0).Iterator()
		for tasksIter.Next(ctx) {
			taskKey := tasksIter.Val()
			fmt.Println("~~ Loading task", groupKey)

			taskData, readTaskErr := storage.Client.Get(ctx, taskKey).Bytes()
			if readTaskErr != nil {
				fmt.Println("~~ Skipping task - failed to read task key", readTaskErr)
				continue
			}

			task := Task{}
			taskParseError := json.Unmarshal(taskData, &task)
			if taskParseError != nil {
				fmt.Println("~~ Skipping task - failed to parse task value", taskKey, taskParseError)
				continue
			}
			pool.CreateTask(&task)
		}
	}

	// TODO RFTODO - build all children and parents after all tasks are loaded
	pool.AlignTasks()
	pool.Bootstrapping = false
	pool.Operate()

	return
}

// NewRedisTaskStorage creates a new RedisTaskStorage.
func NewRedisTaskStorage(Addr string, Password string, DB int) *RedisTaskStorage {
	client := redis.NewClient(&redis.Options{
		Addr:     Addr,
		Password: Password,
		DB:       DB,
	})
	pingCmd := client.Ping(context.Background())
	pingErr := pingCmd.Err()
	if pingErr != nil {
		panic(pingErr)
	}
	storage := RedisTaskStorage{
		Client: client,
	}
	return &storage
}

// // Filesystem Storage (JSON)

// JsonFilesystemTaskStorage stores tasks in the filesystem as JSON files.
type JsonFilesystemTaskStorage struct {
	BasePath string
}

// TaskPath returns the path to a task's JSON file.
func (storage *JsonFilesystemTaskStorage) TaskPath(task *Task) string {
	return storage.BasePath + "/tasks/" + task.Address + ".json"
}

// TaskGroupPath returns the path to a task group's JSON file.
func (storage *JsonFilesystemTaskStorage) TaskGroupPath(group *TaskGroup) string {
	return storage.BasePath + "/task_groups/" + group.Address + ".json"
}

// SaveTask saves a task to the filesystem.
func (storage *JsonFilesystemTaskStorage) SaveTask(task *Task) (err error) {
	if task.Operator.IsDeleting {
		// Avoid re-creating a task that is getting deleted
		return nil
	}

	// Make sure tasks dir exists
	groupDir := storage.BasePath + "/tasks/"
	if _, err := os.Stat(groupDir); os.IsNotExist(err) {
		os.MkdirAll(groupDir, os.ModeDir)
	}

	taskJson, jsonErr := json.Marshal(task)
	if jsonErr != nil {
		return jsonErr
	}
	taskBytes := []byte(taskJson)
	writeErr := os.WriteFile(storage.TaskPath(task), taskBytes, 0644)
	return writeErr
}

// DeleteTask deletes a task from the filesystem.
func (storage *JsonFilesystemTaskStorage) DeleteTask(task *Task) (err error) {
	filePath := storage.TaskPath(task)
	_, statError := os.Stat(filePath)
	if statError != nil {
		// Stat error => file didn't exist
		return nil
	}
	removeError := os.Remove(filePath)
	if removeError != nil {
		fmt.Println("JsonFilesystemTaskStorage.DeleteTask Error", removeError, filePath)
	}
	return removeError
}

// SaveTaskGroup saves a task group to the filesystem.
func (storage *JsonFilesystemTaskStorage) SaveTaskGroup(group *TaskGroup) (err error) {
	if group.IsDeleting {
		// Avoid re-creating a group that is getting deleted
		return nil
	}
	// Make sure task groups dir exists
	groupDir := storage.BasePath + "/task_groups/"
	if _, err := os.Stat(groupDir); os.IsNotExist(err) {
		os.MkdirAll(groupDir, os.ModeDir)
	}

	groupJson, jsonErr := json.Marshal(group)
	if jsonErr != nil {
		return jsonErr
	}
	groupBytes := []byte(groupJson)
	writeErr := os.WriteFile(storage.TaskGroupPath(group), groupBytes, 0644)
	return writeErr
}

// DeleteTaskGroup deletes a task group from the filesystem.
func (storage *JsonFilesystemTaskStorage) DeleteTaskGroup(group *TaskGroup) (err error) {
	filePath := storage.TaskGroupPath(group)
	_, statError := os.Stat(filePath)
	if statError != nil {
		// Stat error => file didn't exist
		return nil
	}
	removeError := os.Remove(filePath)
	if removeError != nil {
		fmt.Println("JsonFilesystemTaskStorage.DeleteTaskGroup Error", removeError, filePath)
	}
	return removeError
}

// Bootstrap loads all task groups and tasks from the filesystem.
func (storage *JsonFilesystemTaskStorage) Bootstrap(shouldOperate bool, client TaskClient, throttler *Throttler) (pool *TaskPool, err error) {
	pool = NewTaskPool(storage, throttler)

	entries, readDirError := os.ReadDir(storage.BasePath + "/task_groups")
	if readDirError != nil {
		return pool, readDirError
	}

	for _, groupEntry := range entries {
		if groupEntry.IsDir() {
			// Look for dir/group.json
			groupDir := storage.BasePath + "/task_groups/" + groupEntry.Name()
			fmt.Println("~~ Bootstrap reading group dir", groupDir)
			groupJsonPath := groupDir + "/group.json"
			_, groupFileExistError := os.Stat(groupJsonPath)
			if !os.IsNotExist(groupFileExistError) {
				// group.json exists => this is a task group directory

				groupData, readGroupErr := os.ReadFile(groupJsonPath)
				if readGroupErr != nil {
					fmt.Println("~~ Skipping group - failed to read group.json", readGroupErr)
					continue
				}

				group := NewTaskGroup("", "")
				groupParseError := json.Unmarshal(groupData, &group)
				if groupParseError != nil {
					fmt.Println("~~ Skipping group - failed to parse group.json", groupParseError)
					continue
				}
				pool.CreateGroup(group)

				taskEntries, readGroupDirError := os.ReadDir(groupDir)
				if readGroupDirError != nil {
					fmt.Println("~~ Failed to scan for for tasks", readGroupDirError)
					continue
				}

				for _, taskEntry := range taskEntries {
					// Make sure entry is a file that ends with .json
					if !taskEntry.IsDir() && taskEntry.Name() != "group.json" && strings.HasSuffix(taskEntry.Name(), ".json") {
						taskFilePath := groupDir + "/" + taskEntry.Name()
						fmt.Println("~~ Bootstrap reading task file", taskFilePath)
						taskData, taskDataErr := os.ReadFile(taskFilePath)
						if taskDataErr != nil {
							fmt.Println("~~ Skipping task - failed to read .json", taskFilePath, taskDataErr)
							continue
						}

						task := Task{
							// Errors:    make([]interface{}, 0),
							// ParentIds: make([]string, 0),
							// Children:  make([]*Task, 0),
						}
						taskParseError := json.Unmarshal(taskData, &task)
						// Make sure group id matches the group being loaded
						if taskParseError != nil {
							fmt.Println("~~ Skipping task - failed to parse .json", taskFilePath, taskParseError)
							continue
						}
						pool.CreateTask(&task)
					}
				}
			} else {
				fmt.Println("~~ Cannot find group.json")
			}
		}
	}

	// TODO RFTODO - build all children and parents after all tasks are loaded
	pool.AlignTasks()
	pool.Bootstrapping = false
	pool.Operate()

	return
}

// NewJsonFilesystemTaskStorage creates a new JsonFilesystemTaskStorage.
func NewJsonFilesystemTaskStorage(basePath string) *JsonFilesystemTaskStorage {
	storage := JsonFilesystemTaskStorage{
		BasePath: basePath,
	}
	return &storage
}

// Memory Storage

// MemoryTaskStorage is a task storage that does not persist tasks. This is meant for use in tests.
type MemoryTaskStorage struct {
}

// SaveTask does nothing.
func (storage *MemoryTaskStorage) SaveTask(task *Task) (err error) {
	// Do nothing
	return nil
}

// DeleteTask does nothing.
func (storage *MemoryTaskStorage) DeleteTask(task *Task) (err error) {
	// Do nothing
	return nil
}

// SaveTaskGroup does nothing.
func (storage *MemoryTaskStorage) SaveTaskGroup(group *TaskGroup) (err error) {
	// Do nothing
	return nil
}

// DeleteTaskGroup does nothing.
func (storage *MemoryTaskStorage) DeleteTaskGroup(group *TaskGroup) (err error) {
	// Do nothing
	return nil
}

// Bootstrap creates an empty task group controller.
func (storage *MemoryTaskStorage) Bootstrap(shouldOperate bool, client TaskClient, throttler *Throttler) (taskPool *TaskPool, err error) {
	pool := NewTaskPool(storage, throttler)
	return pool, nil
}

// NewMemoryTaskStorage creates a new MemoryTaskStorage.
func NewMemoryTaskStorage() *MemoryTaskStorage {
	storage := MemoryTaskStorage{}
	return &storage
}
