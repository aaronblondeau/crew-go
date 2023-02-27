package crew

import (
	"fmt"
	"time"
)

type WorkerResponse struct {
	Output                  interface{} `json:"output"`
	Children                []*Task     `json:"children"`
	WorkgroupDelayInSeconds int         `json:"workgroupDelayInSeconds"`
	ChildrenDelayInSeconds  int         `json:"childrenDelayInSeconds"`
	Error                   interface{} `json:"error"`
}

type TaskClient interface {
	Post(task *Task, taskGroup *TaskGroup) (response WorkerResponse, err error)
}

// A Task represents a unit of work that can be completed by a worker.
type Task struct {
	Id                  string        `json:"id"`
	TaskGroupId         string        `json:"taskGroupId"`
	Name                string        `json:"name"`
	Worker              string        `json:"worker"`
	Workgroup           string        `json:"workgroup"`
	Key                 string        `json:"key"`
	RemainingAttempts   int           `json:"remainingAttempts"`
	IsPaused            bool          `json:"isPaused"`
	IsComplete          bool          `json:"isComplete"`
	Priority            int           `json:"priority"`
	RunAfter            time.Time     `json:"runAfter"`
	ProgressWeight      int           `json:"progressWeight"`
	IsSeed              bool          `json:"isSeed"`
	ErrorDelayInSeconds int           `json:"errorDelayInSeconds"`
	Input               interface{}   `json:"input"`
	Output              interface{}   `json:"output"`
	Errors              []interface{} `json:"errors"`
	CreatedAt           time.Time     `json:"createdAt"`
	ParentIds           []string      `json:"parentIds"`
	BusyExecuting       bool          `json:"busyExecuting"`
	Children            []*Task       `json:"-"`
	IsDeleting          bool          `json:"-"`
	// parentsComplete
	// assignedTo
	// assignedAt
}

type TaskUpdate struct {
	Update         map[string]interface{}
	UpdateComplete chan error
}

// A TaskOperator manages the lifecycle and state of a Task.
type TaskOperator struct {
	Task                 *Task
	TaskGroup            *TaskGroup
	ExternalUpdates      chan TaskUpdate
	ExecuteTimer         *time.Timer
	EvaulateTimer        *time.Timer
	Shutdown             chan bool
	ParentCompleteEvents chan *Task
	Operating            bool
	Executing            chan bool
	Terminated           chan bool
	Client               TaskClient
}

func NewTaskOperator(task *Task, taskGroup *TaskGroup) *TaskOperator {
	execTimer := time.NewTimer(1000 * time.Second)
	execTimer.Stop()
	evalTimer := time.NewTimer(1000 * time.Second)
	evalTimer.Stop()

	// client := NewHttpPostClient()
	t := TaskOperator{
		Task:                 task,
		TaskGroup:            taskGroup,
		ExternalUpdates:      make(chan TaskUpdate, 8),
		ExecuteTimer:         execTimer,
		EvaulateTimer:        evalTimer,
		Shutdown:             make(chan bool),
		ParentCompleteEvents: make(chan *Task, len(task.Children)),
		Executing:            make(chan bool),
		Terminated:           make(chan bool),
		//Client:               client,
	}
	// Don't let initial timer run
	t.CancelExecute()
	return &t
}

