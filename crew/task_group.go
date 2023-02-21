package crew

import (
	"errors"
	"fmt"
	"time"
)

func init() {
	fmt.Println("Crew package initialized - do db migrations?")
}

// A TaskUpdateEvent notifies listeners of changes to a TaskGroup's state.
type TaskUpdateEvent struct {
	Event string `json:"type"`
	Task  Task   `json:"task"`
}

type WorkgroupDelayEvent struct {
	Workgroup         string `json:"workgroup"`
	DelayInSeconds    int    `json:"delayInSeconds"`
	OriginTaskGroupId string `json:"originTaskGroupId"`
}

// A TaskGroup represents a collection of all the tasks required to complete a body of work.
type TaskGroup struct {
	Id            string                   `json:"id"`
	Name          string                   `json:"name"`
	IsPaused      bool                     `json:"isPaused"`
	CreatedAt     time.Time                `json:"createdAt"`
	TaskOperators map[string]*TaskOperator `json:"-"` // `json:"tasks"`
	Storage       TaskStorage              `json:"-"`
	Controller    *TaskGroupController     `json:"-"`
}

func NewTaskGroup(id string, name string, controller *TaskGroupController) *TaskGroup {
	tg := TaskGroup{
		Id:            id,
		Name:          name,
		IsPaused:      false,
		CreatedAt:     time.Now(),
		TaskOperators: make(map[string]*TaskOperator),
		Storage:       NewMemoryTaskStorage(),
		Controller:    controller,
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

	// Make a second pass to inflate Task.Children from Task.ParentIds
	for _, task := range tasks {
		children, childrenExist := childrenIndex[task.Id]
		if childrenExist {
			task.Children = children
		}
	}
}

// AddTask adds the given task to the group, wrapping each it an operator, and calling operate.
// This method updates parent and child relationships within the group.
// Use this after calling Operate on a task group.
func (taskGroup *TaskGroup) AddTask(task *Task, client TaskClient) error {
	// Make sure task doesn't already exist
	for _, op := range taskGroup.TaskOperators {
		if op.Task.Id == task.Id {
			return errors.New("task with same id already exists in group")
		}
	}

	// Make sure parents exist
	for _, parentId := range task.ParentIds {
		_, found := taskGroup.TaskOperators[parentId]
		if !found {
			return errors.New("cannot find all parents for task")
		}
	}

	// Create operator
	operator := NewTaskOperator(task, taskGroup)
	operator.Client = client

	// Add to group
	taskGroup.TaskOperators[task.Id] = operator

	// Update parents' children
	for _, parentId := range task.ParentIds {
		parentOperator, found := taskGroup.TaskOperators[parentId]
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
	operator, found := taskGroup.TaskOperators[id]
	if !found {
		return errors.New("cannot find task")
	}

	// Task can only be deleted if it doesn't have any children
	if len(operator.Task.Children) > 0 {
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
			parentOperator, parentOperatorFound := taskGroup.TaskOperators[parentId]
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
	delete(taskGroup.TaskOperators, operator.Task.Id)

	// persist the change
	taskGroup.Storage.DeleteTask(operator.TaskGroup, operator.Task)

	return nil
}

// Operate begins the lifecycle of every task in the group
func (taskGroup *TaskGroup) Operate() {
	for _, operator := range taskGroup.TaskOperators {
		operator.Operate()
	}
}

// Operate terminates the lifecycle of every task in the group
func (taskGroup *TaskGroup) Shutdown() {
	for _, operator := range taskGroup.TaskOperators {
		operator.Shutdown <- true
	}
}

type TaskGroupController struct {
	TaskGroups map[string]*TaskGroup
	// This is for sending updates to UI, all group and task create/update/delete events should get sent here:
	TaskUpdates chan TaskUpdateEvent `json:"-"`
}

func NewTaskGroupController() *TaskGroupController {
	op := TaskGroupController{
		TaskGroups:  make(map[string]*TaskGroup),
		TaskUpdates: make(chan TaskUpdateEvent, 8),
	}
	return &op
}

func (controller *TaskGroupController) DelayWorkgroup(workgroup string, delayInSeconds int, originTaskGroupId string) {
	// send update to all tasks in all groups that match workgroup
	for _, group := range controller.TaskGroups {
		for _, task := range group.TaskOperators {
			if task.Task.Workgroup == workgroup && !task.Task.IsComplete {
				// Update runAfter for task
				newRunAfter := time.Now().Add(time.Duration(delayInSeconds * int(time.Second)))
				task.ExternalUpdates <- map[string]interface{}{
					"runAfter": newRunAfter,
				}
			}
		}
	}
}

func (controller *TaskGroupController) AddGroup(group *TaskGroup) {
	controller.TaskGroups[group.Id] = group
}

func (controller *TaskGroupController) Operate() {
	for _, taskGroup := range controller.TaskGroups {
		taskGroup.Operate()
	}
}

func (controller *TaskGroupController) ProcessTaskUpdate(update TaskUpdateEvent) {
	select {
	case controller.TaskUpdates <- update:
	default:
	}
}
