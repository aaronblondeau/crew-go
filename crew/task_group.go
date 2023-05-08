package crew

import (
	"errors"
	"strings"
	"sync"
	"time"
)

// func init() {
// 	fmt.Println("Crew package initialized - do db migrations?")
// }

// TaskUpdateEvent defines the data emitted when a task is updated.
type TaskUpdateEvent struct {
	Event string `json:"type"`
	Task  Task   `json:"task"`
}

// WorkgroupDelayEvent defines the data emitted when a workgroup is delayed.
type WorkgroupDelayEvent struct {
	Workgroup         string `json:"workgroup"`
	DelayInSeconds    int    `json:"delayInSeconds"`
	OriginTaskGroupId string `json:"originTaskGroupId"`
}

// TaskGroup represents a collection of all the tasks required to complete a body of work.
type TaskGroup struct {
	Id             string                   `json:"id"`
	Name           string                   `json:"name"`
	CreatedAt      time.Time                `json:"createdAt"`
	TaskOperators  map[string]*TaskOperator `json:"-"` // `json:"tasks"`
	OperatorsMutex *sync.RWMutex            `json:"-"`
	Storage        TaskStorage              `json:"-"`
	Controller     *TaskGroupController     `json:"-"`
	IsDeleting     bool                     `json:"-"`
}

// NewTaskGroup creates a new TaskGroup.
func NewTaskGroup(id string, name string, controller *TaskGroupController) *TaskGroup {
	tg := TaskGroup{
		Id:             id,
		Name:           name,
		CreatedAt:      time.Now(),
		TaskOperators:  make(map[string]*TaskOperator),
		OperatorsMutex: &sync.RWMutex{},
		Storage:        NewMemoryTaskStorage(),
		Controller:     controller,
	}
	return &tg
}

// PreloadTasks adds the given tasks to the group, wrapping each with an operator.
// This method also populates Task.Children from other Task.ParentIds within
// the group. Use this before calling Operate on a task group.
func (taskGroup *TaskGroup) PreloadTasks(tasks []*Task, client TaskClient) {
	// Key = parentId
	// Value = child tasks
	childrenIndex := make(map[string][]*Task)

	// Create an operator for each task in the group
	taskGroup.OperatorsMutex.Lock()
	for _, task := range tasks {
		operator := NewTaskOperator(task, taskGroup)
		operator.Client = client
		taskGroup.TaskOperators[task.Id] = operator

		// Track children on first pass
		for _, parentId := range task.ParentIds {
			_, childrenInitialized := childrenIndex[parentId]
			if !childrenInitialized {
				childrenIndex[parentId] = make([]*Task, 0)
			}
			childrenIndex[parentId] = append(childrenIndex[parentId], task)
		}
	}
	taskGroup.OperatorsMutex.Unlock()

	// Make a second pass to inflate Task.Children from Task.ParentIds
	for _, task := range tasks {
		children, childrenExist := childrenIndex[task.Id]
		if childrenExist {
			task.Children = children
		}
	}
}

