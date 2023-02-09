package crew

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

type PostInvocationCountClient struct {
	PostInvocationCount int
}

func (client *PostInvocationCountClient) Post(URL string, task *Task, taskGroup *TaskGroup) (response WorkerResponse, err error) {
	client.PostInvocationCount++
	response = WorkerResponse{
		Output: map[string]interface{}{
			"test": "Hook Complete",
		},
		Children:                make([]*Task, 0),
		Error:                   nil,
		WorkgroupDelayInSeconds: 0,
		ChildrenDelayInSeconds:  0,
	}
	return
}

type PostReturnsChildrenClient struct {
	Children []*Task
	Output   interface{}
}

func (client *PostReturnsChildrenClient) Post(URL string, task *Task, taskGroup *TaskGroup) (response WorkerResponse, err error) {
	var children []*Task
	if task.Name == "Parent" {
		children = client.Children
	} else {
		children = make([]*Task, 0)
	}
	response = WorkerResponse{
		Output:                  client.Output,
		Children:                children,
		Error:                   nil,
		WorkgroupDelayInSeconds: 0,
		ChildrenDelayInSeconds:  0,
	}
	return
}

type PostErrorClient struct {
	ErrorMessage string
	Output       interface{}
}

func (client *PostErrorClient) Post(URL string, task *Task, taskGroup *TaskGroup) (response WorkerResponse, err error) {
	response = WorkerResponse{
		Output:                  client.Output,
		Children:                make([]*Task, 0),
		Error:                   client.ErrorMessage,
		WorkgroupDelayInSeconds: 0,
		ChildrenDelayInSeconds:  0,
	}
	return
}

type FailOnceThenSucceedClient struct {
	PostInvocationCount int
	ErrorMessage        string
	Output              interface{}
}

func (client *FailOnceThenSucceedClient) Post(URL string, task *Task, taskGroup *TaskGroup) (response WorkerResponse, err error) {
	var workerError interface{}
	if client.PostInvocationCount == 0 {
		workerError = client.ErrorMessage
	} else {
		workerError = nil
	}

	client.PostInvocationCount++

	response = WorkerResponse{
		Output:                  client.Output,
		Children:                make([]*Task, 0),
		Error:                   workerError,
		WorkgroupDelayInSeconds: 0,
		ChildrenDelayInSeconds:  0,
	}

	return
}

