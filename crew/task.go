package crew

import (
	"fmt"
	"time"
)

type UpdateTaskMessage struct {
	ToTaskId string
	Update   map[string]interface{}
}

type ExecutedMessage struct {
	TaskId                  string
	Worker                  string
	Workgroup               string
	Key                     string
	WorkgroupDelayInSeconds int
	ChildrenDelayInSeconds  int
	IsComplete              bool
}

type ExecuteMessage struct {
	ToTaskId string
	Worker   string
}

type ParentCompletedMessage struct {
	ToTaskId string
	ParentId string
	Worker   string
	Input    interface{}
	Output   interface{}
}

type RefreshTaskIndexesMessage struct {
	Task *Task
}

type DeleteTaskMessage struct {
	ToTaskId string
}

type ContinuationMessage struct {
	Task *Task
}

type ChildIntroductionMessage struct {
	ToTaskId string
	ChildId  string
}

type ChildIntroductionAcknowledgementMessage struct {
	ToTaskId   string
	ParentId   string
	IsComplete bool
}

type TaskUpdatedMessage struct {
	Event string `json:"type"`
	Task  *Task  `json:"task"`
}

type DeduplicateTaskMessage struct {
	ToTaskId string
	Output   interface{}
}

type DelayTaskMessage struct {
	ToTaskId       string
	DelayInSeconds int
}

type ParentState struct {
	ParentId          string
	Worker            string
	Input             interface{}
	Output            interface{}
	IsComplete        bool
	IntroSent         bool
	IntroAcknowledged bool
}

// TaskClient defines the interface for delivering tasks to workers.
type TaskClient interface {
	Post(task *Task) (response WorkerResponse, err error)
}

type ChildTask struct {
	Id                  string      `json:"id"`
	Name                string      `json:"name"`
	Worker              string      `json:"worker"`
	Workgroup           string      `json:"workgroup"`
	Key                 string      `json:"key"`
	RemainingAttempts   int         `json:"remainingAttempts"`
	IsPaused            bool        `json:"isPaused"`
	IsComplete          bool        `json:"isComplete"`
	RunAfter            time.Time   `json:"runAfter"`
	ErrorDelayInSeconds int         `json:"errorDelayInSeconds"`
	Input               interface{} `json:"input"`
	ParentIds           []string    `json:"parentIds"`
}

// WorkerResponse defines the schema of output returned from workers.
type WorkerResponse struct {
	Output                  interface{}  `json:"output"`
	Children                []*ChildTask `json:"children"`
	WorkgroupDelayInSeconds int          `json:"workgroupDelayInSeconds"`
	ChildrenDelayInSeconds  int          `json:"childrenDelayInSeconds"`
	Error                   interface{}  `json:"error"`
}

// A Task represents a unit of work that can be completed by a worker.
type Task struct {
	Id                  string                 `json:"id"`
	GroupId             string                 `json:"groupId"`
	Name                string                 `json:"name"`
	Worker              string                 `json:"worker"`
	Workgroup           string                 `json:"workgroup"`
	Key                 string                 `json:"key"`
	RemainingAttempts   int                    `json:"remainingAttempts"`
	IsPaused            bool                   `json:"isPaused"`
	IsComplete          bool                   `json:"isComplete"`
	RunAfter            time.Time              `json:"runAfter"`
	IsSeed              bool                   `json:"isSeed"`
	ErrorDelayInSeconds int                    `json:"errorDelayInSeconds"`
	Input               interface{}            `json:"input"`
	Output              interface{}            `json:"output"`
	Errors              []string               `json:"errors"`
	CreatedAt           time.Time              `json:"createdAt"`
	ParentIds           []string               `json:"parentIds"`
	BusyExecuting       bool                   `json:"busyExecuting"`
	IsDeleting          bool                   `json:"-"`
	Inbox               chan interface{}       `json:"-"`
	Outbox              chan interface{}       `json:"-"`
	ChildIds            []string               `json:"-"`
	ParentStates        map[string]ParentState `json:"-"`
	ExecuteTimer        *time.Timer            `json:"-"`
	Client              TaskClient             `json:"-"`
	Storage             TaskStorage            `json:"-"`
	Throttler           *Throttler             `json:"-"`
	Running             bool                   `json:"-"`
}