// Operate brings a Task operator to life.
// Will need passed in
// map to all other tasks (in group) (so we can look up parent ids, send events to children)
// map to channels
func (operator *TaskOperator) Operate() {
	// TODO - multi-threaded way to ensure Operate
	// only gets called once?
	if operator.Operating {
		return
	}
	operator.Operating = true

	fmt.Println("About to operate")
	// All of the Task's lifecycle should live in this goroutine
	go func() {
		fmt.Println("Operating")
		// Continuously handle channel events (till we get an event on Shutdown)
		for {
			select {
			case <-operator.ExecuteTimer.C:
				operator.Execute()

			case <-operator.EvaulateTimer.C:
				operator.Execute()

			case update := <-operator.ExternalUpdates:
				newName, hasNewName := update.Update["name"].(string)
				if hasNewName {
					operator.Task.Name = newName
				}

				newIsPaused, hasIsPaused := update.Update["isPaused"].(bool)
				if hasIsPaused {
					operator.Task.IsPaused = newIsPaused
				}

				newRunAfter, hasRunAfter := update.Update["runAfter"].(time.Time)
				if hasRunAfter {
					operator.Task.RunAfter = newRunAfter
				}

				newIsComplete, hasIsComplete := update.Update["isComplete"].(bool)
				if hasIsComplete {
					operator.Task.IsComplete = newIsComplete
				}

				newRemainingAttempts, hasRemainingAttempts := update.Update["remainingAttempts"].(int)
				if hasRemainingAttempts {
					operator.Task.RemainingAttempts = newRemainingAttempts
				}

				newInput, hasInput := update.Update["input"]
				if hasInput {
					operator.Task.Input = newInput
				}

				newOutput, hasOutput := update.Update["output"]
				if hasOutput {
					operator.Task.Output = newOutput
				}

				newErrors, hasErrors := update.Update["errors"].([]interface{})
				if hasErrors {
					operator.Task.Errors = newErrors
				}

				// TODO - handle additional fields

				// persist the change
				saveError := operator.TaskGroup.Storage.SaveTask(operator.TaskGroup, operator.Task)

				// Let anyone waiting on the update (rest api) know that the update has been persisted
				if update.UpdateComplete != nil {
					select {
					case update.UpdateComplete <- saveError:
					default:
						// Ignore no update complete listener...
					}
				}

				// emit update event
				operator.TaskGroup.Controller.ProcessTaskUpdate(TaskUpdateEvent{
					Event: "update",
					Task:  *operator.Task,
				})

				operator.Evaluate()

			case <-operator.ParentCompleteEvents: // parentTask :=
				operator.Evaluate()

			case isShuttingDown := <-operator.Shutdown:
				if isShuttingDown {
					// Save immediately once we get a shutdown request
					operator.TaskGroup.Storage.SaveTask(operator.TaskGroup, operator.Task)

					// if operator.Task.BusyExecuting = true, wait up to 60 seconds till false
					if operator.Task.BusyExecuting {
						waitCount := 0
						for waitCount < 10 {
							time.Sleep(10 * time.Second)
							waitCount++
						}
					}

					// Save again after any pending execution id complete
					operator.TaskGroup.Storage.SaveTask(operator.TaskGroup, operator.Task)

					operator.CancelExecute()
					operator.Terminated <- true
					return
				}
			}
		}
	}()
	operator.Evaluate()
}

// Evaluate determines if a Task is eligible to be executed and begins
// an execution timer if it is.
func (operator *TaskOperator) Evaluate() {
	fmt.Println("Evaluate")
	// Task execution workflow starts here!
	// This code needs to run whenever exciting things happen to a task:
	// - Task is paused/unpaused
	// - Task is Modified
	// - Task's group is paused/unpaused
	// - One of task's parents have completed
	// ...

	// Check if task is ready to execute:
	taskCanExecute := operator.Task.CanExecute(operator.TaskGroup)

	// If task is good to go, create an execution timer (if one doesn't exist already)
	if taskCanExecute {
		operator.CancelEvaluate()
		now := time.Now()
		if now.Before(operator.Task.RunAfter) {
			// Task's run after has not passed
			operator.ExecuteTimer.Reset(operator.Task.RunAfter.Sub(now))
			// operator.ExecuteTimer = time.NewTimer(operator.Task.RunAfter.Sub(now))
		} else {
			// Task's run after has already passed or was not set
			operator.ExecuteTimer.Reset(time.Millisecond)
			// operator.ExecuteTimer = time.NewTimer(time.Second)
		}
	}

	// If task is NOT good to go, kill execute timer if it exists
	if !taskCanExecute {
		// If there was a timer setup to execute the task, stop it
		operator.CancelExecute()
	}
}

