package crew

import (
	"fmt"
	"os"
	"testing"
	"time"
)

type StorageTestClient struct{}

func (client StorageTestClient) Post(task *Task, taskGroup *TaskGroup) (response WorkerResponse, err error) {
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

func TestStoreTaskGroup(t *testing.T) {
	cwd, _ := os.Getwd()
	storage := NewJsonFilesystemTaskStorage(cwd + "/test_storage")

	taskGroupController := NewTaskGroupController()
	group := NewTaskGroup("GS1", "Test storage", taskGroupController)
	group.Storage = storage

	saveTaskGroupError := storage.SaveTaskGroup(group)
	if saveTaskGroupError != nil {
		t.Fatal(`Got an unexpected error when saving task group`, saveTaskGroupError)
	}
}

func TestStoreTaskGroupAndTask(t *testing.T) {
	cwd, _ := os.Getwd()
	storage := NewJsonFilesystemTaskStorage(cwd + "/test_storage")

	taskGroupController := NewTaskGroupController()
	group := NewTaskGroup("GS2", "Test storage", taskGroupController)
	group.Storage = storage

	task := Task{
		Id:                  "TS2",
		TaskGroupId:         "GS2",
		Name:                "Task One",
		Worker:              "worker-a",
		Workgroup:           "",
		Key:                 "TS2",
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
		Children:            make([]*Task, 0),
	}

	saveTaskGroupError := storage.SaveTaskGroup(group)
	if saveTaskGroupError != nil {
		t.Fatal(`Got an unexpected error when saving task group`, saveTaskGroupError)
	}

	saveTaskError := storage.SaveTask(group, &task)
	if saveTaskError != nil {
		t.Fatal(`Got an unexpected error when saving task`, saveTaskError)
	}
}

func TestBootstrap(t *testing.T) {
	cwd, _ := os.Getwd()
	storage := NewJsonFilesystemTaskStorage(cwd + "/test_storage/bootstrap_a")
	client := StorageTestClient{}

	taskGroupController, bootstrapError := storage.Bootstrap(false, client)
	if bootstrapError != nil {
		t.Fatal(`Got an unexpected error when loading bootstrap_a`, bootstrapError)
	}
	if len(taskGroupController.TaskGroups) != 1 {
		t.Fatalf(`len(taskGroupController.TaskGroups) = %v, want 1`, len(taskGroupController.TaskGroups))
	}

	// check group fields
	if taskGroupController.TaskGroups["BSG1"].Name != "Storage bootstrap test 1" {
		t.Fatalf(`taskGroupController.TaskGroups["BSG1"].Name = %v, want "Storage bootstrap test 1"`, taskGroupController.TaskGroups["BSG1"].Name)
	}
	if taskGroupController.TaskGroups["BSG1"].IsPaused != false {
		t.Fatalf(`taskGroupController.TaskGroups["BSG1"].IsPaused = %v, want false`, taskGroupController.TaskGroups["BSG1"].IsPaused)
	}
	if taskGroupController.TaskGroups["BSG1"].CreatedAt.Year() != 2023 {
		t.Fatalf(`taskGroupController.TaskGroups["BSG1"].CreatedAt.Year() = %v, want 2023`, taskGroupController.TaskGroups["BSG1"].CreatedAt.Year())
	}

	// check task fields
	if len(taskGroupController.TaskGroups["BSG1"].TaskOperators) != 1 {
		t.Fatalf(`len(taskGroupController.TaskGroups["BSG1"].TaskOperators) = %v, want 1`, len(taskGroupController.TaskGroups["BSG1"].TaskOperators))
	}
	if taskGroupController.TaskGroups["BSG1"].TaskOperators["BSG1T1"].Task.Name != "Bootstrap Test Task 1" {
		t.Fatalf(`taskGroupController.TaskGroups["BSG1"].TaskOperators["BSG1T1"].Task.Name = %v, want "Bootstrap Test Task 1"`, taskGroupController.TaskGroups["BSG1"].TaskOperators["BSG1T1"].Task.Name)
	}
	if taskGroupController.TaskGroups["BSG1"].TaskOperators["BSG1T1"].Task.RemainingAttempts != 7 {
		t.Fatalf(`taskGroupController.TaskGroups["BSG1"].TaskOperators["BSG1T1"].Task.RemainingAttempts = %v, want 7`, taskGroupController.TaskGroups["BSG1"].TaskOperators["BSG1T1"].Task.RemainingAttempts)
	}
}

func TestDeleteTask(t *testing.T) {
	cwd, _ := os.Getwd()
	storage := NewJsonFilesystemTaskStorage(cwd + "/test_storage")

	taskGroupController := NewTaskGroupController()
	group := NewTaskGroup("GS3", "Test storage", taskGroupController)
	group.Storage = storage

	task := Task{
		Id:                  "TS3",
		TaskGroupId:         "GS3",
		Name:                "Farewell cruel world",
		Worker:              "worker-a",
		Workgroup:           "",
		Key:                 "",
		RemainingAttempts:   5,
		IsPaused:            false,
		IsComplete:          false,
		Priority:            1,
		RunAfter:            time.Now().Add(5 * time.Second),
		ProgressWeight:      1,
		IsSeed:              false,
		ErrorDelayInSeconds: 5,
		Input:               "",
		Errors:              make([]interface{}, 0),
		CreatedAt:           time.Now(),
		ParentIds:           make([]string, 0),
		Children:            make([]*Task, 0),
	}

	saveTaskGroupError := storage.SaveTaskGroup(group)
	if saveTaskGroupError != nil {
		t.Fatal(`Got an unexpected error when saving task group`, saveTaskGroupError)
	}

	saveTaskError := storage.SaveTask(group, &task)
	if saveTaskError != nil {
		t.Fatal(`Got an unexpected error when saving task`, saveTaskError)
	}

	// Verify existence of task file
	fmt.Println(storage.BasePath + "/task_groups/GS3/TS3.json")
	_, taskFileExistError1 := os.Stat(storage.BasePath + "/task_groups/GS3/TS3.json")
	if os.IsNotExist(taskFileExistError1) {
		t.Fatal(`Task file was not found when it was expected to exist.`, taskFileExistError1)
	}

	// Delete task
	storage.DeleteTask(group, &task)

	// Verify non-existence of task file
	_, taskFileExistError2 := os.Stat(storage.BasePath + "/task_groups/GS3/TS3.json")
	if !os.IsNotExist(taskFileExistError2) {
		t.Fatal(`Task file was found when it should have been deleted.`, taskFileExistError2)
	}
}

func TestDeleteTaskGroup(t *testing.T) {
	cwd, _ := os.Getwd()
	storage := NewJsonFilesystemTaskStorage(cwd + "/test_storage")

	taskGroupController := NewTaskGroupController()
	group := NewTaskGroup("GS4", "Test storage", taskGroupController)
	group.Storage = storage

	task := Task{
		Id:                  "TS4",
		TaskGroupId:         "GS4",
		Name:                "Farewell cruel world",
		Worker:              "worker-a",
		Workgroup:           "",
		Key:                 "",
		RemainingAttempts:   5,
		IsPaused:            false,
		IsComplete:          false,
		Priority:            1,
		RunAfter:            time.Now().Add(5 * time.Second),
		ProgressWeight:      1,
		IsSeed:              false,
		ErrorDelayInSeconds: 5,
		Input:               "",
		Errors:              make([]interface{}, 0),
		CreatedAt:           time.Now(),
		ParentIds:           make([]string, 0),
		Children:            make([]*Task, 0),
	}

	saveTaskGroupError := storage.SaveTaskGroup(group)
	if saveTaskGroupError != nil {
		t.Fatal(`Got an unexpected error when saving task group`, saveTaskGroupError)
	}

	saveTaskError := storage.SaveTask(group, &task)
	if saveTaskError != nil {
		t.Fatal(`Got an unexpected error when saving task`, saveTaskError)
	}

	// Verify existence of task/group file
	_, taskFileExistError1 := os.Stat(storage.BasePath + "/task_groups/GS4/TS4.json")
	if os.IsNotExist(taskFileExistError1) {
		t.Fatal(`Task file was not found when it was expected to exist.`, taskFileExistError1)
	}
	_, taskFileExistError2 := os.Stat(storage.BasePath + "/task_groups/GS4/group.json")
	if os.IsNotExist(taskFileExistError2) {
		t.Fatal(`Task group file was not found when it was expected to exist.`, taskFileExistError2)
	}

	// Delete task group
	storage.DeleteTaskGroup(group)

	// Verify non-existence of task file
	_, taskFileExistError3 := os.Stat(storage.BasePath + "/task_groups/GS4/TS4.json")
	if !os.IsNotExist(taskFileExistError3) {
		t.Fatal(`Task file was found when it should have been deleted.`, taskFileExistError3)
	}
	_, taskFileExistError4 := os.Stat(storage.BasePath + "/task_groups/GS4/group.json")
	if !os.IsNotExist(taskFileExistError4) {
		t.Fatal(`Task group file found when it should have been deleted.`, taskFileExistError4)
	}
}
