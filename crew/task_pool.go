package crew

import (
	"errors"
	"strings"
	"sync"
	"time"
)

// TaskUpdateEvent defines the data emitted when a task is updated.
type TaskUpdateEvent struct {
	Event string `json:"type"`
	Task  Task   `json:"task"`
}

// A TaskGroupUpdateEvent notifies listeners of changes to a TaskGroup's state.
type TaskGroupUpdateEvent struct {
	Event     string    `json:"type"`
	TaskGroup TaskGroup `json:"task_group"`
}

// A ThrottlePushQuery is a request to the throttler to see if there is enough bandwidth for a worker to run.
type ThrottlePushQuery struct {
	TaskAddress string
	Worker      string
	Resp        chan bool
}

// ThrottlePopQuery is a request to the throttler to notify that a worker is done.
type ThrottlePopQuery struct {
	TaskAddress string
	Worker      string
}

type Throttler struct {
	Push chan ThrottlePushQuery
	Pop  chan ThrottlePopQuery
}

type WorkgroupDelay struct {
	Workgroup      string
	DelayInSeconds int
}

// A TaskPool combines a set of task groups with a storage mechanism and event notification channels.
type TaskPool struct {
	Tasks       map[string]*Task
	TasksMutex  sync.RWMutex
	Groups      map[string]*TaskGroup
	GroupsMutex sync.RWMutex
	// This is for sending updates to UI, all group and task create/update/delete events should get sent here:
	TaskUpdates      chan TaskUpdateEvent
	TaskGroupUpdates chan TaskGroupUpdateEvent
	TaskChildren     chan Task
	TaskCompletions  chan Task
	DelayWorkgroup   chan WorkgroupDelay
	Storage          TaskStorage
	Throttler        *Throttler
	Bootstrapping    bool
}

// NewTaskPool creates a new TaskPool.
func NewTaskPool(storage TaskStorage, throttler *Throttler) *TaskPool {
	op := TaskPool{
		Tasks:            make(map[string]*Task),
		TasksMutex:       sync.RWMutex{},
		Groups:           make(map[string]*TaskGroup),
		GroupsMutex:      sync.RWMutex{},
		TaskUpdates:      make(chan TaskUpdateEvent, 8),
		TaskGroupUpdates: make(chan TaskGroupUpdateEvent, 8),
		TaskChildren:     make(chan Task), // Important, keep unbuffered so code in Task.Execute blocks as expected
		TaskCompletions:  make(chan Task), // Important, keep unbuffered so code in Task.Execute blocks as expected
		DelayWorkgroup:   make(chan WorkgroupDelay),
		Storage:          storage,
		Throttler:        throttler,
		Bootstrapping:    true,
	}
	return &op
}

// CreateGroup adds a new group to the pool.
func (pool *TaskPool) CreateGroup(group *TaskGroup) error {
	// Make sure group address doesn't contain filesystem path characters
	// Prevents filesystem traversal attacks
	if strings.ContainsAny(group.Address, "/.\\") {
		return errors.New("group address contains invalid characters")
	}

	// Persist the group
	saveTaskGroupError := pool.Storage.SaveTaskGroup(group)
	if saveTaskGroupError != nil {
		return saveTaskGroupError
	}

	// Add the group to the pool
	pool.GroupsMutex.Lock()
	pool.Groups[group.Address] = group
	pool.GroupsMutex.Unlock()

	// Emit a group event
	pool.ProcessTaskGroupUpdate(TaskGroupUpdateEvent{
		Event:     "create",
		TaskGroup: *group,
	})
	return nil
}

// ReadGroup
func (pool *TaskPool) ReadGroup(address string) (*TaskGroup, error) {
	pool.GroupsMutex.RLock()
	group, found := pool.Groups[address]
	pool.GroupsMutex.RUnlock()
	if !found {
		return &TaskGroup{}, errors.New("group not found")
	}
	return group, nil
}

// UpdateGroup updates a group. Only the name can be updated.
func (pool *TaskPool) UpdateGroup(group *TaskGroup, update map[string]interface{}) error {
	// Only editable field is Name
	newName, hasNewName := update["name"].(string)
	if hasNewName {
		group.Name = newName
	}
	err := pool.Storage.SaveTaskGroup(group)
	if err != nil {
		return err
	}
	pool.ProcessTaskGroupUpdate(TaskGroupUpdateEvent{
		Event:     "update",
		TaskGroup: *group,
	})
	return nil
}

// DeleteGroup removes a group from the controller.
func (pool *TaskPool) DeleteGroup(address string) error {
	pool.GroupsMutex.Lock()
	group, found := pool.Groups[address]
	pool.GroupsMutex.Unlock()
	if !found {
		return errors.New("cannot find group to remove")
	}
	group.IsDeleting = true

	// Remove group from storage
	err := pool.Storage.DeleteTaskGroup(group)
	if err != nil {
		return err
	}

	// Find all tasks that were in the group and send their operators a Delete message
	pool.TasksMutex.RLock()
	tasksToDelete := make([]*Task, 0)
	for _, task := range pool.Tasks {
		tasksToDelete = append(tasksToDelete, task)
	}
	pool.TasksMutex.RUnlock()
	for _, task := range tasksToDelete {
		pool.DestroyTask(task)
	}

	// // Child tasks won't delete themselves from the pool (tasks don't know about the pool)
	// // so remove them from the pool here
	// // Child tasks will remove themselves from storage
	// pool.TasksMutex.Lock()
	// for _, task := range tasksToDelete {
	// 	delete(pool.Tasks, task.Address)
	// }
	// pool.TasksMutex.Unlock()

	// Remove group from pool
	pool.GroupsMutex.Lock()
	delete(pool.Groups, group.Address)
	pool.GroupsMutex.Unlock()

	pool.ProcessTaskGroupUpdate(TaskGroupUpdateEvent{
		Event:     "delete",
		TaskGroup: *group,
	})

	return nil
}