// NewTask creates a new Task.
func NewTask() *Task {
	task := Task{
		Id:                  "",
		GroupId:             "",
		Name:                "",
		Worker:              "",
		Workgroup:           "",
		Key:                 "",
		RemainingAttempts:   5,
		IsPaused:            false,
		IsComplete:          false,
		RunAfter:            time.Now(),
		IsSeed:              false,
		ErrorDelayInSeconds: 0,
		Input:               nil,
		Output:              nil,
		Errors:              make([]string, 0),
		CreatedAt:           time.Now(),
		ParentIds:           make([]string, 0),
		BusyExecuting:       false,
		IsDeleting:          false,
		Inbox:               make(chan interface{}, 8),
		ChildIds:            make([]string, 0),
		Running:             false,
	}
	return &task
}

func (task *Task) Start(client TaskClient, storage TaskStorage, throttler *Throttler, outbox chan interface{}) {
	task.Client = client
	task.Outbox = outbox
	task.Storage = storage
	task.Throttler = throttler

	execTimer := time.NewTimer(1000 * time.Second)
	execTimer.Stop()
	task.ExecuteTimer = execTimer

	parentStates := make(map[string]ParentState)
	for _, parentId := range task.ParentIds {
		parentStates[parentId] = ParentState{
			ParentId:          parentId,
			IsComplete:        false,
			IntroSent:         false,
			IntroAcknowledged: false,
		}
	}
	task.ParentStates = parentStates

	// Watch the execute timer:
	go func() {
		for _ = range task.ExecuteTimer.C {
			// Exec message for self is still sent via outbox so
			// that the layer above can perform throttling.
			task.Outbox <- ExecuteMessage{
				ToTaskId: task.Id,
				Worker:   task.Worker,
			}
		}
	}()

	// Entire task lifecycle occurs in this goroutine:
	go func() {
		task.Running = true
		// Process messages:
		for message := range task.Inbox {
			switch v := message.(type) {
			case UpdateTaskMessage:
				task.Update(message.(UpdateTaskMessage).Update)
			case ParentCompletedMessage:
				task.ParentCompleted(message.(ParentCompletedMessage))
			case DeleteTaskMessage:
				task.Delete()
			case ChildIntroductionMessage:
				task.ChildIntroduction(message.(ChildIntroductionMessage).ChildId)
			case ChildIntroductionAcknowledgementMessage:
				task.ChildIntroductionAcknowledgement(message.(ChildIntroductionAcknowledgementMessage).ParentId, message.(ChildIntroductionAcknowledgementMessage).IsComplete)
			case DeduplicateTaskMessage:
				task.Deduplicate(message.(DeduplicateTaskMessage).Output)
			case DelayTaskMessage:
				task.Delay(message.(DelayTaskMessage).DelayInSeconds)
			default:
				fmt.Printf("I don't know how to handle message of type %T!\n", v)
			}
		}
		// Channel closed => system shutting down
		task.Running = false
		task.Stop()
	}()

	// Send intro to parents (make sure to set ack sent)
	for _, parentId := range task.ParentIds {
		task.Outbox <- ChildIntroductionMessage{
			ToTaskId: parentId,
			ChildId:  task.Id,
		}
		parentState := task.ParentStates[parentId]
		parentState.IntroSent = true
	}
}

func (task *Task) Delay(delayInSeconds int) {
	task.CancelExecute()
	task.RunAfter = time.Now().Add(time.Duration(delayInSeconds) * time.Second)
	task.Save()
	task.Evaluate()
}

func (task *Task) Deduplicate(output interface{}) {
	task.CancelExecute()
	task.Output = output
	task.IsComplete = true
	task.Save()
}