// AddTask adds the given task to the group, wrapping each in an operator, and calling operate.
// This method updates parent and child relationships within the group.
// Use this after calling Operate on a task group.
func (taskGroup *TaskGroup) AddTask(task *Task, client TaskClient) error {
	// Make sure group id doesn't contain filesystem path characters
	// Prevents filesystem traversal attacks
	if strings.ContainsAny(task.Id, "/.\\") {
		return errors.New("task id contains invalid characters")
	}

	if task.Worker == "" {
		return errors.New("task worker is required")
	}

	task.TaskGroupId = taskGroup.Id

	// Make sure task doesn't already exist
	shouldUnlock1 := true
	taskGroup.OperatorsMutex.RLock()
	for _, op := range taskGroup.TaskOperators {
		if op.Task.Id == task.Id {
			taskGroup.OperatorsMutex.RUnlock()
			shouldUnlock1 = false
			return errors.New("task with same id already exists in group")
		}
	}
	if shouldUnlock1 {
		taskGroup.OperatorsMutex.RUnlock()
	}

	// Make sure parents exist
	shouldUnlock2 := true
	taskGroup.OperatorsMutex.RLock()
	for _, parentId := range task.ParentIds {
		_, found := taskGroup.TaskOperators[parentId]
		if !found {
			taskGroup.OperatorsMutex.RUnlock()
			shouldUnlock2 = false
			return errors.New("cannot find all parents for task")
		}
	}
	if shouldUnlock2 {
		taskGroup.OperatorsMutex.RUnlock()
	}

	// Create operator
	operator := NewTaskOperator(task, taskGroup)
	operator.Client = client

	// Add to group
	taskGroup.OperatorsMutex.Lock()
	taskGroup.TaskOperators[task.Id] = operator
	taskGroup.OperatorsMutex.Unlock()

	// Update parents' children
	for _, parentId := range task.ParentIds {
		taskGroup.OperatorsMutex.RLock()
		parentOperator, found := taskGroup.TaskOperators[parentId]
		taskGroup.OperatorsMutex.RUnlock()
		if found {
			parentOperator.Task.Children = append(parentOperator.Task.Children, task)
		}
	}

	// Call operate
	operator.Operate()

	// emit update event
	operator.TaskGroup.Controller.ProcessTaskUpdate(TaskUpdateEvent{
		Event: "create",
		Task:  *operator.Task,
	})

	// Persist the task
	taskGroup.Storage.SaveTask(operator.TaskGroup, operator.Task)

	return nil
}

// DeleteTask removes task with the given id from the group.
// This method updates parent and child relationships within the group.
func (taskGroup *TaskGroup) DeleteTask(id string) error {
	// Find the task
	taskGroup.OperatorsMutex.RLock()
	operator, found := taskGroup.TaskOperators[id]
	taskGroup.OperatorsMutex.RUnlock()
	if !found {
		return errors.New("cannot find task")
	}
	operator.Task.IsDeleting = true

	// Task can only be deleted if it doesn't have any children
	if len(operator.Task.Children) > 0 {
		operator.Task.IsDeleting = false
		return errors.New("cannot delete tasks with children")
	}

	// Stop the task's operator
	select {
	case operator.Shutdown <- true:
	default:
		// Ignore no shutdown listener...
	}

	// If task has parents, remove from parents' Children array
	if len(operator.Task.ParentIds) > 0 {
		for _, parentId := range operator.Task.ParentIds {
			taskGroup.OperatorsMutex.RLock()
			parentOperator, parentOperatorFound := taskGroup.TaskOperators[parentId]
			taskGroup.OperatorsMutex.RUnlock()
			if parentOperatorFound {
				newChildren := make([]*Task, 0)
				// Rebuild children array without removed task
				for _, child := range parentOperator.Task.Children {
					if child.Id != id {
						newChildren = append(newChildren, child)
					}
				}
				parentOperator.Task.Children = newChildren
			}
		}
	}

	// emit update event
	operator.TaskGroup.Controller.ProcessTaskUpdate(TaskUpdateEvent{
		Event: "delete",
		Task:  *operator.Task,
	})

	// Remove from the group
	taskGroup.OperatorsMutex.Lock()
	delete(taskGroup.TaskOperators, operator.Task.Id)
	taskGroup.OperatorsMutex.Unlock()

	// persist the change
	fileDeleteErr := taskGroup.Storage.DeleteTask(operator.TaskGroup, operator.Task)

	return fileDeleteErr
}

// Operate begins the lifecycle of every task in the group
func (taskGroup *TaskGroup) Operate() {
	taskGroup.OperatorsMutex.RLock()
	defer taskGroup.OperatorsMutex.RUnlock()

	for _, operator := range taskGroup.TaskOperators {
		operator.Operate()
	}
}

// Shutdown terminates the lifecycle of every task in the group
func (taskGroup *TaskGroup) Shutdown() {
	taskGroup.OperatorsMutex.RLock()
	defer taskGroup.OperatorsMutex.RUnlock()

	for _, operator := range taskGroup.TaskOperators {
		operator.Shutdown <- true
	}
}

