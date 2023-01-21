package crew

import (
	"fmt"
	"time"
)

type TaskClient interface {
	Post(URL string, input interface{}) (output interface{}, children []*Task, err error) // output, children, error
}

// A Task represents a unit of work that can be completed by a worker.
type Task struct {
	Id                  string        `json:"id"`
	TaskGroupId         string        `json:"taskGroupId"`
	Name                string        `json:"name"`
	Channel             string        `json:"channel"`
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
	// parentsComplete
	// assignedTo
	// assignedAt
}

// A TaskOperator manages the lifecycle and state of a Task.
type TaskOperator struct {
	Task                 *Task
	TaskGroup            *TaskGroup
	ExternalUpdates      chan map[string]interface{}
	ExecuteTimer         *time.Timer
	Shutdown             chan bool
	ParentCompleteEvents chan *Task
	Operating            bool
	Executing            chan bool
	Terminated           chan bool
	Client               TaskClient
}

func NewTaskOperator(task *Task, taskGroup *TaskGroup, channels map[string]Channel, client TaskClient) *TaskOperator {
	t := TaskOperator{
		Task:                 task,
		TaskGroup:            taskGroup,
		ExternalUpdates:      make(chan map[string]interface{}, 8),
		ExecuteTimer:         time.NewTimer(time.Second * -1),
		Shutdown:             make(chan bool),
		ParentCompleteEvents: make(chan *Task, len(task.Children)),
		Executing:            make(chan bool),
		Terminated:           make(chan bool),
		Client:               client,
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

			case update := <-operator.ExternalUpdates:
				newName, hasNewName := update["name"].(string)
				if hasNewName {
					operator.Task.Name = newName
				}

				newIsPaused, hasIsPaused := update["isPaused"].(bool)
				if hasIsPaused {
					operator.Task.IsPaused = newIsPaused
				}

				// TODO - persist the change
				// TODO - handle additional fields

				select {
				case operator.TaskGroup.TaskUpdates <- TaskUpdateEvent{
					Event: "update",
					Task:  *operator.Task,
				}:
					fmt.Println("sent executing(false) event")
				default:
					fmt.Println("no task_update event sent")
				}
				operator.Evaluate()

			case <-operator.ParentCompleteEvents: // parentTask :=
				operator.Evaluate()

			case isShuttingDown := <-operator.Shutdown:
				if isShuttingDown {
					// TODO, stop any resources, persist task...

					// if operator.Task.BusyExecuting = true, wait up to 60 seconds till false
					if operator.Task.BusyExecuting {
						waitCount := 0
						for waitCount < 10 {
							time.Sleep(10 * time.Second)
							waitCount++
						}
					}

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
		<-operator.ExecuteTimer.C
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

		select {
		case operator.TaskGroup.TaskUpdates <- TaskUpdateEvent{
			Event: "update",
			Task:  *operator.Task,
		}:
		default:
		}

		channel := operator.TaskGroup.Channels[operator.Task.Channel]
		fmt.Println("Timer fired!  Sending to client:", channel.Url)

		output, children, err := operator.Client.Post(channel.Url, operator.Task.Input)

		// decrement attempts
		operator.Task.RemainingAttempts--

		// capture output
		operator.Task.Output = output

		if err != nil {
			// Capture Error
			operator.Task.Errors = append(operator.Task.Errors, err)
		} else {
			childrenOk := true
			// Create child tasks
			if len(children) > 0 {
				// Children have to be created in order so that parents exist before children exist
				// TaskGroup's AddTask throws an error if a task's parents aren't found
				// We can use that here by iteratively trying to create children till they're all done
				// If we get stuck then something is wrong with structure of children and we should record an error
				createdChildren := 0
				lastCreatedChildren := 0
				expectedChildren := len(children)
				for createdChildren < expectedChildren {
					for _, child := range children {
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

						// Add task will error if child exists or parents are missing
						// Add task also emits TaskUpdates for us
						err := operator.TaskGroup.AddTask(child, operator.Client)
						if err == nil {
							createdChildren++
						}
					}
					// If number of createdChildren didn't change on a pass then we have a corrupt parent/child structure in children
					if createdChildren != lastCreatedChildren {
						// Un-create children from above so we don't leave a half baked structure
						for _, child := range children {
							operator.TaskGroup.DeleteTask(child.Id)
						}
						childrenOk = false
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
						if keySiblingOperator.Task.Key == operator.Task.Key {
							keySiblingOperator.Task.IsComplete = true
							keySiblingOperator.Task.Output = output

							// TODO - persist keySiblingOperator.Task

							select {
							case operator.TaskGroup.TaskUpdates <- TaskUpdateEvent{
								Event: "update",
								Task:  *keySiblingOperator.Task,
							}:
							default:
							}
						}
					}
				}

				// When a task is completed, find all children and send their operator an ParentCompleteEvents
				for _, child := range operator.Task.Children {
					operator.TaskGroup.TaskOperators[child.Id].ParentCompleteEvents <- operator.Task
				}
			} else {
				// Children not ok
				operator.Task.IsComplete = false
				operator.Task.Errors = append(operator.Task.Errors, "Unable to create children - corrupt parent/child relationship.")
			}
		}

		operator.Task.BusyExecuting = false

		// TODO - persist task
		select {
		case operator.TaskGroup.TaskUpdates <- TaskUpdateEvent{
			Event: "update",
			Task:  *operator.Task,
		}:
		default:
		}

		select {
		case operator.Executing <- false:
			fmt.Println("sent executing(false) event")
		default:
			fmt.Println("no executing event sent")
		}
	}
}

// CanExecute determines if a Task is in a state where it can be executed.
func (task Task) CanExecute(taskGroup *TaskGroup) bool {
	// Task should not execute if
	// - it is already complete
	// - it is paused
	// - it has no remaining attempts
	// - its task group is paused
	// Note that we do not check runAfter here, task timing is handled by operator
	if task.IsComplete || task.IsPaused || task.RemainingAttempts <= 0 || taskGroup.IsPaused {
		return false
	}

	// Task should not execute if any of its parents are incomplete
	for _, parentId := range task.ParentIds {
		parent := taskGroup.TaskOperators[parentId].Task
		if !parent.IsComplete {
			return false
		}
	}

	// TODO add check for channel, cannot execute if no channel
	// Along with this, setup chan in operator to notify and re-evaluate when channels are added/removed

	return true
}
