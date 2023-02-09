package crew

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

type TaskGroupTestClient struct{}

func (client TaskGroupTestClient) Post(URL string, task *Task, taskGroup *TaskGroup) (response WorkerResponse, err error) {
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
		Id:                "T1",
		TaskGroupId:       "G1",
		Name:              "Incomplete Task Parent",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T1",
		RemainingAttempts: 5,
		IsPaused:          false,
		IsComplete:        false,
		Priority:          1,
		ProgressWeight:    1,
	}

	task := Task{
		Id:                "T2",
		TaskGroupId:       "G1",
		Name:              "Task One",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T1",
		RemainingAttempts: 5,
		IsPaused:          false,
		IsComplete:        false,
		Priority:          1,
		ProgressWeight:    1,
		ParentIds:         []string{"T1"},
	}

	group := TaskGroup{
		Id:            "G1",
		Name:          "Test",
		IsPaused:      false,
		CreatedAt:     time.Now(),
		TaskOperators: make(map[string]*TaskOperator),
		TaskUpdates:   make(chan TaskUpdateEvent, 8),
	}

	urlGen := func(task *Task) (url string, err error) {
		return "https://example.com/test", nil
	}

	group.Prepare([]*Task{&parent, &task}, urlGen, &TaskGroupTestClient{})

	if parent.Children[0] != &task {
		t.Fatal("Parent task's Children slice was not inflated!")
	}
}

func TestCanDeleteTask(t *testing.T) {
	task := Task{
		Id:                "T2",
		TaskGroupId:       "G1",
		Name:              "Task One",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T1",
		RemainingAttempts: 5,
		IsPaused:          true,
		IsComplete:        false,
		Priority:          1,
		ProgressWeight:    1,
		ParentIds:         []string{},
	}

	group := TaskGroup{
		Id:            "G1",
		Name:          "Test",
		IsPaused:      true,
		CreatedAt:     time.Now(),
		TaskOperators: make(map[string]*TaskOperator),
		TaskUpdates:   make(chan TaskUpdateEvent, 8),
	}

	urlGen := func(task *Task) (url string, err error) {
		return "https://example.com/test", nil
	}

	group.Prepare([]*Task{&task}, urlGen, &TaskTestClient{})
	group.Operate()
	// Give operate goroutine a second to get going

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
		Id:                "T2",
		TaskGroupId:       "G1",
		Name:              "Task One",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T1",
		RemainingAttempts: 5,
		// Start task as paused
		IsPaused:       true,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{},
	}

	group := TaskGroup{
		Id:            "G1",
		Name:          "Test",
		IsPaused:      false,
		CreatedAt:     time.Now(),
		TaskOperators: make(map[string]*TaskOperator),
		TaskUpdates:   make(chan TaskUpdateEvent, 8),
	}

	if len(group.TaskOperators) != 0 {
		t.Errorf("len(group.TaskOperators) = %d; want 0", len(group.TaskOperators))
	}

	urlGen := func(task *Task) (url string, err error) {
		return "https://example.com/test", nil
	}

	group.Prepare([]*Task{}, urlGen, &TaskGroupTestClient{})

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
