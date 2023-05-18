package crew

import "time"

// TaskGroup represents a group of tasks.
type TaskGroup struct {
	Id        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

// NewTaskGroup creates a new TaskGroup.
func NewTaskGroup(id string, name string) *TaskGroup {
	tg := TaskGroup{
		Id:        id,
		Name:      name,
		CreatedAt: time.Now(),
	}
	return &tg
}
