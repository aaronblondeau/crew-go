package crew

// TODO - cascading deletes of children? (only when deleting child's last parent)
// TODO - prevent changes to workgroup, key, taskGroupId in task

import (
	"errors"
	"sync"

	"github.com/google/uuid"
)

// TaskStorage defines the methods required for implementing crew's task storage interface.
type TaskStorage interface {
	SaveTask(task *Task) (err error)
	FindTask(taskId string) (task *Task, err error)
	DeleteTask(taskId string) (err error)
	GetTaskChildren(taskId string) (tasks []*Task, err error)
	GetTaskParents(taskId string) (tasks []*Task, err error)
	GetTasksInWorkgroup(workgroup string) (tasks []*Task, err error)
	GetTasksWithKey(key string) (tasks []*Task, err error)

	SaveTaskGroup(taskGroup *TaskGroup) (err error)
	AllTaskGroups() (taskGroups []*TaskGroup)
	AllTasksInGroup(taskGroupId string) (tasks []*Task)
	FindTaskGroup(taskGroupId string) (taskGroup *TaskGroup, err error)
	DeleteTaskGroup(taskGroupId string) (err error)
}

// MemoryTaskStorage is a task storage that only stores state in memory.
type MemoryTaskStorage struct {
	taskGroups    map[string]*TaskGroup
	tasks         map[string]*Task
	idxWorkgroups map[string][]*Task
	idxKeys       map[string][]*Task
	idxGroups     map[string][]*Task
	Mutex         sync.RWMutex
}

// NewMemoryTaskStorage creates a new MemoryTaskStorage.
func NewMemoryTaskStorage() *MemoryTaskStorage {
	storage := MemoryTaskStorage{
		taskGroups:    make(map[string]*TaskGroup),
		tasks:         make(map[string]*Task),
		idxWorkgroups: make(map[string][]*Task),
		idxKeys:       make(map[string][]*Task),
		idxGroups:     make(map[string][]*Task),
	}
	return &storage
}

// SaveTask saves a task.
func (storage *MemoryTaskStorage) SaveTask(task *Task) (err error) {
	storage.Mutex.Lock()
	defer storage.Mutex.Unlock()
	if task.Id == "" {
		task.Id = uuid.New().String()
	}
	_, exists := storage.tasks[task.Id]
	if !exists {
		storage.tasks[task.Id] = task

		// Add to indexes
		if _, idxWorkgroupsExists := storage.idxWorkgroups[task.Workgroup]; !idxWorkgroupsExists {
			storage.idxWorkgroups[task.Workgroup] = make([]*Task, 0)
		}
		storage.idxWorkgroups[task.Workgroup] = append(storage.idxWorkgroups[task.Workgroup], task)

		if _, idxKeysExists := storage.idxKeys[task.Key]; !idxKeysExists {
			storage.idxKeys[task.Key] = make([]*Task, 0)
		}
		storage.idxKeys[task.Key] = append(storage.idxKeys[task.Key], task)

		if _, idxGroupsExists := storage.idxGroups[task.TaskGroupId]; !idxGroupsExists {
			storage.idxGroups[task.TaskGroupId] = make([]*Task, 0)
		}
		storage.idxGroups[task.TaskGroupId] = append(storage.idxGroups[task.TaskGroupId], task)
	}
	// Nothing to do for memory storage if already exists
	return nil
}

// FindTask finds a task by task group id and task id.
func (storage *MemoryTaskStorage) FindTask(taskId string) (task *Task, err error) {
	storage.Mutex.RLock()
	defer storage.Mutex.RUnlock()
	task, found := storage.tasks[taskId]
	if !found {
		return nil, errors.New("task not found")
	}
	return task, nil
}

// Delete task deletes a task by task id.
func (storage *MemoryTaskStorage) DeleteTask(taskId string) (err error) {
	storage.Mutex.Lock()
	defer storage.Mutex.Unlock()

	task, found := storage.tasks[taskId]
	if found {
		// Remove from workgroups index
		idxWorkgroup, idxWorkgroupFound := storage.idxWorkgroups[task.Workgroup]
		if idxWorkgroupFound {
			for i, task := range idxWorkgroup {
				if task.Id == taskId {
					storage.idxWorkgroups[task.Workgroup] = append(idxWorkgroup[:i], idxWorkgroup[i+1:]...)
					break
				}
			}
		}
		// If was last item in workgroups index, remove the index
		if len(idxWorkgroup) == 0 {
			delete(storage.idxWorkgroups, task.Workgroup)
		}

		// Remove from keys index
		idxKey, idxKeyFound := storage.idxKeys[task.Key]
		if idxKeyFound {
			for i, task := range idxKey {
				if task.Id == taskId {
					storage.idxKeys[task.Key] = append(idxKey[:i], idxKey[i+1:]...)
					break
				}
			}
		}
		// If was last item in keys index, remove the index
		if len(idxKey) == 0 {
			delete(storage.idxKeys, task.Key)
		}

		// Remove from groups index
		idxGroup, idxGroupFound := storage.idxGroups[task.TaskGroupId]
		if idxGroupFound {
			for i, task := range idxGroup {
				if task.Id == taskId {
					storage.idxGroups[task.TaskGroupId] = append(idxGroup[:i], idxGroup[i+1:]...)
					break
				}
			}
		}
		// If was last item in groups index, remove the index
		if len(idxGroup) == 0 {
			delete(storage.idxGroups, task.TaskGroupId)
		}

		// Remove from tasks
		delete(storage.tasks, taskId)
	}
	return nil
}

