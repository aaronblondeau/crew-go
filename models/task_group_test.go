package models

import (
	"testing"
	"time"
)

func TestPrepareInflatesChildren(t *testing.T) {
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

	if parent.Children[0] != &task {
		t.Fatal("Parent task's Chilren slice was not inflated!")
	}
}
