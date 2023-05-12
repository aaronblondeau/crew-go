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

func TestCanUpdateTask(t *testing.T) {
	task := CreateTaskTestTask("5")
	client := TaskTestClient{}
	storage := NewMemoryTaskStorage()
	outbox := make(chan interface{})
	task.Start(client, storage, nil, outbox)

	task.Inbox <- UpdateTaskMessage{
		ToTaskId: task.Id,
		Update: map[string]interface{}{
			"name": "New Name",
		},
	}

	// Wait for something to hit outbox (should be a task updated message)
	msg := <-outbox

	if task.Name != "New Name" {
		t.Fatalf(`Task.Name = %v, want %v`, task.Name, "New Name")
	}

	// msg should be a TaskUpdatedMessage
	_, msgTypeOk := msg.(TaskUpdatedMessage)
	if !msgTypeOk {
		t.Fatalf("msg is not a TaskUpdatedMessage")
	}
	task.Stop()
}
