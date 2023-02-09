package crew

import (
	"sync"
	"testing"
	"time"
)

type TaskTestClient struct{}

func (client TaskTestClient) Post(task *Task, taskGroup *TaskGroup) (response WorkerResponse, err error) {
	response = WorkerResponse{
		Output: map[string]interface{}{
			"demo": "Test Complete",
		},
		Children:                make([]*Task, 0),
		Error:                   nil,
		WorkgroupDelayInSeconds: 0,
		ChildrenDelayInSeconds:  0,
	}
	return
}

func TestCanExecute(t *testing.T) {
	task := Task{
		Id:                "T10",
		TaskGroupId:       "G10",
		Name:              "Task Ten",
		Worker:            "worker-a",
		Workgroup:         "",
		Key:               "T1",
		RemainingAttempts: 5,
		IsPaused:          false,
		IsComplete:        false,
		Priority:          1,
		ProgressWeight:    1,
	}
	group := NewTaskGroup("G10", "Test")

	group.AddTask(&task, &TaskTestClient{})

	canExecute := task.CanExecute(group)
	if !canExecute {
		t.Fatalf(`CanExecute() = false, want true`)
	}
}

func TestCannotExecuteIfTaskIsPaused(t *testing.T) {
	task := Task{
		Id:                "T11",
		TaskGroupId:       "G11",
		Name:              "Task Eleven",
		Worker:            "worker-a",
		Workgroup:         "",
		Key:               "T11",
		RemainingAttempts: 5,
		// Tasks cannot be executed if they are paused
		IsPaused:            true,
		IsComplete:          false,
		Priority:            1,
		ProgressWeight:      1,
		IsSeed:              false,
		ErrorDelayInSeconds: 5,
		Input:               "Test",
		Errors:              make([]interface{}, 0),
		CreatedAt:           time.Now(),
		ParentIds:           make([]string, 0),
		Children:            make([]*Task, 0),
	}
	group := NewTaskGroup("G11", "Test")

	canExecute := task.CanExecute(group)
	if canExecute {
		t.Fatalf(`CanExecute() = true, want false (task is paused)`)
	}
}

func TestCannotExecuteIfParentsIncomplete(t *testing.T) {
	parent := Task{
		Id:                "T12P",
		TaskGroupId:       "G12",
		Name:              "Incomplete Task Parent",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T12P",
		RemainingAttempts: 5,
		IsPaused:          false,
		IsComplete:        false,
		Priority:          1,
		ProgressWeight:    1,
	}

	task := Task{
		Id:                "T12C",
		TaskGroupId:       "G12",
		Name:              "Task One",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T12C",
		RemainingAttempts: 5,
		IsPaused:          false,
		IsComplete:        false,
		Priority:          1,
		ProgressWeight:    1,
		ParentIds:         []string{"T12P"},
	}

	group := NewTaskGroup("G12", "Test")
	group.PreloadTasks([]*Task{&parent, &task}, &TaskTestClient{})

	canExecute := task.CanExecute(group)
	if canExecute {
		t.Fatalf(`CanExecute() = true, want false (parent not complete)`)
	}
}

func TestCanUpdateTask(t *testing.T) {
	task := Task{
		Id:                "T13",
		TaskGroupId:       "G13",
		Name:              "Task Thirteen",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T13",
		RemainingAttempts: 5,
		IsPaused:          true,
		IsComplete:        false,
		Priority:          1,
		ProgressWeight:    1,
		ParentIds:         []string{"T1"},
	}

	group := NewTaskGroup("G13", "Test")
	group.PreloadTasks([]*Task{&task}, &TaskTestClient{})
	group.Operate()

	// This test needs to wait for a TaskUpdate event to know when the task has been updated
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		<-group.TaskUpdates
		wg.Done()
	}()

	group.TaskOperators[task.Id].ExternalUpdates <- map[string]interface{}{
		"name": "New Name",
	}
	wg.Wait()

	if task.Name != "New Name" {
		t.Fatalf(`Task.Name = %v, want %v`, task.Name, "New Name")
	}

	group.TaskOperators[task.Id].Shutdown <- true
}