// SaveTaskGroup doesn't do anything for memory storage.
func (storage *MemoryTaskStorage) SaveTaskGroup(taskGroup *TaskGroup) (err error) {
	storage.Mutex.Lock()
	defer storage.Mutex.Unlock()
	if taskGroup.Id == "" {
		taskGroup.Id = uuid.New().String()
	}
	_, exists := storage.taskGroups[taskGroup.Id]
	if !exists {
		storage.taskGroups[taskGroup.Id] = taskGroup
	}
	return nil
}

// FindTaskGroup finds a task group by task group id.
func (storage *MemoryTaskStorage) FindTaskGroup(taskGroupId string) (taskGroup *TaskGroup, err error) {
	storage.Mutex.RLock()
	defer storage.Mutex.RUnlock()
	taskGroup, found := storage.taskGroups[taskGroupId]
	if !found {
		return nil, errors.New("task group not found")
	}
	return taskGroup, nil
}

// All TaskGroups returns all task groups.
func (storage *MemoryTaskStorage) AllTaskGroups() (taskGroups []*TaskGroup) {
	storage.Mutex.RLock()
	defer storage.Mutex.RUnlock()
	for _, taskGroup := range storage.taskGroups {
		taskGroups = append(taskGroups, taskGroup)
	}
	return taskGroups
}

// All TaskGroups returns all task groups.
func (storage *MemoryTaskStorage) AllTasksInGroup(taskGroupId string) (tasks []*Task) {
	storage.Mutex.RLock()
	defer storage.Mutex.RUnlock()
	tasks = make([]*Task, 0)
	groupTasks, groupTasksFound := storage.idxGroups[taskGroupId]
	if groupTasksFound {
		for _, task := range groupTasks {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// GetTaskChildren returns the children of a task.
func (storage *MemoryTaskStorage) GetTaskChildren(taskId string) (tasks []*Task, err error) {
	storage.Mutex.RLock()
	defer storage.Mutex.RUnlock()

	children := make([]*Task, 0)
	for _, task := range storage.tasks {
		for _, parentId := range task.ParentIds {
			if parentId == taskId {
				children = append(children, task)
			}
		}
	}
	return children, nil
}

// GetTaskParents returns the parents of a task.
func (storage *MemoryTaskStorage) GetTaskParents(taskId string) (tasks []*Task, err error) {
	storage.Mutex.RLock()
	defer storage.Mutex.RUnlock()

	parents := make([]*Task, 0)
	task, findError := storage.FindTask(taskId)
	if findError != nil {
		return parents, findError
	}

	for _, parentId := range task.ParentIds {
		parent, parentFindError := storage.FindTask(parentId)
		if parentFindError == nil {
			parents = append(parents, parent)
		}
	}

	return parents, nil
}

func (storage *MemoryTaskStorage) GetTasksInWorkgroup(workgroup string) (tasks []*Task, err error) {
	storage.Mutex.RLock()
	defer storage.Mutex.RUnlock()

	tasks = make([]*Task, 0)
	idxWorkgroup, idxWorkgroupFound := storage.idxWorkgroups[workgroup]
	if idxWorkgroupFound {
		tasks = idxWorkgroup
	}
	return tasks, nil
}

func (storage *MemoryTaskStorage) GetTasksWithKey(key string) (tasks []*Task, err error) {
	storage.Mutex.RLock()
	defer storage.Mutex.RUnlock()

	tasks = make([]*Task, 0)
	idxKey, idxKeyFound := storage.idxKeys[key]
	if idxKeyFound {
		tasks = idxKey
	}
	return tasks, nil
}

// DeleteTaskGroup deletes a task group by task group id.
func (storage *MemoryTaskStorage) DeleteTaskGroup(taskGroupId string) (err error) {
	// Get all tasks in the group (in own lock)
	storage.Mutex.RLock()
	tasks := make([]*Task, 0)
	groupTasks, groupTasksFound := storage.idxGroups[taskGroupId]
	if groupTasksFound {
		tasks = groupTasks
	}
	storage.Mutex.RUnlock()

	// Delete all tasks in the group
	for _, task := range tasks {
		storage.DeleteTask(task.Id)
	}

	// Delete the task group
	storage.Mutex.Lock()
	defer storage.Mutex.Unlock()

	delete(storage.taskGroups, taskGroupId)
	delete(storage.idxGroups, taskGroupId)
	return nil
}
