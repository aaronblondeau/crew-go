package crew

import (
	"testing"
)

type TaskTestClient struct{}

func (client TaskTestClient) Post(task *Task) (response WorkerResponse, err error) {
	response = WorkerResponse{
		Output: map[string]interface{}{
			"demo": "Test Complete",
		},
		Children:                make([]*ChildTask, 0),
		Error:                   nil,
		WorkgroupDelayInSeconds: 0,
		ChildrenDelayInSeconds:  0,
	}
	return
}

func CreateTaskTestTask(taskNumber string) *Task {
	task := NewTask()
	task.Id = "T" + taskNumber
	task.GroupId = "G" + taskNumber
	task.Name = "Task" + taskNumber
	task.Worker = "worker-a"
	task.Workgroup = ""
	return task
}

func TestCanExecute(t *testing.T) {
	task := CreateTaskTestTask("1")
	canExecute := task.CanExecute()
	if !canExecute {
		t.Fatalf(`CanExecute() = false, want true`)
	}
}

func TestCannotExecuteIfTaskIsPaused(t *testing.T) {
	task := CreateTaskTestTask("2")
	task.IsPaused = true
	canExecute := task.CanExecute()
	if canExecute {
		t.Fatalf(`CanExecute() = true, want false (task is paused)`)
	}
}

func TestCannotExecuteIfParentsIncomplete(t *testing.T) {
	parent := CreateTaskTestTask("3")
	parent.IsComplete = false

	task := CreateTaskTestTask("4")
	task.ParentIds = []string{parent.Id}
	task.ParentStates = make(map[string]ParentState)
	task.ParentStates[parent.Id] = ParentState{
		ParentId:   parent.Id,
		IsComplete: false,
	}

	canExecute := task.CanExecute()
	if canExecute {
		t.Fatalf(`CanExecute() = true, want false (task is paused)`)
	}
}