func (task *Task) Update(update map[string]interface{}) {
	shouldReIndex := false
	newName, hasNewName := update["name"].(string)
	if hasNewName {
		task.Name = newName
	}

	newWorker, hasNewWorker := update["worker"].(string)
	if hasNewWorker {
		task.Worker = newWorker
	}

	newWorkgroup, hasNewWorkgroup := update["workgroup"].(string)
	if hasNewWorkgroup && task.Workgroup != newWorkgroup {
		task.Workgroup = newWorkgroup
		shouldReIndex = true
	}

	newKey, hasNewKey := update["key"].(string)
	if hasNewKey && task.Key != newKey {
		task.Key = newKey
		shouldReIndex = true
	}

	newIsPaused, hasIsPaused := update["isPaused"].(bool)
	if hasIsPaused {
		task.IsPaused = newIsPaused
	}

	newRunAfter, hasRunAfter := update["runAfter"]
	if hasRunAfter {
		switch t := newRunAfter.(type) {
		case time.Time:
			task.RunAfter = t
		default:
			task.RunAfter = time.Time{}
		}
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
	task.Save()
	task.Evaluate()

	if shouldReIndex {
		task.Outbox <- RefreshTaskIndexesMessage{
			Task: task,
		}
	}
}

func (task *Task) ParentCompleted(message ParentCompletedMessage) {
	parentState := task.ParentStates[message.ParentId]
	parentState.IsComplete = true
	parentState.Input = message.Input
	parentState.Output = message.Output
	parentState.Worker = message.Worker
	task.Evaluate()
}

func (task *Task) Delete() {
	task.IsDeleting = true
	task.Stop()
	task.Storage.DeleteTask(task)
	task.EmitUpdate("delete")
}

func (task *Task) Stop() {
	// TODO - What do we do if task is currently executing?
	task.CancelExecute()
	if task.Running {
		close(task.Inbox)
	}
}

func (task *Task) Save() {
	task.Storage.SaveTask(task)
	task.EmitUpdate("update")
}

func (task *Task) EmitUpdate(event string) {
	task.Outbox <- TaskUpdatedMessage{
		Event: event,
		Task:  task,
	}
}

func (task *Task) Evaluate() {
	// Task execution workflow starts here!
	// This code needs to run whenever exciting things happen to a task:
	// - Task is paused/unpaused
	// - Task is Modified
	// - Task's group is paused/unpaused
	// - One of task's parents have completed

	// Check if task is ready to execute:
	taskCanExecute := task.CanExecute()

	// If task is good to go, create an execution timer (if one doesn't exist already)
	if taskCanExecute {
		now := time.Now()
		if now.Before(task.RunAfter) {
			// Task's run after has not passed
			task.ExecuteTimer.Reset(task.RunAfter.Sub(now))
		} else {
			// Task's run after has already passed or was not set
			task.ExecuteTimer.Reset(time.Millisecond)
		}
	}

	// If task is NOT good to go, kill execute timer if it exists
	if !taskCanExecute {
		// If there was a timer setup to execute the task, stop it
		task.CancelExecute()
	}
}

// CancelExecute cancels a task's execution.  Will not terminate tasks that are
// already being executed.
func (task *Task) CancelExecute() {
	// Stop and drain timer
	if !task.ExecuteTimer.Stop() {
		select {
		case <-task.ExecuteTimer.C:
		default:
		}
	}
}

func (task *Task) CanExecute() bool {
	// Task should not execute if
	// - it is already complete
	// - it is paused
	// - it has no remaining attempts
	// - its task group is paused
	// Note that we do not check runAfter here, task timing is handled by operator
	if task.IsComplete || task.IsPaused || task.RemainingAttempts <= 0 {
		return false
	}

	if task.Worker == "" {
		return false
	}

	// Task should not execute if any of its parents are incomplete
	for _, parentState := range task.ParentStates {
		if !parentState.IsComplete {
			return false
		}
	}

	return true
}

func (task *Task) ChildIntroduction(childId string) {
	// Add this child to our list of ids
	task.ChildIds = append(task.ChildIds, childId)

	// Send ack to child
	task.Outbox <- ChildIntroductionAcknowledgementMessage{
		ToTaskId:   childId,
		ParentId:   task.Id,
		IsComplete: task.IsComplete,
	}
}

func (task *Task) ChildIntroductionAcknowledgement(parentId string, isComplete bool) {
	// Record parent's state
	parentState := task.ParentStates[parentId]
	parentState.IntroAcknowledged = true
	parentState.IsComplete = isComplete

	task.Evaluate()
}

func (task *Task) Execute() {
	// TODO
	// Note - send ParentCompletedMessage for each child

	if task.CanExecute() {
		task.BusyExecuting = true

		// We don't save BusyExecuting to storage, so send update to clients
		task.EmitUpdate("update")

		// Apply worker throttling if a throttler is defined
		if (task.Throttler != nil) && (task.Worker != "") {
			query := ThrottlePushQuery{
				TaskId: task.Id,
				Worker: task.Worker,
				Resp:   make(chan bool)}
			task.Throttler.Push <- query
			// Block until throttler says it is ok to send task request
			<-query.Resp
		}

		workerResponse, err := task.Client.Post(task)

		if (task.Throttler != nil) && (task.Worker != "") {
			query := ThrottlePopQuery{
				TaskId: task.Id,
				Worker: task.Worker}
			// Let throttler know that task attempt is complete
			task.Throttler.Pop <- query
		}

		// decrement attempts
		task.RemainingAttempts--

		// capture output
		task.Output = workerResponse.Output

		// Error can come from two places : 1) Processing response from worker, a normal go error 2) An error in the response from the worker
		// These first two conditions look for each possibility
		if err != nil {
			fmt.Println("~~ Got standard error", err)
			// Capture Error
			task.Errors = append(task.Errors, fmt.Sprintf("%v", err))

			// Setup another evaluation after error delay
			errorDelay := time.Duration(task.ErrorDelayInSeconds * int(time.Second))
			task.RunAfter = time.Now().Add(errorDelay)
		} else if workerResponse.Error != nil {
			fmt.Println("~~ Got worker response error", workerResponse.Error)
			// Capture Error
			task.Errors = append(task.Errors, fmt.Sprintf("%v", workerResponse.Error))

			// Setup another evaluation after error delay
			errorDelay := time.Duration(task.ErrorDelayInSeconds * int(time.Second))
			task.RunAfter = time.Now().Add(errorDelay)
		} else {
			// No error!
			task.IsComplete = true

			if len(workerResponse.Children) > 0 {
				// Inflate all children to task objects
				inflatedChildren := make([]*Task, 0)
				for _, child := range workerResponse.Children {
					inflatedChild := NewTask()

					// Copy over fields that can be set in children
					inflatedChild.Id = child.Id
					inflatedChild.Name = child.Name
					inflatedChild.Worker = child.Worker
					inflatedChild.Workgroup = child.Workgroup
					inflatedChild.Key = child.Key
					inflatedChild.RemainingAttempts = child.RemainingAttempts
					inflatedChild.IsPaused = child.IsPaused
					inflatedChild.IsComplete = child.IsComplete
					inflatedChild.RunAfter = child.RunAfter
					inflatedChild.ErrorDelayInSeconds = child.ErrorDelayInSeconds
					inflatedChild.Input = child.Input
					inflatedChild.ParentIds = child.ParentIds

					// Set fields that can't be set in children but need to be configured
					inflatedChild.GroupId = task.GroupId

					inflatedChildren = append(inflatedChildren, inflatedChild)
				}

				for _, child := range inflatedChildren {
					// If worker didn't specify at least one parent for the child, add current task as a parent
					if len(child.ParentIds) == 0 {
						child.ParentIds = append(child.ParentIds, task.Id)
					}
					if workerResponse.ChildrenDelayInSeconds > 0 {
						child.RunAfter = time.Now().Add(time.Duration(workerResponse.ChildrenDelayInSeconds * int(time.Second)))
					}
					if workerResponse.WorkgroupDelayInSeconds > 0 && task.Workgroup != "" && child.Workgroup == task.Workgroup {
						child.RunAfter = time.Now().Add(time.Duration(workerResponse.WorkgroupDelayInSeconds * int(time.Second)))
					}

					// Create child task
					task.Outbox <- ContinuationMessage{
						Task: child,
					}
				}
			}
		}

		task.BusyExecuting = false

		task.Storage.SaveTask(task)

		// Let pool (and throttler) know we're done executing
		task.Outbox <- ExecutedMessage{
			TaskId:                  task.Id,
			Worker:                  task.Worker,
			Workgroup:               task.Workgroup,
			IsComplete:              task.IsComplete,
			Key:                     task.Key,
			WorkgroupDelayInSeconds: workerResponse.WorkgroupDelayInSeconds,
			ChildrenDelayInSeconds:  workerResponse.ChildrenDelayInSeconds,
		}

		// Let all children know that we're done executing
		if task.IsComplete {
			for _, childId := range task.ChildIds {
				task.Outbox <- ParentCompletedMessage{
					ToTaskId: childId,
					ParentId: task.Id,
					Worker:   task.Worker,
					Input:    task.Input,
					Output:   task.Output,
				}
			}
		}

	} else {
		// TODO cannot execute
	}
}
