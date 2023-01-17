package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/crew-go/crew"
)

func main() {
	devChannel := crew.Channel{
		Id:  "worker-a",
		Url: "https://us-central1-dose-board-aaron-dev.cloudfunctions.net/worker-a",
	}
	channels := make(map[string]crew.Channel)
	channels[devChannel.Id] = devChannel

	// Pull each task group out of storage
	taskGroups := make(map[string]*crew.TaskGroup)
	group := crew.TaskGroup{
		Id:            "G1",
		Name:          "Test",
		IsPaused:      false,
		CreatedAt:     time.Now(),
		TaskOperators: make(map[string]*crew.TaskOperator),
		TaskUpdates:   make(chan crew.TaskUpdateEvent, 8),
	}
	// Keeping an index of them by id
	taskGroups[group.Id] = &group

	// Pull each task out of storage
	taskGroupTasks := make(map[string][]*crew.Task)
	task := crew.Task{
		Id:                  "T1",
		TaskGroupId:         "G1",
		Name:                "Task One",
		Channel:             "worker-a",
		Workgroup:           "",
		Key:                 "T1",
		RemainingAttempts:   5,
		IsPaused:            false,
		IsComplete:          false,
		Priority:            1,
		RunAfter:            time.Now().Add(10 * time.Second),
		ProgressWeight:      1,
		IsSeed:              false,
		ErrorDelayInSeconds: 5,
		Input:               "Test",
		Errors:              make([]interface{}, 0),
		CreatedAt:           time.Now(),
		ParentIds:           make([]string, 0),
		Children:            make([]*crew.Task, 0),
	}
	// Add each task to an index of tasks (by taskGroup)
	// So that we can send them to TaskGroup.Prepare after they are
	// all loaded
	_, initialized := taskGroupTasks[task.TaskGroupId]
	if !initialized {
		taskGroupTasks[task.TaskGroupId] = make([]*crew.Task, 0)
	}
	taskGroupTasks[task.TaskGroupId] = append(taskGroupTasks[task.TaskGroupId], &task)

	// Prepare each task group (creates operator for each task)
	for _, taskGroup := range taskGroups {
		taskGroup.Prepare(taskGroupTasks[taskGroup.Id], channels)
	}

	// Some debug code...
	fmt.Println(taskGroups[group.Id].Name)

	// To update a task:
	// taskGroups[group.Id].TaskOperators[task.Id].Updates <- map[string]interface{}{
	// 	"name": "New Name",
	// }

	// To delete a task:
	// TODO ???
	// TODO - make sure children get recursively deleted (but only if they don't have any other parents!)

	// To emit SSE events from task to subscribers
	go func() {
		event := <-group.TaskUpdates
		fmt.Println("Would emit an SSE event on group (" + group.Id + ") " + event.Event)
	}()

	// TODO, When service is terminating, do this for every task
	// operatorPtr.Shutdown <- true

	var wg sync.WaitGroup

	fmt.Println("About to wait for stuff to happen")
	wg.Add(1)
	go func() {
		fmt.Println("Waiting for stuff to happen")
		timeout := time.NewTimer(20 * time.Second)
		// Wait for a complete/error event (or timeout the test)
		select {
		case output := <-taskGroups[group.Id].TaskOperators[task.Id].Complete:
			fmt.Println("Completed", output)
		case error := <-taskGroups[group.Id].TaskOperators[task.Id].Error:
			fmt.Println("Error", error)
		case <-timeout.C:
			fmt.Println("Timed out!")
		}
		wg.Done()
	}()

	// Call operate on every operator!
	for _, taskGroup := range taskGroups {
		taskGroup.Operate()
	}

	wg.Wait()

	// TODO : Bootstrap Rest API
	// TODO : Bootstrap Server Sent Events
}
