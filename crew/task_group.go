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

// A TaskGroup represents a collection of all the tasks required to complete a body of work.
type TaskGroup struct {
	Id            string                   `json:"id"`
	Name          string                   `json:"name"`
	IsPaused      bool                     `json:"isPaused"`
	CreatedAt     time.Time                `json:"createdAt"`
	Channels      map[string]Channel       `json:"-"`
	TaskOperators map[string]*TaskOperator `json:"-"` // `json:"tasks"`
	// This is for sending updates to UI, all group and task create/update/delete events should get sent here:
	TaskUpdates chan TaskUpdateEvent `json:"-"`
}

// Prepare adds the given tasks to the group, wrapping each with an operator.
// This method also populates Task.Children from other Task.ParentIds within
// the group.
func (taskGroup TaskGroup) Prepare(tasks []*Task, channels map[string]Channel, client TaskClient) {
	taskGroup.Channels = channels

	// Key = parentId
	// Value = child tasks
	childrenIndex := make(map[string][]*Task)

	// Create an operator for each task in the group
	for _, task := range tasks {
		operator := NewTaskOperator(task, &taskGroup, channels, client)
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

func (taskGroup TaskGroup) AddTask(task *Task, client TaskClient) error {
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
	operator := NewTaskOperator(task, &taskGroup, taskGroup.Channels, client)

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

	select {
	case operator.TaskGroup.TaskUpdates <- TaskUpdateEvent{
		Event: "create",
		Task:  *operator.Task,
	}:
	default:
	}

	// TODO - persist
	return nil
}

func (taskGroup TaskGroup) DeleteTask(id string) error {
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

	select {
	case operator.TaskGroup.TaskUpdates <- TaskUpdateEvent{
		Event: "delete",
		Task:  *operator.Task,
	}:
	default:
	}

	// Remove from the group
	delete(taskGroup.TaskOperators, operator.Task.Id)

	// TODO - persist the change
	return nil
}

// Operate begins the lifecycle of every task in the group
func (taskGroup TaskGroup) Operate() {
	for _, operator := range taskGroup.TaskOperators {
		operator.Operate()
	}
}
