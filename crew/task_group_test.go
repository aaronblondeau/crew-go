package crew

import (
	"fmt"
	"sync"
	"testing"
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
		Priority:          1,
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
		Priority:          1,
		ProgressWeight:    1,
		ParentIds:         []string{"T7A"},
	}
	group := NewTaskGroup("G7", "Test")
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
		Priority:          1,
		ProgressWeight:    1,
		ParentIds:         []string{},
	}

	group := NewTaskGroup("G8", "Test")
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
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{},
	}

	group := NewTaskGroup("G9", "Test")

	if len(group.TaskOperators) != 0 {
		t.Errorf("len(group.TaskOperators) = %d; want 0", len(group.TaskOperators))
	}

	group.PreloadTasks([]*Task{}, &TaskGroupTestClient{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for event := range group.TaskUpdates {
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
	group.TaskOperators[task.Id].ExternalUpdates <- map[string]interface{}{
		"isPaused": false,
	}

	// Wait for task to complete
	wg.Wait()

	if task.IsComplete != true {
		t.Fatalf(`Task.IsComplete = %v, want true`, task.IsComplete)
	}
}
