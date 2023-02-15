package crew

import (
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

	group := NewTaskGroup("GS1", "Test storage")
	group.Storage = storage

	saveTaskGroupError := storage.SaveTaskGroup(group)
	if saveTaskGroupError != nil {
		t.Fatal(`Got an unexpected error when saving task group`, saveTaskGroupError)
	}
}

func TestStoreTaskGroupAndTask(t *testing.T) {
	cwd, _ := os.Getwd()
	storage := NewJsonFilesystemTaskStorage(cwd + "/test_storage")

	group := NewTaskGroup("GS2", "Test storage")
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

	groups, bootstrapError := storage.Bootstrap(false, client)
	if bootstrapError != nil {
		t.Fatal(`Got an unexpected error when loading bootstrap_a`, bootstrapError)
	}
	if len(groups) != 1 {
		t.Fatalf(`len(groups) = %v, want 1`, len(groups))
	}

	// check group fields
	if groups["BSG1"].Name != "Storage bootstrap test 1" {
		t.Fatalf(`groups["BSG1"].Name = %v, want "Storage bootstrap test 1"`, groups["BSG1"].Name)
	}
	if groups["BSG1"].IsPaused != false {
		t.Fatalf(`groups["BSG1"].IsPaused = %v, want false`, groups["BSG1"].IsPaused)
	}
	if groups["BSG1"].CreatedAt.Year() != 2023 {
		t.Fatalf(`groups["BSG1"].CreatedAt.Year() = %v, want 2023`, groups["BSG1"].CreatedAt.Year())
	}

	// check task fields
	if len(groups["BSG1"].TaskOperators) != 1 {
		t.Fatalf(`len(groups["BSG1"].TaskOperators) = %v, want 1`, len(groups["BSG1"].TaskOperators))
	}
	if groups["BSG1"].TaskOperators["BSG1T1"].Task.Name != "Bootstrap Test Task 1" {
		t.Fatalf(`groups["BSG1"].TaskOperators["BSG1T1"].Task.Name = %v, want "Bootstrap Test Task 1"`, groups["BSG1"].TaskOperators["BSG1T1"].Task.Name)
	}
	if groups["BSG1"].TaskOperators["BSG1T1"].Task.RemainingAttempts != 7 {
		t.Fatalf(`groups["BSG1"].TaskOperators["BSG1T1"].Task.RemainingAttempts = %v, want 7`, groups["BSG1"].TaskOperators["BSG1T1"].Task.RemainingAttempts)
	}
}