// CreateTask adds a new task to the pool.
func (pool *TaskPool) CreateTask(task *Task) error {
	// Make sure task address doesn't contain filesystem path characters
	// Prevents filesystem traversal attacks
	if strings.ContainsAny(task.Address, "/.\\") {
		return errors.New("task address contains invalid characters")
	}

	if !pool.Bootstrapping {

	}

	// TODO - setup task Parents, Children
	// If issue with parents/children - set error in task??? or throw error???
	// TODO - make sure children, parents are inflated

	// TODO - create operator for the task
	// TODO - start the operator

}

func (pool *TaskPool) AlignTasks() {
	// TODO - ensure Parents, Children slices are setup for each task
}

func (pool *TaskPool) AlignTask(task *Task) {
	// TODO - ensure Parents, Children slices are setup for the task
}

func (pool *TaskPool) Operate() {
	// TODO (bootstrapping is complete)
}

// ReadTask
func (pool *TaskPool) ReadTask(address string) (*Task, error) {
	pool.TasksMutex.RLock()
	task, found := pool.Tasks[address]
	pool.TasksMutex.RUnlock()
	if !found {
		return &Task{}, errors.New("task not found")
	}
	return task, nil
}

// UpdateTask
func (pool *TaskPool) UpdateTask(task *Task, update map[string]interface{}) error {
	task.Operator.ExternalUpdates <- TaskUpdate{
		Update: update,
	}
	return nil
}

// DeleteTask
func (pool *TaskPool) DeleteTask(address string) error {
	pool.TasksMutex.Lock()
	task, found := pool.Tasks[address]
	pool.TasksMutex.Unlock()
	if !found {
		return errors.New("cannot find task to remove")
	}
	pool.DestroyTask(task)
	return nil
}

func (pool *TaskPool) DestroyTask(task *Task) error {
	// Child task will remove itself from storage
	// Child task will remove itself from the pool?
	task.Operator.Delete <- true
}

// DelayWorkgroup delays all tasks in a workgroup by a given number of seconds.
func (pool *TaskPool) ProcessWorkgroupDelay(workgroup string, delayInSeconds int) {
	// send update to all tasks in all groups that match workgroup
	pool.TasksMutex.RLock()
	tasksToDelay := make([]*Task, 0)
	for _, task := range pool.Tasks {
		if task.Workgroup == workgroup && !task.IsComplete {
			tasksToDelay = append(tasksToDelay, task)
		}
	}
	pool.TasksMutex.RUnlock()
	for _, task := range tasksToDelay {
		// Update runAfter for task
		newRunAfter := time.Now().Add(time.Duration(delayInSeconds * int(time.Second)))
		task.Operator.ExternalUpdates <- TaskUpdate{
			Update: map[string]interface{}{
				"runAfter": newRunAfter,
			},
			UpdateComplete: nil,
		}
	}
}

func (pool *TaskPool) ProcessTaskCompleted(task *Task) {
	// TODO - make sure children, parents are correct for task (and children?)

	// Complete all other tasks with matching key
	if operator.Task.Key != "" {
		operator.TaskGroup.OperatorsMutex.RLock()
		// Collect siblings in lock, process later
		keySiblingOperators := make([]*TaskOperator, 0)
		for _, keySiblingOperator := range operator.TaskGroup.TaskOperators {
			if (keySiblingOperator.Task.Key == operator.Task.Key) && (keySiblingOperator.Task.Id != operator.Task.Id) {
				keySiblingOperators = append(keySiblingOperators, keySiblingOperator)
			}
		}
		operator.TaskGroup.OperatorsMutex.RUnlock()

		for _, keySiblingOperator := range keySiblingOperators {
			keySiblingOperator.Task.IsComplete = true
			keySiblingOperator.Task.Output = workerResponse.Output

			// persist keySiblingOperator.Task
			keySiblingOperator.TaskGroup.Storage.SaveTask(keySiblingOperator.TaskGroup, keySiblingOperator.Task)

			// emit update event
			operator.TaskGroup.Controller.ProcessTaskUpdate(TaskUpdateEvent{
				Event: "update",
				Task:  *keySiblingOperator.Task,
			})

			// Let children know parent is complete
			for _, child := range keySiblingOperator.Task.Children {
				operator.TaskGroup.TaskOperators[child.Id].ParentCompleteEvents <- keySiblingOperator.Task
			}
		}
	}
}

// ProcessTaskUpdate pushes an update event to the controller's TaskUpdates channel.
func (pool *TaskPool) ProcessTaskUpdate(update TaskUpdateEvent) {
	select {
	case pool.TaskUpdates <- update:
	default:
	}
}

// ProcessTaskGroupUpdate pushes an update event to the controller's TaskGroupUpdates channel.
func (pool *TaskPool) ProcessTaskGroupUpdate(update TaskGroupUpdateEvent) {
	select {
	case pool.TaskGroupUpdates <- update:
	default:
	}
}
