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

	cwd, _ := os.Getwd()
	storage := crew.NewJsonFilesystemTaskStorage(cwd + "/main_demo")
	client := crew.NewHttpPostClient()

	taskGroups, bootstrapError := storage.Bootstrap(false, client)
	if bootstrapError != nil {
		panic(bootstrapError)
	}

	group := taskGroups["demo"]

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

	wg.Add(1)
	go func() {
		timeout := time.NewTimer(20 * time.Second)
		for {
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
