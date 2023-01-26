package crew

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

type TaskGroupTestClient struct{}

func (client TaskGroupTestClient) Post(URL string, task *Task) (output interface{}, children []*Task, err error) {
	output = map[string]interface{}{
		"demo": "Test Complete",
	}
	children = make([]*Task, 0)
	err = nil
	return
}

func TestPrepareInflatesChildren(t *testing.T) {
	parent := Task{
		Id:                "T1",
		TaskGroupId:       "G1",
		Name:              "Incomplete Task Parent",
		WorkerId:          "test",
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
		WorkerId:          "test",
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

	testWorker := Worker{
		Id:  "test",
		Url: "https://example.com/test",
	}
	workers := make(map[string]Worker)
	workers[testWorker.Id] = testWorker

	group.Prepare([]*Task{&parent, &task}, workers, &TaskGroupTestClient{})

	if parent.Children[0] != &task {
		t.Fatal("Parent task's Children slice was not inflated!")
	}
}

func TestCanDeleteTask(t *testing.T) {
	task := Task{
		Id:                "T2",
		TaskGroupId:       "G1",
		Name:              "Task One",
		WorkerId:          "test",
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

	group.Prepare([]*Task{&task}, make(map[string]Worker), &TaskTestClient{})
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
		WorkerId:          "test",
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

	testWorker := Worker{
		Id:  "test",
		Url: "https://example.com/test",
	}
	workers := make(map[string]Worker)
	workers[testWorker.Id] = testWorker

	group.Prepare([]*Task{}, workers, &TaskGroupTestClient{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for {
			// Wait for a complete/error event (or timeout the test)
			select {
			case event := <-group.TaskUpdates:
				fmt.Println("Got an update!", event.Event, event.Task.IsComplete)
				if event.Task.IsComplete {
					wg.Done()
					return
				}
			default:
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
