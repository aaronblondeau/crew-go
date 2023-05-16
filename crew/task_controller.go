package crew

import (
	"sort"
	"strings"
	"time"
)

// TaskGroup represents a group of tasks.
type TaskController struct {
	Storage TaskStorage
	Feed    chan interface{}
}

type TaskFeedEvent struct {
	Event string `json:"type"`
	Task  *Task  `json:"task"`
}

type TaskGroupFeedEvent struct {
	Event     string     `json:"type"`
	TaskGroup *TaskGroup `json:"taskGroup"`
}

func (controller *TaskController) GetTaskGroups(page int, pageSize int, search string) (taskGroups []*TaskGroup, total int, err error) {

	// create an all groups slice (while performing search)
	groups := make([]*TaskGroup, 0)
	for _, group := range controller.Storage.AllTaskGroups() {
		if search != "" {
			if strings.Contains(strings.ToLower(group.Name), strings.ToLower(search)) {
				groups = append(groups, group)
			}
		} else {
			groups = append(groups, group)
		}
	}

	// sort all groups slice
	sort.Slice(groups, func(a, b int) bool {
		return groups[a].CreatedAt.Before(groups[b].CreatedAt)
	})

	if pageSize == 0 {
		pageSize = len(groups)
	}

	// pagninate groups slice
	slice_start := (page - 1) * pageSize
	slice_end := slice_start + pageSize
	slice_count := len(groups)
	if slice_start < 0 {
		slice_start = 0
	}
	if slice_end < slice_start {
		slice_end = slice_start
	}
	if slice_start > slice_count {
		slice_start = slice_count
	}
	if slice_end > slice_count {
		slice_end = slice_count
	}
	sliced := groups[slice_start:slice_end]

	return sliced, slice_count, nil
}

func (controller *TaskController) GetTaskGroup(id string) (taskGroup *TaskGroup, err error) {
	return controller.Storage.FindTaskGroup(id)
}

func (controller *TaskController) GetTasksInGroup(taskGroupId string, page int, pageSize int, search string) (tasks []*Task, total int, err error) {

	allTasksInGroup := controller.Storage.AllTasksInGroup(taskGroupId)

	// create an all tasks slice
	tasks = make([]*Task, 0)
	for _, task := range allTasksInGroup {
		if search != "" {
			if strings.Contains(strings.ToLower(task.Name), strings.ToLower(search)) {
				tasks = append(tasks, task)
			}
		} else {
			tasks = append(tasks, task)
		}
	}

	// sort all tasks slice
	sort.Slice(tasks, func(a, b int) bool {
		return tasks[a].CreatedAt.Before(tasks[b].CreatedAt)
	})

	if pageSize == 0 {
		pageSize = len(tasks)
	}

	// pagninate tasks slice
	slice_start := (page - 1) * pageSize
	slice_end := slice_start + pageSize
	slice_count := len(tasks)
	if slice_start < 0 {
		slice_start = 0
	}
	if slice_end < slice_start {
		slice_end = slice_start
	}
	if slice_start > slice_count {
		slice_start = slice_count
	}
	if slice_end > slice_count {
		slice_end = slice_count
	}
	sliced := tasks[slice_start:slice_end]

	return sliced, slice_count, nil
}

func (controller *TaskController) GetTaskGroupProgress(id string) (completedPercent float64) {
	allTasksInGroup := controller.Storage.AllTasksInGroup(id)
	total := len(allTasksInGroup)
	completed := 0
	// Iterate all tasks
	for _, task := range allTasksInGroup {
		if task.IsComplete {
			completed++
		}
	}

	completedPercent = 0.0
	if total > 0 {
		completedPercent = float64(completed) / float64(total)
	}
	return completedPercent
}

func (controller *TaskController) GetTask(id string) (task *Task, err error) {
	task, err = controller.Storage.FindTask(id)
	return task, err
}

func (controller *TaskController) EmitTaskGroupFeedEvent(event string, taskGroup *TaskGroup) {
	if controller.Feed != nil {
		select {
		case controller.Feed <- TaskGroupFeedEvent{
			Event:     event,
			TaskGroup: taskGroup,
		}:
		default:
			// Ignore no event feed listener
		}
	}
}

