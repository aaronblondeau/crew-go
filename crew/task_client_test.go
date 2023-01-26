package crew

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type PostInvocationCountClient struct {
	PostInvocationCount int
}

func (client *PostInvocationCountClient) Post(URL string, task *Task) (output interface{}, children []*Task, err error) {
	output = map[string]interface{}{
		"test": "Hook Complete",
	}
	children = make([]*Task, 0)
	err = nil
	client.PostInvocationCount++
	return
}

type PostReturnsChildrenClient struct {
	Children []*Task
	Output   interface{}
}

func (client *PostReturnsChildrenClient) Post(URL string, task *Task) (output interface{}, children []*Task, err error) {
	output = client.Output
	if task.Name == "Parent" {
		children = client.Children
	} else {
		children = make([]*Task, 0)
	}
	err = nil
	return
}

type PostErrorClient struct {
	ErrorMessage string
	Output       interface{}
}

func (client *PostErrorClient) Post(URL string, task *Task) (output interface{}, children []*Task, err error) {
	output = client.Output
	err = errors.New(client.ErrorMessage)
	children = make([]*Task, 0)
	return
}

type FailOnceThenSucceedClient struct {
	PostInvocationCount int
	ErrorMessage        string
	Output              interface{}
}

func (client *FailOnceThenSucceedClient) Post(URL string, task *Task) (output interface{}, children []*Task, err error) {
	output = client.Output
	children = make([]*Task, 0)
	if client.PostInvocationCount == 0 {
		err = errors.New(client.ErrorMessage)
	} else {
		err = nil
	}
	client.PostInvocationCount++
	return
}

func TestTaskInvokesClientPost(t *testing.T) {
	task := Task{
		Id:                "T2",
		TaskGroupId:       "G1",
		Name:              "Task One",
		WorkerId:          "test",
		Workgroup:         "",
		Key:               "T1",
		RemainingAttempts: 5,
		// Start task as paused
		IsPaused:       false,
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

	client := PostInvocationCountClient{}
	group.Prepare([]*Task{&task}, workers, &client)
	group.Operate()

	// Wait for task to complete
	wg.Wait()

	if task.IsComplete != true {
		t.Fatalf(`Task.IsComplete = %v, want true`, task.IsComplete)
	}
	if client.PostInvocationCount != 1 {
		t.Fatalf(`client.PostInvocationCount = %v, want 1`, client.PostInvocationCount)
	}
	output := task.Output.(map[string]interface{})["test"]
	if output != "Hook Complete" {
		t.Fatalf(`task.Output["test"] = %v, want "Hook Complete"`, output)
	}
}

func TestCaptureError(t *testing.T) {
	task := Task{
		Id:                "T2",
		TaskGroupId:       "G1",
		Name:              "Task One",
		WorkerId:          "test",
		Workgroup:         "",
		Key:               "T1",
		RemainingAttempts: 1,
		// Start task as paused
		IsPaused:       false,
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

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for event := range group.TaskUpdates {
			fmt.Println("Got an update!", event.Event, event.Task.IsComplete)
			if len(event.Task.Errors) > 0 {
				wg.Done()
				return
			}
		}
	}()

	client := PostErrorClient{}
	client.ErrorMessage = "Oops, I died"
	group.Prepare([]*Task{&task}, workers, &client)
	group.Operate()

	// Wait for task to complete
	wg.Wait()

	if task.IsComplete != false {
		t.Fatalf(`Task.IsComplete = %v, want false`, task.IsComplete)
	}
	err := fmt.Sprintf("%v", task.Errors[0])
	if err != client.ErrorMessage {
		t.Fatalf(`task.Errors[0] = %v, want %v`, err, client.ErrorMessage)
	}
	if task.RemainingAttempts != 0 {
		t.Fatalf(`task.RemainingAttempts = %v, want 0`, task.RemainingAttempts)
	}
}

func TestErrorOnceThenSucceed(t *testing.T) {
	task := Task{
		Id:                "T2",
		TaskGroupId:       "G1",
		Name:              "Task One",
		WorkerId:          "test",
		Workgroup:         "",
		Key:               "T1",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:            false,
		IsComplete:          false,
		Priority:            1,
		ProgressWeight:      1,
		ParentIds:           []string{},
		ErrorDelayInSeconds: 1.0,
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

	client := FailOnceThenSucceedClient{}
	client.ErrorMessage = "Oops, I goofed"
	group.Prepare([]*Task{&task}, workers, &client)
	group.Operate()

	// Wait for task to complete
	wg.Wait()

	err := fmt.Sprintf("%v", task.Errors[0])
	if task.IsComplete != true {
		t.Fatalf(`Task.IsComplete = %v, want true`, task.IsComplete)
	}
	if err != client.ErrorMessage {
		t.Fatalf(`task.Errors[0] = %v, want %v`, err, client.ErrorMessage)
	}
	if task.RemainingAttempts != 0 {
		t.Fatalf(`task.RemainingAttempts = %v, want 0`, task.RemainingAttempts)
	}
}

func TestSingleChildOutput(t *testing.T) {
	parent := Task{
		Id:                "P1",
		TaskGroupId:       "G1",
		Name:              "Parent",
		WorkerId:          "test",
		Workgroup:         "",
		Key:               "T1",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{},
	}

	child := Task{
		Id:                "C1",
		TaskGroupId:       "G1",
		Name:              "Child",
		WorkerId:          "test",
		Workgroup:         "",
		Key:               "C1",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
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

	var wgParent sync.WaitGroup
	wgParent.Add(1)
	var wgChild sync.WaitGroup
	wgChild.Add(1)
	go func() {
		for event := range group.TaskUpdates {
			fmt.Println("Got an update!", event.Event, event.Task.Id, event.Task.IsComplete)
			if event.Task.Id == parent.Id {
				if event.Task.IsComplete {
					wgParent.Done()
				}
			}
			if event.Task.Id == child.Id {
				if event.Task.IsComplete {
					wgChild.Done()
					return
				}
			}
		}
	}()

	client := PostReturnsChildrenClient{}
	client.Output = map[string]interface{}{
		"children": "How they grow...",
	}
	client.Children = []*Task{&child}

	group.Prepare([]*Task{&parent}, workers, &client)
	group.Operate()

	// Wait for task to complete
	wgParent.Wait()
	if parent.IsComplete != true {
		t.Fatalf(`parent.IsComplete = %v, want true`, parent.IsComplete)
	}
	if len(parent.Children) != 1 {
		t.Fatalf(`len(parent.Children) = %v, want 1`, len(parent.Children))
	}

	// Wait for child to complete
	wgChild.Wait()
	if child.IsComplete != true {
		t.Fatalf(`child.IsComplete = %v, want true`, child.IsComplete)
	}
}
