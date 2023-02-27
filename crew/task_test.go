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
	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G10", "Test", taskGroupController)

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
	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G11", "Test", taskGroupController)

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
	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G12", "Test", taskGroupController)
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

	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G13", "Test", taskGroupController)
	group.PreloadTasks([]*Task{&task}, &TaskTestClient{})
	group.Operate()

	// This test needs to wait for a TaskUpdate event to know when the task has been updated
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		<-group.Controller.TaskUpdates
		wg.Done()
	}()

	group.TaskOperators[task.Id].ExternalUpdates <- TaskUpdate{
		Update: map[string]interface{}{
			"name": "New Name",
		},
		UpdateComplete: nil,
	}
	wg.Wait()

	if task.Name != "New Name" {
		t.Fatalf(`Task.Name = %v, want %v`, task.Name, "New Name")
	}

	group.TaskOperators[task.Id].Shutdown <- true
}

func TestCanResetTask(t *testing.T) {
	var errors []interface{}
	errors = append(errors, "Internal server error")
	originalRunAfter := time.Now().Add(-1 * time.Second)
	task := Task{
		Id:                "T21",
		TaskGroupId:       "G21",
		Name:              "Reset Task",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T21",
		RemainingAttempts: 0,
		IsPaused:          true,
		IsComplete:        true,
		Output:            map[string]interface{}{"ouput": "stuff"},
		Errors:            errors,
		Priority:          1,
		ProgressWeight:    1,
		RunAfter:          originalRunAfter,
	}

	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G21", "Test", taskGroupController)
	group.PreloadTasks([]*Task{&task}, &TaskTestClient{})
	group.Operate()

	updateComplete := make(chan error)
	group.TaskOperators["T21"].ResetTask(5, updateComplete)
	<-updateComplete

	if task.IsComplete != false {
		t.Fatalf(`Task.IsComplete = %v, want %v`, task.IsComplete, false)
	}
	if task.RemainingAttempts != 5 {
		t.Fatalf(`Task.RemainingAttempts = %v, want %v`, task.RemainingAttempts, 5)
	}
	if task.Output != nil {
		t.Fatalf(`Task.Output = %v, want %v`, task.Output, nil)
	}
	if len(task.Errors) != 0 {
		t.Fatalf(`len(task.Errors) = %v, want %v`, len(task.Errors), 0)
	}
	if !task.RunAfter.After(originalRunAfter) {
		t.Fatalf("Task RunAfter was not reset")
	}

	group.TaskOperators[task.Id].Shutdown <- true
}