func (controller *TaskController) EmitTaskFeedEvent(event string, task *Task) {
	if controller.Feed != nil {
		select {
		case controller.Feed <- TaskFeedEvent{
			Event: event,
			Task:  task,
		}:
		default:
			// Ignore no event feed listener
		}
	}
}

func (controller *TaskController) TriggerTaskEvaluate(id string) (err error) {
	// TODO
	return nil
}

func (controller *TaskController) CreateTaskGroup(taskGroup *TaskGroup) (err error) {
	err = controller.Storage.SaveTaskGroup(taskGroup)
	controller.EmitTaskGroupFeedEvent("create", taskGroup)
	return err
}

func (controller *TaskController) CreateTask(task *Task) (err error) {
	err = controller.Storage.SaveTask(task)
	controller.EmitTaskFeedEvent("create", task)
	controller.TriggerTaskEvaluate(task.Id)
	return err
}

func (controller *TaskController) DeleteTaskGroup(id string) (err error) {
	taskGroup, err := controller.Storage.FindTaskGroup(id)
	if err != nil {
		return err
	}
	err = controller.Storage.DeleteTaskGroup(taskGroup.Id)
	controller.EmitTaskGroupFeedEvent("delete", taskGroup)
	return err
}

func (controller *TaskController) DeleteTask(id string) (err error) {
	task, err := controller.Storage.FindTask(id)
	if err != nil {
		return err
	}
	err = controller.Storage.DeleteTask(task.Id)
	controller.EmitTaskFeedEvent("delete", task)
	return err
}

func (controller *TaskController) ResetTask(task *Task, remainingAttempts int) {
	task.RemainingAttempts = remainingAttempts
	task.IsComplete = false
	task.Output = nil
	task.Errors = make([]string, 0)
	task.RunAfter = time.Now()
	controller.Storage.SaveTask(task)
	controller.EmitTaskFeedEvent("update", task)
	controller.TriggerTaskEvaluate(task.Id)
}

func (controller *TaskController) ResetTaskGroup(id string, remainingAttempts int) (err error) {
	allTasksInGroup := controller.Storage.AllTasksInGroup(id)

	hasSeedTasks := false
	for _, task := range allTasksInGroup {
		if task.IsSeed {
			hasSeedTasks = true
			break
		}
	}

	for _, task := range allTasksInGroup {
		if hasSeedTasks && !task.IsSeed {
			// Delete non-seed tasks in a seeded group
			controller.Storage.DeleteTask(task.Id)
			controller.EmitTaskFeedEvent("delete", task)
		} else {
			// Otherwise reset the task
			controller.ResetTask(task, remainingAttempts)
		}
	}
	return nil
}

func (controller *TaskController) RetryTaskGroup(id string, remainingAttempts int) (err error) {
	allTasksInGroup := controller.Storage.AllTasksInGroup(id)

	for _, task := range allTasksInGroup {
		task.RemainingAttempts = remainingAttempts
		controller.Storage.SaveTask(task)
		controller.EmitTaskFeedEvent("update", task)
		controller.TriggerTaskEvaluate(task.Id)
	}
	return nil
}

func (controller *TaskController) PauseOrResumeTaskGroup(id string, isPaused bool) (err error) {
	allTasksInGroup := controller.Storage.AllTasksInGroup(id)

	for _, task := range allTasksInGroup {
		task.IsPaused = isPaused
		controller.Storage.SaveTask(task)
		controller.EmitTaskFeedEvent("update", task)
		controller.TriggerTaskEvaluate(task.Id)
	}
	return nil
}

func (controller *TaskController) ResetTaskById(id string, remainingAttempts int) (task *Task, err error) {
	foundTask, err := controller.Storage.FindTask(id)
	if err != nil {
		return nil, err
	}
	controller.ResetTask(foundTask, remainingAttempts)
	return foundTask, nil
}

func (controller *TaskController) RetryTaskById(id string, remainingAttempts int) (task *Task, err error) {
	foundTask, err := controller.Storage.FindTask(id)
	if err != nil {
		return nil, err
	}
	foundTask.RemainingAttempts = remainingAttempts
	controller.Storage.SaveTask(foundTask)
	controller.EmitTaskFeedEvent("update", foundTask)
	controller.TriggerTaskEvaluate(task.Id)
	return task, nil
}