// Reset returns the task group to its pre-execution state.
func (taskGroup *TaskGroup) Reset(remainingAttempts int, updateComplete chan error) error {
	// If there are seed tasks, then delete all non-seed tasks, and reset seed tasks
	// If there are no seed tasks, then reset all tasks

	hasSeedTasks := false
	taskGroup.OperatorsMutex.RLock()
	for _, operator := range taskGroup.TaskOperators {
		if operator.Task.IsSeed {
			hasSeedTasks = true
			break
		}
	}
	taskGroup.OperatorsMutex.RUnlock()

	if hasSeedTasks {
		// Remove all non-seed tasks
		deletedTasks := 1
		for deletedTasks > 0 {
			deletedTasks = 0

			idsToDelete := make([]string, 0)

			taskGroup.OperatorsMutex.RLock()
			for _, operator := range taskGroup.TaskOperators {
				if !operator.Task.IsSeed && len(operator.Task.Children) == 0 {
					idsToDelete = append(idsToDelete, operator.Task.Id)
				}
			}
			taskGroup.OperatorsMutex.RUnlock()

			for _, idToDelete := range idsToDelete {
				taskGroup.DeleteTask(idToDelete)
				deletedTasks++
			}
		}
	}

	// Reset remaining tasks
	hasCleanupError := false

	taskGroup.OperatorsMutex.RLock()
	operatorsToReset := make([]*TaskOperator, 0)
	for _, operator := range taskGroup.TaskOperators {
		operatorsToReset = append(operatorsToReset, operator)
	}
	taskGroup.OperatorsMutex.RUnlock()

	for _, operator := range operatorsToReset {
		operator.ResetTask(remainingAttempts, updateComplete)

		if hasSeedTasks && !operator.Task.IsSeed {
			hasCleanupError = true
		}
	}

	if hasCleanupError {
		return errors.New("unable to reset task hierarchy - found non-seed tasks after clean cycle")
	}
	return nil
}

// UpdateAllTasks updates all tasks in the group with the given update.
func (taskGroup *TaskGroup) UpdateAllTasks(update map[string]interface{}) error {
	taskGroup.OperatorsMutex.RLock()
	ops := make([]*TaskOperator, 0)
	for _, op := range taskGroup.TaskOperators {
		ops = append(ops, op)
	}
	taskGroup.OperatorsMutex.RUnlock()

	for _, op := range ops {
		updateComplete := make(chan error)
		op.ExternalUpdates <- TaskUpdate{
			Update:         update,
			UpdateComplete: updateComplete,
		}
		// TODO, can we find a way to allow all updates to happen in parallel?
		err := <-updateComplete
		if err != nil {
			return err
		}
	}

	return nil
}

// RetryAllTasks retries all tasks in the group with the given remaining attempts.
func (taskGroup *TaskGroup) RetryAllTasks(remainingAttempts int) error {
	taskGroup.OperatorsMutex.RLock()
	ops := make([]*TaskOperator, 0)
	for _, op := range taskGroup.TaskOperators {
		ops = append(ops, op)
	}
	taskGroup.OperatorsMutex.RUnlock()

	for _, op := range ops {
		if !op.Task.IsComplete {
			updateComplete := make(chan error)
			op.ExternalUpdates <- TaskUpdate{
				Update:         map[string]interface{}{"remainingAttempts": remainingAttempts},
				UpdateComplete: updateComplete,
			}
			// TODO, can we find a way to allow all updates to happen in parallel?
			err := <-updateComplete
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// PauseAllTasks pauses all tasks in the group.
func (taskGroup *TaskGroup) PauseAllTasks() error {
	return taskGroup.UpdateAllTasks(map[string]interface{}{"isPaused": true})
}

// UnPauseAllTasks unpauses all tasks in the group.
func (taskGroup *TaskGroup) UnPauseAllTasks() error {
	return taskGroup.UpdateAllTasks(map[string]interface{}{"isPaused": false})
}
