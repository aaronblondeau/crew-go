package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/crew-go/crew"
)

func main() {
	os.Setenv("CREW_WORKER_BASE_URL", "https://us-central1-dose-board-aaron-dev.cloudfunctions.net/")

	// Pull each task group out of storage
	taskGroups := make(map[string]*crew.TaskGroup)
	group := crew.NewTaskGroup("G1", "test")
	// Keeping an index of them by id
	taskGroups[group.Id] = group

	// Pull each task out of storage
	taskGroupTasks := make(map[string][]*crew.Task)
	task := crew.Task{
		Id:                  "T1",
		TaskGroupId:         "G1",
		Name:                "Task One",
		Worker:              "worker-a",
		Workgroup:           "",
		Key:                 "T1",
		RemainingAttempts:   5,
		IsPaused:            false,
		IsComplete:          false,
		Priority:            1,
		RunAfter:            time.Now().Add(5 * time.Second),
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
	httpPostClient := crew.NewHttpPostClient()
	for _, taskGroup := range taskGroups {
		taskGroup.PreloadTasks(taskGroupTasks[taskGroup.Id], httpPostClient)
	}

	// Some debug code...
	fmt.Println(taskGroups[group.Id].Name)

	// To update a task:
	// taskGroups[group.Id].TaskOperators[task.Id].Updates <- map[string]interface{}{
	// 	"name": "New Name",
	// }

	// To delete a task:
	// taskGroups[group.id].DeleteTask(taskIdToDelete)

	// TODO, When Go service is terminating, do this for every task
	// operatorPtr.Shutdown <- true

	// TODO, When worker is added or renamed, call every taskGroup's WorkerAvailable()

	var wg sync.WaitGroup

	fmt.Println("About to wait for stuff to happen")
	wg.Add(1)
	go func() {
		timeout := time.NewTimer(20 * time.Second)
		for {
			fmt.Println("Waiting for stuff to happen")
			// Wait for a complete/error event (or timeout the test)
			select {
			case wgDelayEvent := <-group.WorkgroupDelays:
				fmt.Println("Got a workgroup delay!", wgDelayEvent.Workgroup)
				// Pass workgroup delay on to other task groups
				for _, group := range taskGroups {
					// Group where event originated will have already processed the delay so skip it
					if group.Id != wgDelayEvent.OriginTaskGroupId {
						group.DelayTasksInWorkgroup(wgDelayEvent.Workgroup, wgDelayEvent.DelayInSeconds)
					}
				}

			case event := <-group.TaskUpdates:
				fmt.Println("Got an update!", event.Event, event.Task.IsComplete)
				if event.Task.IsComplete {
					wg.Done()
					return
				}
				// TODO - emit SSE event for each update
			case <-timeout.C:
				fmt.Println("Timed out!")
				for _, op := range group.TaskOperators {
					op.Shutdown <- true
				}
				wg.Done()
				return
			}
			fmt.Println("Something happened...")
		}
	}()

	// Call operate on every operator!
	for _, taskGroup := range taskGroups {
		taskGroup.Operate()
	}

	wg.Wait()

	// TODO : Bootstrap Rest API
	// TODO : Bootstrap Server Sent Events
}
