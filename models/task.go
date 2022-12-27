package models

import (
	"fmt"
	"time"
)

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
	Children            []*Task       `json:"-"`
	// parentsComplete
	// assignedTo
	// assignedAt
}

// A TaskUpdate defines the fields of a Task that can be modified.
type TaskUpdate struct {
	Name     string `json:"name"`
	IsPaused bool   `json:"isPaused"`
	// TODO - additional fields
}

// A TaskOperator manages the lifecycle and state of a Task.
type TaskOperator struct {
	Task                 *Task
	TaskGroup            *TaskGroup
	Channels             map[string]Channel
	Updates              chan TaskUpdate
	ExecuteTimer         *time.Timer
	Shutdown             chan bool
	ParentCompleteEvents chan *Task
	Operating            bool
	Executing            chan bool
	Complete             chan interface{} // emit output
	Error                chan interface{} // emit error
	Terminated           chan bool
}

func NewTaskOperator(task *Task, taskGroup *TaskGroup, channels map[string]Channel) *TaskOperator {
	t := TaskOperator{
		Task:                 task,
		TaskGroup:            taskGroup,
		Channels:             channels,
		Updates:              make(chan TaskUpdate, 8),
		ExecuteTimer:         time.NewTimer(time.Second * -1),
		Shutdown:             make(chan bool),
		ParentCompleteEvents: make(chan *Task, len(task.Children)),
		Executing:            make(chan bool),
		Complete:             make(chan interface{}),
		Error:                make(chan interface{}),
		Terminated:           make(chan bool),
	}
	// Don't let initial timer run
	t.TerminateExecute()
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

			case update := <-operator.Updates:
				operator.Task.Name = update.Name
				operator.Task.IsPaused = update.IsPaused
				// TODO - persist the change
				// TOOD - handle additional fields

				operator.TaskGroup.Events <- TaskGroupEvent{
					Type: "task_update",
					// TODO - use select to make non-blocking, wrap in function?
				}
				operator.Evaluate()

			case <-operator.ParentCompleteEvents: // parentTask :=
				operator.Evaluate()

			case isShuttingDown := <-operator.Shutdown:
				if isShuttingDown {
					// TODO, stop any resources, persist task...

					// TODO - if operator.Executing = true, wait till false

					operator.TerminateExecute()
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

	// TODO - check if task is ready to execute:
	// IsComplete = false
	// All parents IsComplete = true
	// runAfter passed
	// RemainingAttempts >0
	// etc...
	// TODO - move to Task.CanExecute()
	taskCanExecute := !operator.Task.IsComplete

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
		operator.TerminateExecute()
	}
}

// TerminateExecute cancels a task's execution.  Will not terminate tasks that are
// already being executed.
func (operator *TaskOperator) TerminateExecute() {
	// Stop and drain timer
	if !operator.ExecuteTimer.Stop() {
		<-operator.ExecuteTimer.C
	}
}

// Execute sends a task to a worker and then processes the response or error.
func (operator *TaskOperator) Execute() {
	// This func does the work of making http call, processing result or error
	if (*operator.Task).CanExecute(operator.TaskGroup) {

		select {
		case operator.Executing <- true:
			fmt.Println("sent executing(true) event")
		default:
			fmt.Println("no executing event sent")
		}
		operator.TaskGroup.Events <- TaskGroupEvent{
			Type: "task_executing",
			// TODO - payload
			// TODO - use select to make non-blocking, wrap in function?
		}

		channel := operator.Channels[operator.Task.Channel]
		fmt.Println("Timer fired!  Would make http call!", channel.Url)
		// Pretend sleep for http call
		time.Sleep(2 * time.Second)
		fmt.Println("Would finish http call!")

		// TODO - consider doing updates via update channel instead?
		operator.Task.IsComplete = true
		operator.TaskGroup.Events <- TaskGroupEvent{
			Type: "task_update",
			// TODO - payload
			// TODO - use select to make non-blocking, wrap in function?
		}

		// TODO - create child tasks
		// TODO - capture errors
		// TODO - capture output
		// TODO - complete all other tasks with matching key

		// When a task is completed, find all children and send their operator an ParentCompleteEvents
		for _, child := range operator.Task.Children {
			operator.TaskGroup.TaskOperators[child.Id].ParentCompleteEvents <- operator.Task
		}

		operator.TaskGroup.Events <- TaskGroupEvent{
			Type: "task_executing",
			// TODO - payload
			// TODO - use select to make non-blocking, wrap in function?
		}
		select {
		case operator.Executing <- false:
			fmt.Println("sent executing(false) event")
		default:
			fmt.Println("no executing event sent")
		}

		// Emit on complete (or error) last. Automated tests watch these channel to know when execute process has finished.
		select {
		case operator.Complete <- "Task output goes here...":
			fmt.Println("sent complete event")
		default:
			fmt.Println("no complete event sent")
		}

		// TODO
		// operator.Error <-
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