func (controller *TaskController) UpdateTaskGroup(id string, update map[string]interface{}) (taskGroup *TaskGroup, err error) {
	foundTaskGroup, err := controller.Storage.FindTaskGroup(id)
	if err != nil {
		return nil, err
	}

	// Only name can be updated
	newName, hasNewName := update["name"].(string)
	if hasNewName {
		foundTaskGroup.Name = newName
	}

	controller.Storage.SaveTaskGroup(foundTaskGroup)
	controller.EmitTaskGroupFeedEvent("update", foundTaskGroup)
	return foundTaskGroup, nil
}

func (controller *TaskController) UpdateTask(id string, update map[string]interface{}) (updatedTask *Task, err error) {
	task, err := controller.Storage.FindTask(id)
	if err != nil {
		return nil, err
	}

	// shouldReIndex := false
	shouldReEvaluate := false
	newName, hasNewName := update["name"].(string)
	if hasNewName {
		task.Name = newName
	}

	newWorker, hasNewWorker := update["worker"].(string)
	if hasNewWorker {
		task.Worker = newWorker
	}

	// TODO - difficult update because it affects storage indexes
	// newWorkgroup, hasNewWorkgroup := update["workgroup"].(string)
	// if hasNewWorkgroup && task.Workgroup != newWorkgroup {
	// 	task.Workgroup = newWorkgroup
	// 	shouldReIndex = true
	// }

	// TODO - difficult update because it affects storage indexes
	// newKey, hasNewKey := update["key"].(string)
	// if hasNewKey && task.Key != newKey {
	// 	task.Key = newKey
	// 	shouldReIndex = true
	// }

	newIsPaused, hasIsPaused := update["isPaused"].(bool)
	if hasIsPaused {
		task.IsPaused = newIsPaused
		shouldReEvaluate = true
	}

	newRunAfter, hasRunAfter := update["runAfter"]
	if hasRunAfter {
		switch t := newRunAfter.(type) {
		case time.Time:
			task.RunAfter = t
		default:
			task.RunAfter = time.Time{}
		}
		shouldReEvaluate = true
	}

	// TODO
	// shouldStop := false
	// newIsComplete, hasIsComplete := update["isComplete"].(bool)
	// if hasIsComplete {
	// 	task.IsComplete = newIsComplete
	// 	if task.IsComplete {
	// 		shouldStop = true
	// 	}
	// }

	newRemainingAttempts, hasRemainingAttempts := update["remainingAttempts"]
	if hasRemainingAttempts {
		switch t := newRemainingAttempts.(type) {
		case int:
			task.RemainingAttempts = t
		case float64:
			task.RemainingAttempts = int(t)
		default:
			task.RemainingAttempts = 0
		}
		shouldReEvaluate = true
	}

	newErrorDelayInSeconds, hasErrorDelayInSeconds := update["errorDelayInSeconds"]
	if hasErrorDelayInSeconds {
		switch t := newErrorDelayInSeconds.(type) {
		case int:
			task.ErrorDelayInSeconds = t
		case float64:
			task.ErrorDelayInSeconds = int(t)
		default:
			task.ErrorDelayInSeconds = 0
		}
	}

	newInput, hasInput := update["input"]
	if hasInput {
		task.Input = newInput
	}

	newOutput, hasOutput := update["output"]
	if hasOutput {
		task.Output = newOutput
	}

	newErrors, hasErrors := update["errors"].([]string)
	if hasErrors {
		task.Errors = newErrors
	}

	newIsSeed, hasIsSeed := update["isSeed"].(bool)
	if hasIsSeed {
		task.IsSeed = newIsSeed
	}

	controller.Storage.SaveTask(task)
	controller.EmitTaskFeedEvent("update", task)
	// if shouldReIndex {

	// }
	if shouldReEvaluate {
		controller.TriggerTaskEvaluate(task.Id)
	}
	return task, nil
}

func (controller *TaskController) Startup() (err error) {
	// TODO
	return nil
}

func (controller *TaskController) Shutdown() (err error) {
	// TODO
	return nil
}
