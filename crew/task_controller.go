package crew

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron"
)

// A ThrottlePushQuery is a request to the throttler to see if there is enough bandwidth for a worker to run.
type ThrottlePushQuery struct {
	TaskId string
	Worker string
	Resp   chan bool
}

// ThrottlePopQuery is a request to the throttler to notify that a worker is done.
type ThrottlePopQuery struct {
	TaskId string
	Worker string
}

type Throttler struct {
	Push chan ThrottlePushQuery
	Pop  chan ThrottlePopQuery
}

// TaskGroup represents a group of tasks.
type TaskController struct {
	Storage                 TaskStorage
	Client                  TaskClient
	Feed                    chan interface{}
	Throttler               *Throttler
	Pending                 *sync.WaitGroup
	AbandonedCheckScheduler *gocron.Scheduler
	AbandonedCheckMutex     *sync.Mutex
}

// NewTaskController returns a new TaskController.
func NewTaskController(storage TaskStorage, client TaskClient, throttler *Throttler) *TaskController {
	return &TaskController{
		Storage:   storage,
		Client:    client,
		Feed:      make(chan interface{}, 8),
		Throttler: throttler,
		Pending:   &sync.WaitGroup{},
		// AbandonedCheckScheduler is created in startup
		AbandonedCheckMutex: &sync.Mutex{},
	}
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
	allGroups, allTaskGroupsError := controller.Storage.AllTaskGroups()
	if allTaskGroupsError != nil {
		return nil, 0, allTaskGroupsError
	}
	for _, group := range allGroups {
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
		return groups[b].CreatedAt.Before(groups[a].CreatedAt)
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

func (controller *TaskController) GetTasksInGroup(taskGroupId string, page int, pageSize int, search string, skipCompleted bool) (tasks []*Task, total int, err error) {

	allTasksInGroup, allTasksInGroupError := controller.Storage.AllTasksInGroup(taskGroupId)
	if allTasksInGroupError != nil {
		return nil, 0, allTasksInGroupError
	}

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

	if skipCompleted {
		// Filter out completed tasks
		filtered := make([]*Task, 0)
		for _, task := range tasks {
			if !task.IsComplete {
				filtered = append(filtered, task)
			}
		}
		tasks = filtered
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

func (controller *TaskController) GetTaskGroupProgress(id string) (completedPercent float64, err error) {
	allTasksInGroup, allTasksInGroupError := controller.Storage.AllTasksInGroup(id)
	if allTasksInGroupError != nil {
		return 0.0, allTasksInGroupError
	}
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
	return completedPercent, nil
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
	// We have two options here:
	// 1) Just call evaluate in a goroutine (for a single host system)

	// 2) Use an API call to trigger the evaluation (for a scalable system)

	// Option 1:
	go func() {
		task, err := controller.Storage.FindTask(id)
		if err == nil {
			controller.Evaluate(task)
		}
	}()

	return nil
}

func (controller *TaskController) CreateTaskGroup(taskGroup *TaskGroup) (err error) {
	err = controller.Storage.SaveTaskGroup(taskGroup, true)
	controller.EmitTaskGroupFeedEvent("create", taskGroup)
	return err
}

func (controller *TaskController) CreateTask(task *Task) (err error) {
	err = controller.Storage.SaveTask(task, true)
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
	controller.Storage.SaveTask(task, false)
	controller.EmitTaskFeedEvent("update", task)
	controller.TriggerTaskEvaluate(task.Id)
}

func (controller *TaskController) ResetTaskGroup(id string, remainingAttempts int) (err error) {
	allTasksInGroup, allTasksInGroupError := controller.Storage.AllTasksInGroup(id)
	if allTasksInGroupError != nil {
		return allTasksInGroupError
	}

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
	allTasksInGroup, allTasksInGroupError := controller.Storage.AllTasksInGroup(id)
	if allTasksInGroupError != nil {
		return allTasksInGroupError
	}

	for _, task := range allTasksInGroup {
		if !task.IsComplete {
			task.RemainingAttempts = remainingAttempts
			controller.Storage.SaveTask(task, false)
			controller.EmitTaskFeedEvent("update", task)
			controller.TriggerTaskEvaluate(task.Id)
		}
	}
	return nil
}

func (controller *TaskController) PauseOrResumeTaskGroup(id string, isPaused bool) (err error) {
	allTasksInGroup, allTasksInGroupError := controller.Storage.AllTasksInGroup(id)
	if allTasksInGroupError != nil {
		return allTasksInGroupError
	}

	for _, task := range allTasksInGroup {
		task.IsPaused = isPaused
		controller.Storage.SaveTask(task, false)
		controller.EmitTaskFeedEvent("update", task)
		if !isPaused && !task.IsComplete {
			// Evaluate is only necessary if un-pausing
			controller.TriggerTaskEvaluate(task.Id)
		}
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
	controller.Storage.SaveTask(foundTask, false)
	controller.EmitTaskFeedEvent("update", foundTask)
	controller.TriggerTaskEvaluate(foundTask.Id)
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

	controller.Storage.SaveTaskGroup(foundTaskGroup, false)
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

	newIsComplete, hasIsComplete := update["isComplete"].(bool)
	if hasIsComplete {
		task.IsComplete = newIsComplete
	}

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

	controller.Storage.SaveTask(task, false)
	controller.EmitTaskFeedEvent("update", task)
	// if shouldReIndex {

	// }
	if shouldReEvaluate && !task.IsComplete && !task.IsPaused {
		controller.TriggerTaskEvaluate(task.Id)
	}
	return task, nil
}

func (controller *TaskController) Startup() (err error) {
	// Restart tasks on startup and/or check for tasks that may have been abandoned due to crashes (or power outages) during execution.
	// Note that for this to work for abandonments the storage mechanism must have expirations on task locks.
	// Only the redis storage mechanism currently supports this.
	s := gocron.NewScheduler(time.UTC)
	// TODO - configure interval with an env var?
	s.Every(15).Minutes().Do(func() {
		log.Println("Abandoned task scan starting")

		// Use a mutex to make sure this doesn't run more than once at a time
		locked := controller.AbandonedCheckMutex.TryLock()
		if !locked {
			log.Println("Previous abandoned task scan still running, bailing out.")
			return
		}
		defer controller.AbandonedCheckMutex.Unlock()

		taskGroups, taskGroupsError := controller.Storage.AllTaskGroups()
		if taskGroupsError == nil {
			for _, group := range taskGroups {
				tasks, tasksError := controller.Storage.AllTasksInGroup(group.Id)
				if tasksError == nil {
					for _, task := range tasks {
						// Do a couple of quick checks to prevent unecessary evaluates
						if !task.IsComplete && !task.IsPaused && task.RunAfter.After(time.Now()) && task.RemainingAttempts > 0 {
							controller.TriggerTaskEvaluate(task.Id)
							// Slight pause here to prevent a flood of evaluates
							time.Sleep(time.Second / 10)
						}
					}
				} else {
					log.Println("Error scanning for abandoned tasks", tasksError)
				}

				// Pause between groups to prevent overloading ourselves.
				time.Sleep(time.Second * 1)
			}
		} else {
			log.Println("Error scanning for abandoned tasks (fetch groups)", taskGroupsError)
		}

		log.Println("Abandoned task scan completed")
	})
	s.StartAsync()
	controller.AbandonedCheckScheduler = s

	return nil
}

func (controller *TaskController) Shutdown() (err error) {
	if controller.AbandonedCheckScheduler != nil {
		controller.AbandonedCheckScheduler.Stop()
	}

	// Wait till all pending task executions are complete
	controller.Pending.Wait()
	return nil
}

func (controller *TaskController) Evaluate(task *Task) {
	parents, _ := controller.Storage.GetTaskParents(task.Id)
	log.Println("Evaluating task", task.Id, len(parents))
	canExecute := task.CanExecute(parents)
	if canExecute {
		controller.Execute(task)
	}
}

func (controller *TaskController) Execute(taskToExecute *Task) {
	log.Println("Executing task", taskToExecute.Id)
	parents, _ := controller.Storage.GetTaskParents(taskToExecute.Id)

	timer := time.NewTimer(1000 * time.Second)
	timer.Stop()

	controller.Pending.Add(1)
	go func() {
		defer controller.Pending.Done()

		log.Println("Waiting for task start time (go routine)", taskToExecute.Id)
		<-timer.C

		log.Println("Executing task (go routine)", taskToExecute.Id)

		// Lock is as close to worker request send as possible (in case task delay is longer than lock timeout)
		unlocker, lockError := controller.Storage.TryLockTask(taskToExecute.Id)
		if lockError != nil {
			// Failed to lock!
			return
		}
		// Unlock task no matter what else happens below!
		defer unlocker()

		if lockError != nil {
			// Couldn't lock task, do not execute
			log.Println("Executing task (lock fail)", taskToExecute.Id)
			return
		}

		// Make sure task hasn't been deleted
		task, err := controller.Storage.FindTask(taskToExecute.Id)
		if err != nil {
			// Task was deleted while we were waiting for the timer.
			// TODO - what if error was a db connection issue?
			return
		}

		// If runAfter has not passed, it may have been updated while we were waiting for the timer.
		// Do not execute, but re-evaluate
		if task.RunAfter.After(time.Now()) {
			controller.TriggerTaskEvaluate(task.Id)
			return
		}

		canExecute := task.CanExecute(parents)
		// Double check if task is still executable
		if canExecute {

			// Apply worker throttling if a throttler is defined
			throttler := controller.Throttler
			if (throttler != nil) && (task.Worker != "") {
				query := ThrottlePushQuery{
					TaskId: task.Id,
					Worker: task.Worker,
					Resp:   make(chan bool)}
				throttler.Push <- query
				// Block until throttler says it is ok to send task request
				<-query.Resp
			}

			task.BusyExecuting = true
			controller.Storage.SaveTask(task, false)
			controller.EmitTaskFeedEvent("update", task)

			workerResponse, err := controller.Client.Post(task, parents)

			if (throttler != nil) && (task.Worker != "") {
				query := ThrottlePopQuery{
					TaskId: task.Id,
					Worker: task.Worker}
				// Let throttler know that task attempt is complete
				throttler.Pop <- query
			}

			// post exec state updates
			task.RemainingAttempts--
			task.Output = workerResponse.Output
			task.BusyExecuting = false

			if err != nil {
				log.Println("Got standard error", task.Id, err)
				controller.HandleExecuteError(task, fmt.Sprintf("%v", err))
			} else if workerResponse.Error != nil {
				log.Println("Got worker response error", task.Id, workerResponse.Error)
				controller.HandleExecuteError(task, fmt.Sprintf("%v", workerResponse.Error))
			} else {
				// No error!
				task.IsComplete = true

				// Create children
				createdChildren := make([]*Task, 0)
				var errorCreatingChildren error
				for _, childTask := range workerResponse.Children {
					child := NewTask()
					child.Id = childTask.Id
					child.TaskGroupId = task.TaskGroupId
					child.Name = childTask.Name
					child.Worker = childTask.Worker
					child.Workgroup = childTask.Workgroup
					child.Key = childTask.Key
					child.RemainingAttempts = childTask.RemainingAttempts
					if child.RemainingAttempts == 0 {
						child.RemainingAttempts = 5
					}
					child.IsPaused = childTask.IsPaused
					child.IsComplete = false
					child.RunAfter = childTask.RunAfter
					child.ErrorDelayInSeconds = childTask.ErrorDelayInSeconds
					if child.ErrorDelayInSeconds == 0 {
						child.ErrorDelayInSeconds = 60
					}
					child.Input = childTask.Input
					child.ParentIds = childTask.ParentIds

					// NOTE - current task is always added as a parent so that children won't begin exec until we are done creating them all
					// This allows children to be created in any order.
					child.ParentIds = append(child.ParentIds, task.Id)

					// Save the new child
					// Create children in a "transaction" so that if one fails to create, all get removed?
					errorCreatingChildren := controller.Storage.SaveTask(child, true)
					if errorCreatingChildren != nil {
						break
					}
					createdChildren = append(createdChildren, child)
				}

				if errorCreatingChildren != nil {
					// A child failed to create, delete all created children
					for _, child := range createdChildren {
						controller.Storage.DeleteTask(child.Id)
					}

					// Because children failed we have to fail the task so that users will know something went wrong.
					task.IsComplete = false
					log.Println("Got child creation error", task.Id, errorCreatingChildren)
					controller.HandleExecuteError(task, fmt.Sprintf("Child create failure : %v", errorCreatingChildren))
				} else {
					for _, child := range createdChildren {
						controller.EmitTaskFeedEvent("create", child)
					}
				}
			}

			// Apply child delays
			// Note that child delays are done here instead of above because task may have a mix of pre-populated children and children created from its output.
			if workerResponse.ChildrenDelayInSeconds > 0 {
				// This can happen in the background
				go func() {
					allChildren, getChildrenError := controller.Storage.GetTaskChildren(task.Id)
					if getChildrenError != nil {
						for _, child := range allChildren {
							child.RunAfter = time.Now().Add(time.Duration(workerResponse.ChildrenDelayInSeconds * int(time.Second)))
							controller.Storage.SaveTask(child, false)
							controller.EmitTaskFeedEvent("update", child)
							// No evaluate sent here, is sent below if parent complete
						}
					}
					// else {
					// 	// TODO What should we do here? (failed to fetch children)
					// }
				}()
			}

			// TODO - apply workgroup delays
			if workerResponse.WorkgroupDelayInSeconds > 0 {
				// This can happen in the background
				go func() {
					workgroupTasks, workgroupTasksError := controller.Storage.GetTasksInWorkgroup(task.Workgroup)
					if workgroupTasksError != nil {
						for _, workgroupTask := range workgroupTasks {
							workgroupTask.RunAfter = time.Now().Add(time.Duration(workerResponse.WorkgroupDelayInSeconds * int(time.Second)))
							controller.Storage.SaveTask(workgroupTask, false)
							controller.EmitTaskFeedEvent("update", workgroupTask)
							// No evaluate sent here, is sent below if parent complete
						}
					}
				}()
			}

			controller.Storage.SaveTask(task, false)
			controller.EmitTaskFeedEvent("update", task)

			if !task.IsComplete {
				controller.TriggerTaskEvaluate(task.Id)
			} else {
				// Notify children that parent is complete (via an evaluate)
				go func() {
					allChildren, getChildrenError := controller.Storage.GetTaskChildren(task.Id)
					if getChildrenError == nil {
						for _, child := range allChildren {
							controller.TriggerTaskEvaluate(child.Id)
						}
					}
				}()

				// Apply de-duplication (and notify the children of duplicates!)
				if task.Key != "" {
					go func() {
						keyMatches, keyMatchesError := controller.Storage.GetTasksWithKey(task.Key)
						if (keyMatchesError == nil) && (len(keyMatches) > 1) {
							for _, keyMatch := range keyMatches {
								if keyMatch.Id != task.Id {
									keyMatch.IsComplete = true
									keyMatch.Output = task.Output
									controller.Storage.SaveTask(keyMatch, false)
									controller.EmitTaskFeedEvent("update", keyMatch)

									// Notify children that parent is complete (via an evaluate)
									keyMatchChildren, keyMatchChildrenError := controller.Storage.GetTaskChildren(keyMatch.Id)
									if keyMatchChildrenError != nil {
										for _, child := range keyMatchChildren {
											controller.TriggerTaskEvaluate(child.Id)
										}
									}
								}
							}
						}
					}()
				}
			}
		}
	}()

	now := time.Now()
	if now.Before(taskToExecute.RunAfter) {
		// Task's run after has not passed
		timer.Reset(taskToExecute.RunAfter.Sub(now))
	} else {
		// Task's run after has already passed or was not set
		timer.Reset(time.Millisecond)
	}
}

func (controller *TaskController) HandleExecuteError(task *Task, message string) {
	task.Errors = append(task.Errors, message)
	errorDelay := time.Duration(task.ErrorDelayInSeconds * int(time.Second))
	task.RunAfter = time.Now().Add(errorDelay)
}