func TestTaskInvokesClientPost(t *testing.T) {
	task := Task{
		Id:                "T2",
		TaskGroupId:       "G1",
		Name:              "Task One",
		Worker:            "test",
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

	urlGen := func(task *Task) (url string, err error) {
		return "https://example.com/test", nil
	}

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
	group.Prepare([]*Task{&task}, urlGen, &client)
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
		Worker:            "test",
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

	urlGen := func(task *Task) (url string, err error) {
		return "https://example.com/test", nil
	}

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
	group.Prepare([]*Task{&task}, urlGen, &client)
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
		Worker:            "test",
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

	urlGen := func(task *Task) (url string, err error) {
		return "https://example.com/test", nil
	}

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
	group.Prepare([]*Task{&task}, urlGen, &client)
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
		Worker:            "test",
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
		Worker:            "test",
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

	urlGen := func(task *Task) (url string, err error) {
		return "https://example.com/test", nil
	}

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

	group.Prepare([]*Task{&parent}, urlGen, &client)
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

func TestMultipleChildOutput(t *testing.T) {
	parent := Task{
		Id:                "P1",
		TaskGroupId:       "G1",
		Name:              "Parent",
		Worker:            "test",
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

	child1 := Task{
		Id:                "C1",
		TaskGroupId:       "G1",
		Name:              "Child",
		Worker:            "test",
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

	child2A := Task{
		Id:                "C2A",
		TaskGroupId:       "G1",
		Name:              "Child",
		Worker:            "test",
		Workgroup:         "",
		Key:               "C2A",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{"C1"},
	}

	child2B := Task{
		Id:                "C2B",
		TaskGroupId:       "G1",
		Name:              "Child",
		Worker:            "test",
		Workgroup:         "",
		Key:               "C2B",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{"C1"},
	}

	child3 := Task{
		Id:                "C3",
		TaskGroupId:       "G1",
		Name:              "Child",
		Worker:            "test",
		Workgroup:         "",
		Key:               "C3",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{"C2A", "C2B"},
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

	var wgParent sync.WaitGroup
	wgParent.Add(1)
	var wgChildren sync.WaitGroup
	wgChildren.Add(4)
	childCompletionOrder := []string{}
	go func() {
		for event := range group.TaskUpdates {
			fmt.Println("Got an update!", event.Event, event.Task.Id, event.Task.IsComplete)
			if event.Task.Id == parent.Id {
				if event.Task.IsComplete {
					wgParent.Done()
				}
			}
			if event.Task.Id == child1.Id || event.Task.Id == child2A.Id || event.Task.Id == child2B.Id || event.Task.Id == child3.Id {
				if event.Task.IsComplete {
					wgChildren.Done()
					childCompletionOrder = append(childCompletionOrder, event.Task.Id)
				}
				fmt.Println("~~ len(childCompletionOrder)", len(childCompletionOrder))
				if len(childCompletionOrder) > 3 {
					return
				}
			}
		}
	}()

	client := PostReturnsChildrenClient{}
	client.Output = map[string]interface{}{
		"children": "How they grow...",
	}
	client.Children = []*Task{&child1, &child2A, &child2B, &child3}

	group.Prepare([]*Task{&parent}, urlGen, &client)
	group.Operate()

	// Wait for task to complete
	wgParent.Wait()
	if parent.IsComplete != true {
		t.Fatalf(`parent.IsComplete = %v, want true`, parent.IsComplete)
	}
	if len(parent.Children) != 4 {
		t.Fatalf(`len(parent.Children) = %v, want 4`, len(parent.Children))
	}

	// Wait for child to complete
	wgChildren.Wait()
	if child1.IsComplete != true {
		t.Fatalf(`child1.IsComplete = %v, want true`, child1.IsComplete)
	}
	if child2A.IsComplete != true {
		t.Fatalf(`child2A.IsComplete = %v, want true`, child2A.IsComplete)
	}
	if child2B.IsComplete != true {
		t.Fatalf(`child2B.IsComplete = %v, want true`, child2B.IsComplete)
	}
	if child3.IsComplete != true {
		t.Fatalf(`child3.IsComplete = %v, want true`, child3.IsComplete)
	}

	// Make sure children completed in proper order
	if childCompletionOrder[0] != "C1" {
		t.Fatalf(`childCompletionOrder[0] = %v, want C1`, childCompletionOrder[0])
	}
	if !(childCompletionOrder[1] == "C2A" || childCompletionOrder[1] == "C2B") {
		t.Fatalf(`childCompletionOrder[1] = %v, want C2A or C2B`, childCompletionOrder[1])
	}
	if !(childCompletionOrder[2] == "C2A" || childCompletionOrder[2] == "C2B") {
		t.Fatalf(`childCompletionOrder[2] = %v, want C2A or C2B`, childCompletionOrder[2])
	}
	if childCompletionOrder[3] != "C3" {
		t.Fatalf(`childCompletionOrder[3] = %v, want C3`, childCompletionOrder[3])
	}
}

func TestBadChildrenOutput(t *testing.T) {
	parent := Task{
		Id:                "P1",
		TaskGroupId:       "G1",
		Name:              "Parent",
		Worker:            "test",
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

	child1 := Task{
		Id:                "C1",
		TaskGroupId:       "G1",
		Name:              "Child",
		Worker:            "test",
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

	child2 := Task{
		Id:                "C1",
		TaskGroupId:       "G1",
		Name:              "Child",
		Worker:            "test",
		Workgroup:         "",
		Key:               "C1",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{"CX"},
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

	var wgParent sync.WaitGroup
	wgParent.Add(1)
	go func() {
		defer wgParent.Done()
		for event := range group.TaskUpdates {
			fmt.Println("Got an update!", event.Event, event.Task.Id, event.Task.IsComplete)
			if event.Task.Id == parent.Id {
				if event.Task.IsComplete || len(event.Task.Errors) > 0 {
					return
				}
			}
		}
	}()

	client := PostReturnsChildrenClient{}
	client.Output = map[string]interface{}{
		"children": "How they grow...",
	}
	client.Children = []*Task{&child1, &child2}

	group.Prepare([]*Task{&parent}, urlGen, &client)
	group.Operate()

	// Wait for task to complete
	wgParent.Wait()
	if parent.IsComplete != false {
		t.Fatalf(`parent.IsComplete = %v, want false`, parent.IsComplete)
	}
	if len(parent.Children) != 0 {
		t.Fatalf(`len(parent.Children) = %v, want 0`, len(parent.Children))
	}

	// Workgroup should still only have one task
	if len(group.TaskOperators) != 1 {
		t.Fatalf(`len(group.TaskOperators) = %v, want 1`, len(group.TaskOperators))
	}

	// parent should have an error
	if len(parent.Errors) != 1 {
		t.Fatalf(`len(parent.Errors) = %v, want 1`, len(parent.Errors))
	}

	group.Shutdown()
}
