package crew

import (
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
	TaskOperators map[string]*TaskOperator `json:"-"` // `json:"tasks"`
	// This is for sending updates to UI, all group and task create/update/delete events should get sent here:
	TaskUpdates chan TaskUpdateEvent `json:"-"`
}

// Prepare adds the given tasks to the group, wrapping each with an operator.
// This method also populates Task.Children from other Task.ParentIds within
// the group.
func (taskGroup TaskGroup) Prepare(tasks []*Task, channels map[string]Channel) {

	// Key = parentId
	// Value = child tasks
	childrenIndex := make(map[string][]*Task)

	// Create an operator for each task in the group
	for _, task := range tasks {
		operator := NewTaskOperator(task, &taskGroup, channels)
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

// Operate begins the lifecycle of every task in the group
func (taskGroup TaskGroup) Operate() {
	for _, operator := range taskGroup.TaskOperators {
		operator.Operate()
	}
}