// CancelExecute cancels a task's execution.  Will not terminate tasks that are
// already being executed.
func (operator *TaskOperator) CancelExecute() {
	// Stop and drain timer
	if !operator.ExecuteTimer.Stop() {
		select {
		case <-operator.ExecuteTimer.C:
		default:
		}
	}
}

// CancelEvaluate cancels a task's evaluation.
func (operator *TaskOperator) CancelEvaluate() {
	// Stop and drain timer
	if !operator.EvaulateTimer.Stop() {
		select {
		case <-operator.EvaulateTimer.C:
		default:
		}
	}
}

// ResetTask modifies the state of a task as if it has never been executed.
// Does not change IsPaused.
func (operator *TaskOperator) ResetTask(remainingAttempts int, updateComplete chan error) {
	operator.ExternalUpdates <- TaskUpdate{
		Update: map[string]interface{}{
			"remainingAttempts": remainingAttempts,
			"isComplete":        false,
			"output":            nil,
			"errors":            make([]interface{}, 0),
			"runAfter":          time.Now(),
		},
		UpdateComplete: updateComplete,
	}
}

// Execute sends a task to a worker and then processes the response or error.
func (operator *TaskOperator) Execute() {
	// This func does the work of making http call, processing result or error
	if (*operator.Task).CanExecute(operator.TaskGroup) {

		operator.Task.BusyExecuting = true

		select {
		case operator.Executing <- true:
			fmt.Println("sent executing(true) event")
		default:
			fmt.Println("no executing event sent")
		}

		// emit update event
		operator.TaskGroup.Controller.ProcessTaskUpdate(TaskUpdateEvent{
			Event: "update",
			Task:  *operator.Task,
		})

		workerResponse, err := operator.Client.Post(operator.Task, operator.TaskGroup)

		// decrement attempts
		operator.Task.RemainingAttempts--

		// capture output
		operator.Task.Output = workerResponse.Output

		// Error can come from two places : 1) Processing response from worker, a normal go error 2) An error in the response from the worker
		// These first two conditions look for each possibility
		if err != nil {
			// Capture Error
			operator.Task.Errors = append(operator.Task.Errors, err)

			// Setup another evaluation after error delay
			if operator.Task.RemainingAttempts > 0 {
				operator.EvaulateTimer.Reset(time.Duration(operator.Task.ErrorDelayInSeconds * int(time.Second)))
			}
		} else if workerResponse.Error != nil {
			// Capture Error
			operator.Task.Errors = append(operator.Task.Errors, workerResponse.Error)

			// Setup another evaluation after error delay
			if operator.Task.RemainingAttempts > 0 {
				operator.EvaulateTimer.Reset(time.Duration(operator.Task.ErrorDelayInSeconds * int(time.Second)))
			}
		} else {
			childrenOk := true
			// Create child tasks
			if len(workerResponse.Children) > 0 {
				// Children have to be created in order so that parents exist before children exist
				// TaskGroup's AddTask throws an error if a task's parents aren't found
				// We can use that here by iteratively trying to create children till they're all done
				// If we get stuck then something is wrong with structure of children and we should record an error
				createdChildren := 0
				lastCreatedChildren := 0
				expectedChildren := len(workerResponse.Children)
				for createdChildren < expectedChildren {
					for _, child := range workerResponse.Children {
						// If worker didn't specify that current task is parent of the children, add current task as a parent
						currentTaskIsParent := false
						for _, parentId := range child.ParentIds {
							if parentId == operator.Task.Id {
								currentTaskIsParent = true
							}
						}
						if !currentTaskIsParent {
							child.ParentIds = append(child.ParentIds, operator.Task.Id)
						}
						if workerResponse.ChildrenDelayInSeconds > 0 {
							child.RunAfter = time.Now().Add(time.Duration(workerResponse.ChildrenDelayInSeconds * int(time.Second)))
						}
						if workerResponse.WorkgroupDelayInSeconds > 0 && operator.Task.Workgroup != "" && child.Workgroup == operator.Task.Workgroup {
							child.RunAfter = time.Now().Add(time.Duration(workerResponse.WorkgroupDelayInSeconds * int(time.Second)))
						}

						// Add task will error if child exists or parents are missing
						// Add task also emits TaskUpdates for us
						err := operator.TaskGroup.AddTask(child, operator.Client)
						if err == nil {
							createdChildren++
						}
					}
					// If number of createdChildren didn't change on a pass (and we're not done) then we have a corrupt parent/child structure in children
					if createdChildren != lastCreatedChildren {
						if createdChildren < len(workerResponse.Children) {
							// Something went wrong creating the children
							// Un-create children from above so we don't leave a half baked structure
							for _, child := range workerResponse.Children {
								operator.TaskGroup.DeleteTask(child.Id)
							}
							childrenOk = false
						}
						break
					}
					lastCreatedChildren = createdChildren
				}
			}

			if childrenOk {
				operator.Task.IsComplete = true
				// Complete all other tasks with matching key
				if operator.Task.Key != "" {
					for _, keySiblingOperator := range operator.TaskGroup.TaskOperators {
						if (keySiblingOperator.Task.Key == operator.Task.Key) && (keySiblingOperator.Task.Id != operator.Task.Id) {
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
			} else {
				// Children not ok
				operator.Task.IsComplete = false
				operator.Task.Errors = append(operator.Task.Errors, "Unable to create children - corrupt parent/child relationship.")
			}
		}

		if workerResponse.WorkgroupDelayInSeconds > 0 && operator.Task.Workgroup != "" {
			operator.TaskGroup.Controller.DelayWorkgroup(operator.Task.Workgroup, workerResponse.WorkgroupDelayInSeconds, operator.TaskGroup.Id)
		}

		if workerResponse.ChildrenDelayInSeconds > 0 {
			// Note, this is done here as well in addition to child.RunAfter= above because some tasks may have children that were pre-populated
			for _, child := range operator.Task.Children {
				if !child.IsComplete {
					childOp, found := operator.TaskGroup.TaskOperators[child.Id]
					if found {
						// Update runAfter for child
						childOp.ExternalUpdates <- TaskUpdate{
							Update: map[string]interface{}{
								"runAfter": time.Now().Add(time.Duration(workerResponse.ChildrenDelayInSeconds * int(time.Second))),
							},
							UpdateComplete: nil,
						}
					}
				}
			}
		}

		operator.Task.BusyExecuting = false

		// persist the task
		operator.TaskGroup.Storage.SaveTask(operator.TaskGroup, operator.Task)

		// emit update event
		operator.TaskGroup.Controller.ProcessTaskUpdate(TaskUpdateEvent{
			Event: "update",
			Task:  *operator.Task,
		})

		select {
		case operator.Executing <- false:
			fmt.Println("sent executing(false) event")
		default:
			fmt.Println("no executing event sent")
		}

		// When a task is completed, find all children and send their operator an ParentCompleteEvents
		if operator.Task.IsComplete {
			for _, child := range operator.Task.Children {
				operator.TaskGroup.TaskOperators[child.Id].ParentCompleteEvents <- operator.Task
			}
		}
	}
}

// CanExecute determines if a Task is in a state where it can be executed.
func (task *Task) CanExecute(taskGroup *TaskGroup) bool {
	// Task should not execute if
	// - it is already complete
	// - it is paused
	// - it has no remaining attempts
	// - its task group is paused
	// Note that we do not check runAfter here, task timing is handled by operator
	if task.IsDeleting || task.IsComplete || task.IsPaused || task.RemainingAttempts <= 0 {
		return false
	}

	// Task should not execute if any of its parents are incomplete
	for _, parentId := range task.ParentIds {
		parent := taskGroup.TaskOperators[parentId].Task
		if !parent.IsComplete {
			return false
		}
	}

	return true
}
