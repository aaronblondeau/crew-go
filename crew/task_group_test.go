package crew

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

type TaskGroupTestClient struct{}

func (client TaskGroupTestClient) Post(task *Task, taskGroup *TaskGroup) (response WorkerResponse, err error) {
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

func TestPrepareInflatesChildren(t *testing.T) {
	parent := Task{
		Id:                "T7A",
		TaskGroupId:       "G7",
		Name:              "Incomplete Task Parent",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T7A",
		RemainingAttempts: 5,
		IsPaused:          false,
		IsComplete:        false,
		ProgressWeight:    1,
	}

	task := Task{
		Id:                "T7B",
		TaskGroupId:       "G7",
		Name:              "Child",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T7B",
		RemainingAttempts: 5,
		IsPaused:          false,
		IsComplete:        false,
		ProgressWeight:    1,
		ParentIds:         []string{"T7A"},
	}
	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G7", "Test", taskGroupController)
	group.PreloadTasks([]*Task{&parent, &task}, &TaskGroupTestClient{})

	if parent.Children[0] != &task {
		t.Fatal("Parent task's Children slice was not inflated!")
	}
}

func TestCanDeleteTask(t *testing.T) {
	task := Task{
		Id:                "T8",
		TaskGroupId:       "G8",
		Name:              "Task Eight",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T8",
		RemainingAttempts: 5,
		IsPaused:          true,
		IsComplete:        false,
		ProgressWeight:    1,
		ParentIds:         []string{},
	}
	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G8", "Test", taskGroupController)
	group.PreloadTasks([]*Task{&task}, &TaskGroupTestClient{})
	// group.Operate()

	if len(group.TaskOperators) != 1 {
		t.Errorf("len(group.TaskOperators) = %d; want 1", len(group.TaskOperators))
	}

	group.DeleteTask(task.Id)

	if len(group.TaskOperators) != 0 {
		t.Errorf("len(group.TaskOperators) = %d; want 0", len(group.TaskOperators))
	}
}

func TestCanAddTask(t *testing.T) {
	task := Task{
		Id:                "T9",
		TaskGroupId:       "G9",
		Name:              "Task Nine",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T9",
		RemainingAttempts: 5,
		// Start task as paused
		IsPaused:       true,
		IsComplete:     false,
		ProgressWeight: 1,
		ParentIds:      []string{},
	}

	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G9", "Test", taskGroupController)

	if len(group.TaskOperators) != 0 {
		t.Errorf("len(group.TaskOperators) = %d; want 0", len(group.TaskOperators))
	}

	group.PreloadTasks([]*Task{}, &TaskGroupTestClient{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for event := range group.Controller.TaskUpdates {
			fmt.Println("Got an update!", event.Event, event.Task.IsComplete)
			if event.Task.IsComplete {
				wg.Done()
				return
			}
		}
	}()

	err := group.AddTask(&task, TaskGroupTestClient{})
	if err != nil {
		t.Fatalf("Got an unexpected error when adding task %v\n", err)
	}

	if len(group.TaskOperators) != 1 {
		t.Errorf("len(group.TaskOperators) = %d; want 1", len(group.TaskOperators))
	}

	// Unpause the task
	group.TaskOperators[task.Id].ExternalUpdates <- TaskUpdate{
		Update: map[string]interface{}{
			"isPaused": false,
		},
		UpdateComplete: nil,
	}

	// Wait for task to complete
	wg.Wait()

	if task.IsComplete != true {
		t.Fatalf(`Task.IsComplete = %v, want true`, task.IsComplete)
	}
}

func TestCanResetTaskGroupNoSeeds(t *testing.T) {
	var errors []interface{}
	errors = append(errors, "Internal server error")
	originalRunAfter := time.Now().Add(-1 * time.Second)
	task := Task{
		Id:                "T22",
		TaskGroupId:       "G22",
		Name:              "Reset Task",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T22",
		RemainingAttempts: 0,
		IsPaused:          true,
		IsComplete:        true,
		IsSeed:            false,
		Output:            map[string]interface{}{"ouput": "stuff"},
		Errors:            errors,
		ProgressWeight:    1,
		RunAfter:          originalRunAfter,
	}

	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G22", "Test", taskGroupController)
	group.PreloadTasks([]*Task{&task}, &TaskTestClient{})
	group.Operate()

	updateComplete := make(chan error)
	group.Reset(5, updateComplete)
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

func TestCanResetTaskGroupWithSeeds(t *testing.T) {
	task1 := Task{
		Id:                "T23A",
		TaskGroupId:       "G23",
		Name:              "Reset Task",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T23A",
		RemainingAttempts: 0,
		IsPaused:          true,
		IsComplete:        true,
		IsSeed:            true,
		ProgressWeight:    1,
	}

	task2 := Task{
		Id:                "T23B",
		TaskGroupId:       "G23",
		Name:              "Reset Task",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T23B",
		RemainingAttempts: 0,
		IsPaused:          true,
		IsComplete:        true,
		IsSeed:            false,
		ProgressWeight:    1,
		ParentIds:         []string{"T23A"},
	}

	task3 := Task{
		Id:                "T23C",
		TaskGroupId:       "G23",
		Name:              "Reset Task",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T23C",
		RemainingAttempts: 0,
		IsPaused:          true,
		IsComplete:        true,
		IsSeed:            false,
		ProgressWeight:    1,
		ParentIds:         []string{"T23B"},
	}

	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G23", "Test", taskGroupController)
	group.PreloadTasks([]*Task{&task1, &task2, &task3}, &TaskTestClient{})
	group.Operate()

	updateComplete := make(chan error)
	group.Reset(5, updateComplete)
	<-updateComplete

	if len(group.TaskOperators) != 1 {
		t.Fatalf(`len(group.TaskOperators) = %v, want %v`, len(group.TaskOperators), 1)
	}
	for _, op := range group.TaskOperators {
		if op.Task.Id != "T23A" {
			t.Fatalf(`op.Task.Id = %v, want %v`, op.Task.Id, "T23A")
		}
	}
	if task1.IsComplete != false {
		t.Fatalf(`task1.IsComplete = %v, want %v`, task1.IsComplete, false)
	}

	group.Shutdown()
}

func TestCanPauseAllTasks(t *testing.T) {
	task1 := Task{
		Id:                "T24A",
		TaskGroupId:       "G24",
		Name:              "Reset Task",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T24A",
		RemainingAttempts: 0,
		IsPaused:          false,
		IsComplete:        true,
		IsSeed:            true,
		ProgressWeight:    1,
	}

	task2 := Task{
		Id:                "T24B",
		TaskGroupId:       "G24",
		Name:              "Reset Task",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T24B",
		RemainingAttempts: 0,
		IsPaused:          false,
		IsComplete:        true,
		IsSeed:            false,
		ProgressWeight:    1,
		ParentIds:         []string{"T24A"},
	}

	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G24", "Test", taskGroupController)
	group.PreloadTasks([]*Task{&task1, &task2}, &TaskTestClient{})
	group.Operate()

	group.UnPauseAllTasks()

	for _, op := range group.TaskOperators {
		if op.Task.IsPaused != false {
			t.Fatalf(`op.Task.IsPaused = %v, want %v`, op.Task.IsPaused, false)
		}
	}

	group.Shutdown()
}
