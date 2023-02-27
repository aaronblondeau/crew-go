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

func (client *PostInvocationCountClient) Post(task *Task, taskGroup *TaskGroup) (response WorkerResponse, err error) {
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
	Children                []*Task
	Output                  interface{}
	WorkgroupDelayInSeconds int
	ChildrenDelayInSeconds  int
}

func (client *PostReturnsChildrenClient) Post(task *Task, taskGroup *TaskGroup) (response WorkerResponse, err error) {
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
		WorkgroupDelayInSeconds: client.WorkgroupDelayInSeconds,
		ChildrenDelayInSeconds:  client.ChildrenDelayInSeconds,
	}
	return
}

type PostErrorClient struct {
	ErrorMessage string
	Output       interface{}
}

func (client *PostErrorClient) Post(task *Task, taskGroup *TaskGroup) (response WorkerResponse, err error) {
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

func (client *FailOnceThenSucceedClient) Post(task *Task, taskGroup *TaskGroup) (response WorkerResponse, err error) {
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
		Id:                "T1",
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
	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G1", "Test", taskGroupController)

	if len(group.TaskOperators) != 0 {
		t.Errorf("len(group.TaskOperators) = %d; want 0", len(group.TaskOperators))
	}

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

	client := PostInvocationCountClient{}
	group.PreloadTasks([]*Task{&task}, &client)
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
		TaskGroupId:       "G2",
		Name:              "Task Two",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T2",
		RemainingAttempts: 1,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{},
	}
	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G2", "Test", taskGroupController)

	if len(group.TaskOperators) != 0 {
		t.Errorf("len(group.TaskOperators) = %d; want 0", len(group.TaskOperators))
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for event := range group.Controller.TaskUpdates {
			fmt.Println("Got an update!", event.Event, event.Task.IsComplete)
			if len(event.Task.Errors) > 0 {
				wg.Done()
				return
			}
		}
	}()

	client := PostErrorClient{}
	client.ErrorMessage = "Oops, I died"
	group.PreloadTasks([]*Task{&task}, &client)
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
		Id:                "T3",
		TaskGroupId:       "G3",
		Name:              "Task Three",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T3",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:            false,
		IsComplete:          false,
		Priority:            1,
		ProgressWeight:      1,
		ParentIds:           []string{},
		ErrorDelayInSeconds: 1.0,
	}
	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G3", "Test", taskGroupController)

	if len(group.TaskOperators) != 0 {
		t.Errorf("len(group.TaskOperators) = %d; want 0", len(group.TaskOperators))
	}

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

	client := FailOnceThenSucceedClient{}
	client.ErrorMessage = "Oops, I goofed"
	group.PreloadTasks([]*Task{&task}, &client)
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
		Id:                "T4P",
		TaskGroupId:       "G4",
		Name:              "Parent",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T4P",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{},
	}

	child := Task{
		Id:                "T4C",
		TaskGroupId:       "G4",
		Name:              "Child",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T4C",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{},
	}
	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G4", "Test", taskGroupController)

	if len(group.TaskOperators) != 0 {
		t.Errorf("len(group.TaskOperators) = %d; want 0", len(group.TaskOperators))
	}

	var wgParent sync.WaitGroup
	wgParent.Add(1)
	var wgChild sync.WaitGroup
	wgChild.Add(1)
	go func() {
		for event := range group.Controller.TaskUpdates {
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
	group.PreloadTasks([]*Task{&parent}, &client)
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
		Id:                "T5P",
		TaskGroupId:       "G5",
		Name:              "Parent",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T5P",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{},
	}

	child1 := Task{
		Id:                "T5C1",
		TaskGroupId:       "G5",
		Name:              "Child",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T5C1",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{},
	}

	child2A := Task{
		Id:                "T5C2A",
		TaskGroupId:       "G5",
		Name:              "Child",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T5C2A",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{"T5C1"},
	}

	child2B := Task{
		Id:                "T5C2B",
		TaskGroupId:       "G5",
		Name:              "Child",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T5C2B",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{"T5C1"},
	}

	child3 := Task{
		Id:                "T5C3",
		TaskGroupId:       "G5",
		Name:              "Child",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T5C3",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{"T5C2A", "T5C2B"},
	}
	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G5", "Test", taskGroupController)

	if len(group.TaskOperators) != 0 {
		t.Errorf("len(group.TaskOperators) = %d; want 0", len(group.TaskOperators))
	}

	var wgParent sync.WaitGroup
	wgParent.Add(1)
	var wgChildren sync.WaitGroup
	wgChildren.Add(4)
	childCompletionOrder := []string{}
	go func() {
		for event := range group.Controller.TaskUpdates {
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
	group.PreloadTasks([]*Task{&parent}, &client)
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
	if childCompletionOrder[0] != "T5C1" {
		t.Fatalf(`childCompletionOrder[0] = %v, want T5C1`, childCompletionOrder[0])
	}
	if !(childCompletionOrder[1] == "T5C2A" || childCompletionOrder[1] == "T5C2B") {
		t.Fatalf(`childCompletionOrder[1] = %v, want T5C2A or T5C2B`, childCompletionOrder[1])
	}
	if !(childCompletionOrder[2] == "T5C2A" || childCompletionOrder[2] == "T5C2B") {
		t.Fatalf(`childCompletionOrder[2] = %v, want T5C2A or T5C2B`, childCompletionOrder[2])
	}
	if childCompletionOrder[3] != "T5C3" {
		t.Fatalf(`childCompletionOrder[3] = %v, want T5C3`, childCompletionOrder[3])
	}
}

func TestBadChildrenOutput(t *testing.T) {
	parent := Task{
		Id:                "T6P1",
		TaskGroupId:       "G6",
		Name:              "Parent",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T6P1",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{},
	}

	child1 := Task{
		Id:                "T6C1",
		TaskGroupId:       "G6",
		Name:              "Child",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T6C1",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{},
	}

	child2 := Task{
		Id:                "T6C2",
		TaskGroupId:       "G6",
		Name:              "Child",
		Worker:            "test",
		Workgroup:         "",
		Key:               "T6C2",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{"CX"},
	}
	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G6", "Test", taskGroupController)

	if len(group.TaskOperators) != 0 {
		t.Errorf("len(group.TaskOperators) = %d; want 0", len(group.TaskOperators))
	}

	var wgParent sync.WaitGroup
	wgParent.Add(1)
	go func() {
		defer wgParent.Done()
		for event := range group.Controller.TaskUpdates {
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
	group.PreloadTasks([]*Task{&parent}, &client)
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

func TestWorkgroupDelayInSecondsOutput(t *testing.T) {
	parent := Task{
		Id:                "T14P",
		TaskGroupId:       "G14",
		Name:              "Parent",
		Worker:            "test",
		Workgroup:         "T14Delay",
		Key:               "T14P",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{},
	}

	child := Task{
		Id:                "T14C",
		TaskGroupId:       "G14",
		Name:              "Child",
		Worker:            "test",
		Workgroup:         "T14Delay",
		Key:               "T14C",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{},
		RunAfter:       time.Time{},
	}
	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G14", "Test", taskGroupController)
	taskGroupController.AddGroup(group)

	if len(group.TaskOperators) != 0 {
		t.Errorf("len(group.TaskOperators) = %d; want 0", len(group.TaskOperators))
	}

	var wgParent sync.WaitGroup
	wgParent.Add(1)
	var wgChild sync.WaitGroup
	wgChild.Add(1)

	parentCompleteTime := time.Time{}
	goFuncFail := ""

	go func() {
		for event := range group.Controller.TaskUpdates {
			if event.Task.Id == parent.Id {
				if event.Task.IsComplete {
					parentCompleteTime = time.Now()
					wgParent.Done()
				}
			}
			if event.Task.Id == child.Id {
				if !event.Task.RunAfter.IsZero() {
					// Make sure child has RunAfter in future
					threshold := parentCompleteTime.Add(3 * time.Second)
					if !child.RunAfter.After(threshold) {
						goFuncFail = fmt.Sprintf(`child.RunAfter = %v, should be after %v`, child.RunAfter, threshold)
					}
				}
				if event.Task.IsComplete {
					if parent.IsComplete != true {
						goFuncFail = fmt.Sprintf(`parent.IsComplete = %v, want true`, parent.IsComplete)
					}
					if len(parent.Children) != 1 {
						goFuncFail = fmt.Sprintf(`len(parent.Children) = %v, want 1`, len(parent.Children))
					}
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
	client.WorkgroupDelayInSeconds = 5
	client.Children = []*Task{&child}
	group.PreloadTasks([]*Task{&parent}, &client)
	group.Operate()

	// Wait for task to complete
	wgParent.Wait()

	// Wait for child to complete
	wgChild.Wait()
	if child.IsComplete != true {
		t.Fatalf(`child.IsComplete = %v, want true`, child.IsComplete)
	}

	// It should have taken more than 5 seconds for child to complete
	now2 := time.Now()
	nowDiff := now2.Sub(parentCompleteTime).Seconds()
	if nowDiff < 5 {
		t.Fatalf(`nowDiff = %v, want 5`, nowDiff)
	}

	if goFuncFail != "" {
		t.Fatalf(goFuncFail)
	}
}

func TestChildrenDelayInSecondsSecondsOutput(t *testing.T) {
	parent := Task{
		Id:                "T15P",
		TaskGroupId:       "G15",
		Name:              "Parent",
		Worker:            "test",
		Workgroup:         "T15Delay",
		Key:               "T15P",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{},
	}

	child := Task{
		Id:                "T15C",
		TaskGroupId:       "G15",
		Name:              "Child",
		Worker:            "test",
		Workgroup:         "T15Delay",
		Key:               "T15C",
		RemainingAttempts: 2,
		// Start task as paused
		IsPaused:       false,
		IsComplete:     false,
		Priority:       1,
		ProgressWeight: 1,
		ParentIds:      []string{},
	}
	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	group := NewTaskGroup("G15", "Test", taskGroupController)
	taskGroupController.AddGroup(group)

	if len(group.TaskOperators) != 0 {
		t.Errorf("len(group.TaskOperators) = %d; want 0", len(group.TaskOperators))
	}

	var wgParent sync.WaitGroup
	wgParent.Add(1)
	var wgChildUpdate sync.WaitGroup
	wgChildUpdate.Add(1)
	var wgChild sync.WaitGroup
	wgChild.Add(1)
	go func() {
		childUpdated := false
		childDone := false
		parentDone := false
		for event := range group.Controller.TaskUpdates {
			fmt.Println("Got an update!", event.Event, event.Task.Id, event.Task.IsComplete)
			if event.Task.Id == parent.Id {
				if !parentDone && event.Task.IsComplete {
					wgParent.Done()
					parentDone = true
				}
			}
			if event.Task.Id == child.Id {
				if !childDone && event.Task.IsComplete {
					wgChild.Done()
					childDone = true
				}
				if !childUpdated && !event.Task.RunAfter.IsZero() {
					wgChildUpdate.Done()
					childUpdated = true
				}
			}
			if childUpdated && childDone && parentDone {
				return
			}
		}
	}()

	client := PostReturnsChildrenClient{}
	client.Output = map[string]interface{}{
		"children": "How they grow...",
	}
	client.ChildrenDelayInSeconds = 5
	client.Children = []*Task{&child}
	group.PreloadTasks([]*Task{&parent}, &client)
	group.Operate()

	now := time.Now()

	var wgSteps sync.WaitGroup
	wgSteps.Add(3)

	// Wait for task to complete
	go func() {
		wgParent.Wait()
		if parent.IsComplete != true {
			t.Fatalf(`parent.IsComplete = %v, want true`, parent.IsComplete)
		}
		if len(parent.Children) != 1 {
			t.Fatalf(`len(parent.Children) = %v, want 1`, len(parent.Children))
		}
		wgSteps.Done()
	}()

	// Wait for child to get updated (runAfter update)
	go func() {
		wgChildUpdate.Wait()
		// Make sure child has RunAfter in future
		threshold := now.Add(3 * time.Second)
		if !child.RunAfter.After(threshold) {
			t.Fatalf(`child.RunAfter = %v, should be after %v`, child.RunAfter, threshold)
		}
		wgSteps.Done()
	}()

	// Wait for child to complete
	go func() {
		wgChild.Wait()
		if child.IsComplete != true {
			t.Fatalf(`child.IsComplete = %v, want true`, child.IsComplete)
		}
		wgSteps.Done()
	}()

	wgSteps.Wait()

	// It should have taken more than 5 seconds for child to complete
	now2 := time.Now()
	nowDiff := now2.Sub(now).Seconds()
	if nowDiff < 5 {
		t.Fatalf(`nowDiff = %v, want 5`, nowDiff)
	}
}
