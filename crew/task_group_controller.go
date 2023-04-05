package crew

import (
	"errors"
	"time"
)

// A TaskGroupUpdateEvent notifies listeners of changes to a TaskGroup's state.
type TaskGroupUpdateEvent struct {
	Event     string    `json:"type"`
	TaskGroup TaskGroup `json:"task_group"`
}

// A TaskGroupController combines a set of task groups with a storage mechanism and event notification channels.
type TaskGroupController struct {
	TaskGroups map[string]*TaskGroup
	// This is for sending updates to UI, all group and task create/update/delete events should get sent here:
	TaskUpdates      chan TaskUpdateEvent      `json:"-"`
	TaskGroupUpdates chan TaskGroupUpdateEvent `json:"-"`
	Storage          TaskStorage               `json:"-"`
}

// NewTaskGroupController creates a new TaskGroupController.
func NewTaskGroupController(storage TaskStorage) *TaskGroupController {
	op := TaskGroupController{
		TaskGroups:       make(map[string]*TaskGroup),
		TaskUpdates:      make(chan TaskUpdateEvent, 8),
		TaskGroupUpdates: make(chan TaskGroupUpdateEvent, 8),
		Storage:          storage,
	}
	return &op
}

// DelayWorkgroup delays all tasks in a workgroup by a given number of seconds.
func (controller *TaskGroupController) DelayWorkgroup(workgroup string, delayInSeconds int) {
	// send update to all tasks in all groups that match workgroup
	for _, group := range controller.TaskGroups {
		for _, task := range group.TaskOperators {
			if task.Task.Workgroup == workgroup && !task.Task.IsComplete {
				// Update runAfter for task
				newRunAfter := time.Now().Add(time.Duration(delayInSeconds * int(time.Second)))
				task.ExternalUpdates <- TaskUpdate{
					Update: map[string]interface{}{
						"runAfter": newRunAfter,
					},
					UpdateComplete: nil,
				}
			}
		}
	}
}

// AddGroup adds a new group to the controller.
func (controller *TaskGroupController) AddGroup(group *TaskGroup) error {
	// Make sure group uses same storage as controller
	group.Storage = controller.Storage

	// Add group to controller
	controller.TaskGroups[group.Id] = group

	// Persist the group
	saveTaskGroupError := controller.Storage.SaveTaskGroup(group)
	if saveTaskGroupError != nil {
		return saveTaskGroupError
	}

	// Emit a group event
	controller.ProcessTaskGroupUpdate(TaskGroupUpdateEvent{
		Event:     "create",
		TaskGroup: *group,
	})
	return nil
}

// RemoveGroup removes a group from the controller.
func (controller *TaskGroupController) RemoveGroup(group_id string) error {
	group, found := controller.TaskGroups[group_id]
	if !found {
		return errors.New("cannot find group to remove")
	}
	group.IsDeleting = true

	// Stop all operators
	for _, operator := range group.TaskOperators {
		operator.Task.IsDeleting = true

		// Stop the task's operator
		select {
		case operator.Shutdown <- true:
		default:
			// Ignore no shutdown listener...
		}
	}

	// Remove the group
	fileDeleteErr := group.Storage.DeleteTaskGroup(group)
	if fileDeleteErr != nil {
		// TODO, If bailing from delete here, should we unset the IsDeleting flag for group and tasks?
		return fileDeleteErr
	}

	// Remove group from controller
	delete(controller.TaskGroups, group_id)

	controller.ProcessTaskGroupUpdate(TaskGroupUpdateEvent{
		Event:     "delete",
		TaskGroup: *group,
	})

	return nil
}

// UpdateGroup updates a group. Only the name can be updated.
func (controller *TaskGroupController) UpdateGroup(group *TaskGroup, update map[string]interface{}) error {
	// Only editable field is Name
	newName, hasNewName := update["name"].(string)
	if hasNewName {
		group.Name = newName
	}
	err := controller.Storage.SaveTaskGroup(group)
	if err != nil {
		return err
	}
	controller.ProcessTaskGroupUpdate(TaskGroupUpdateEvent{
		Event:     "update",
		TaskGroup: *group,
	})
	return nil
}

// Operate cascades the Operate() call to all task groups in the controller.
func (controller *TaskGroupController) Operate() {
	for _, taskGroup := range controller.TaskGroups {
		taskGroup.Operate()
	}
}

// ProcessTaskUpdate pushes an update event to the controller's TaskUpdates channel.
func (controller *TaskGroupController) ProcessTaskUpdate(update TaskUpdateEvent) {
	select {
	case controller.TaskUpdates <- update:
	default:
	}
}

// ProcessTaskGroupUpdate pushes an update event to the controller's TaskGroupUpdates channel.
func (controller *TaskGroupController) ProcessTaskGroupUpdate(update TaskGroupUpdateEvent) {
	select {
	case controller.TaskGroupUpdates <- update:
	default:
	}
}
