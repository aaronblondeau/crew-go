package crew

import "time"

// A Task represents a unit of work that can be completed by a worker.
type Task struct {
	Id                  string      `json:"id"`
	TaskGroupId         string      `json:"taskGroupId"`
	Name                string      `json:"name"`
	Worker              string      `json:"worker"`
	Workgroup           string      `json:"workgroup"`
	Key                 string      `json:"key"`
	RemainingAttempts   int         `json:"remainingAttempts"`
	IsPaused            bool        `json:"isPaused"`
	IsComplete          bool        `json:"isComplete"`
	RunAfter            time.Time   `json:"runAfter"`
	IsSeed              bool        `json:"isSeed"`
	ErrorDelayInSeconds int         `json:"errorDelayInSeconds"`
	Input               interface{} `json:"input"`
	Output              interface{} `json:"output"`
	Errors              []string    `json:"errors"`
	CreatedAt           time.Time   `json:"createdAt"`
	ParentIds           []string    `json:"parentIds"`
	BusyExecuting       bool        `json:"busyExecuting"`
	Storage             TaskStorage `json:"-"`
}

// NewTask creates a new Task.
func NewTask() *Task {
	task := Task{
		Id:                  "",
		TaskGroupId:         "",
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
	}
	return &task
}
