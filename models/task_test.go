package models

import (
	"testing"
	"time"
)

func TestCanExecute(t *testing.T) {
	task := Task{
		Id:                "T1",
		TaskGroupId:       "G1",
		Name:              "Task One",
		Channel:           "worker-a",
		Workgroup:         "",
		Key:               "T1",
		RemainingAttempts: 5,
		IsPaused:          false,
		IsComplete:        false,
		Priority:          1,
		ProgressWeight:    1,
	}
	group := TaskGroup{
		Id:            "G1",
		Name:          "Test",
		IsPaused:      false,
		CreatedAt:     time.Now(),
		TaskOperators: make(map[string]*TaskOperator),
		Events:        make(chan TaskGroupEvent, 8),
	}

	canExecute := task.CanExecute(&group)
	if !canExecute {
		t.Fatalf(`CanExecute() = false, want true`)
	}
}

func TestCannotExecuteIfTaskIsPaused(t *testing.T) {
	task := Task{
		Id:                "T1",
		TaskGroupId:       "G1",
		Name:              "Task One",
		Channel:           "worker-a",
		Workgroup:         "",
		Key:               "T1",
		RemainingAttempts: 5,
		// Tasks cannot be executed if they are paused
		IsPaused:            true,
		IsComplete:          false,
		Priority:            1,
		ProgressWeight:      1,
		IsSeed:              false,
		ErrorDelayInSeconds: 5,
		Input:               "Test",
		Errors:              make([]interface{}, 0),
		CreatedAt:           time.Now(),
		ParentIds:           make([]string, 0),
		Children:            make([]*Task, 0),
	}
	group := TaskGroup{
		Id:            "G1",
		Name:          "Test",
		IsPaused:      false,
		CreatedAt:     time.Now(),
		TaskOperators: make(map[string]*TaskOperator),
		Events:        make(chan TaskGroupEvent, 8),
	}

	canExecute := task.CanExecute(&group)
	if canExecute {
		t.Fatalf(`CanExecute() = true, want false (task is paused)`)
	}
}

func TestCannotExecuteIfParentsIncomplete(t *testing.T) {
	parent := Task{
		Id:                "T1",
		TaskGroupId:       "G1",
		Name:              "Incomplete Task Parent",
		Channel:           "test",
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
		Channel:           "test",
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
		Events:        make(chan TaskGroupEvent, 8),
	}

	testChannel := Channel{
		Id:  "test",
		Url: "https://example.com/test",
	}
	channels := make(map[string]Channel)
	channels[testChannel.Id] = testChannel

	group.Prepare([]*Task{&parent, &task}, channels)

	canExecute := task.CanExecute(&group)
	if canExecute {
		t.Fatalf(`CanExecute() = true, want false (parent not complete)`)
	}
}
