package crew

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/test-worker" {
			t.Errorf("Expected to request '/test-worker', got: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Expected Accept: application/json header, got: %s", r.Header.Get("Accept"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"output":"done!"}`))
	}))
	defer server.Close()

	client := NewHttpPostClient()
	client.UrlForTask = func(task *Task) (url string, err error) {
		return server.URL + "/test-worker", nil
	}

	task := Task{
		Id:                "T16",
		TaskGroupId:       "G16",
		Name:              "Http Task",
		Worker:            "test",
		Workgroup:         "",
		Key:               "",
		RemainingAttempts: 5,
		IsPaused:          false,
		IsComplete:        false,
		ParentIds:         []string{},
	}

	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	taskGroup := NewTaskGroup("HTTPG1", "HTTPG1", taskGroupController)

	response, postError := client.Post(&task, taskGroup)

	if postError != nil {
		t.Fatal("Recieved an unexpected response error", postError)
	}
	if response.Output != `done!` {
		t.Fatalf(`response.Output = %v, want %v`, response.Output, `done!`)
	}
	if len(response.Children) != 0 {
		t.Fatalf(`len(response.Children) = %v, want %v`, len(response.Children), 0)
	}
	if response.WorkgroupDelayInSeconds != 0 {
		t.Fatalf(`response.WorkgroupDelayInSeconds = %v, want %v`, response.WorkgroupDelayInSeconds, 0)
	}
	if response.ChildrenDelayInSeconds != 0 {
		t.Fatalf(`response.ChildrenDelayInSeconds = %v, want %v`, response.ChildrenDelayInSeconds, 0)
	}
	if response.Error != nil {
		t.Fatalf(`response.Error = %v, want nil`, response.Error)
	}
}

func TestHttpErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/test-worker" {
			t.Errorf("Expected to request '/test-worker', got: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Expected Accept: application/json header, got: %s", r.Header.Get("Accept"))
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`I am confused...`))
	}))
	defer server.Close()

	client := NewHttpPostClient()
	client.UrlForTask = func(task *Task) (url string, err error) {
		return server.URL + "/test-worker", nil
	}

	task := Task{
		Id:                "T17",
		TaskGroupId:       "G17",
		Name:              "Http Task",
		Worker:            "test",
		Workgroup:         "",
		Key:               "",
		RemainingAttempts: 5,
		IsPaused:          false,
		IsComplete:        false,
		ParentIds:         []string{},
	}

	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	taskGroup := NewTaskGroup("HTTPG2", "HTTPG2", taskGroupController)

	_, postError := client.Post(&task, taskGroup)

	if postError == nil || postError.Error() != "I am confused..." {
		t.Fatalf("Expected to receive error, but got %v", postError)
	}
}

func TestErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/test-worker" {
			t.Errorf("Expected to request '/test-worker', got: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Expected Accept: application/json header, got: %s", r.Header.Get("Accept"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"error":"oops!","workgroupDelayInSeconds":11}`))
	}))
	defer server.Close()

	client := NewHttpPostClient()
	client.UrlForTask = func(task *Task) (url string, err error) {
		return server.URL + "/test-worker", nil
	}

	task := Task{
		Id:                "T18",
		TaskGroupId:       "G18",
		Name:              "Http Task",
		Worker:            "test",
		Workgroup:         "",
		Key:               "",
		RemainingAttempts: 5,
		IsPaused:          false,
		IsComplete:        false,
		ParentIds:         []string{},
	}

	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	taskGroup := NewTaskGroup("HTTPG3", "HTTPG3", taskGroupController)

	response, postError := client.Post(&task, taskGroup)

	if postError != nil {
		t.Fatal("Recieved an unexpected response error", postError)
	}
	if response.Output != nil {
		t.Fatalf(`response.Output = %v, want nil`, response.Output)
	}
	if response.WorkgroupDelayInSeconds != 11 {
		t.Fatalf(`response.WorkgroupDelayInSeconds = %v, want %v`, response.WorkgroupDelayInSeconds, 11)
	}
	if response.Error != "oops!" {
		t.Fatalf(`response.Error = %v, want %v`, response.Error, "oops!")
	}
}

func TestParentDataInPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/test-worker" {
			t.Errorf("Expected to request '/test-worker', got: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Expected Accept: application/json header, got: %s", r.Header.Get("Accept"))
		}

		defer r.Body.Close()
		bodyBytes, bodyErr := io.ReadAll(r.Body)
		if bodyErr != nil {
			t.Error("Failed to parse request body", bodyErr)
		}

		// Should have a parseable payload
		payload := WorkerPayload{}
		json.Unmarshal(bodyBytes, &payload)

		if payload.TaskId != "T19C" {
			t.Fatalf(`payload.TaskId = %v, want %v`, payload.TaskId, "T19C")
		}
		if payload.Worker != "test" {
			t.Fatalf(`payload.Worker = %v, want %v`, payload.Worker, "test")
		}
		if len(payload.Parents) != 1 {
			t.Fatalf(`len(payload.Parents) = %v, want %v`, len(payload.Parents), 1)
		}
		if payload.Parents[0].TaskId != "T19P" {
			t.Fatalf(`payload.Parents[0].TaskId = %v, want %v`, payload.Parents[0].TaskId, "T19P")
		}
		parentInput := payload.Parents[0].Input.(map[string]interface{})
		if parentInput["in"] != float64(42) {
			t.Fatalf(`parentInput["in"] = %v, want %v`, parentInput["in"], 42)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"output":"done!"}`))
	}))
	defer server.Close()

	client := NewHttpPostClient()
	client.UrlForTask = func(task *Task) (url string, err error) {
		return server.URL + "/test-worker", nil
	}

	task := Task{
		Id:                "T19P",
		TaskGroupId:       "G19",
		Name:              "Http Task",
		Worker:            "test",
		Workgroup:         "",
		Key:               "",
		RemainingAttempts: 4,
		IsPaused:          false,
		IsComplete:        true,
		ParentIds:         []string{},
		Input:             map[string]int{"in": 42},
		Output:            map[string]int{"foo": 1, "bar": 2},
	}

	child := Task{
		Id:                "T19C",
		TaskGroupId:       "G19",
		Name:              "A Child Task",
		Worker:            "test",
		Workgroup:         "",
		Key:               "",
		RemainingAttempts: 5,
		IsPaused:          false,
		IsComplete:        false,
		ParentIds:         []string{"T19P"},
	}

	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	taskGroup := NewTaskGroup("HTTPG4", "HTTPG4", taskGroupController)

	taskGroup.PreloadTasks([]*Task{&task, &child}, client)

	_, postError := client.Post(&child, taskGroup)
	if postError != nil {
		t.Fatal("Recieved an unexpected response error", postError)
	}
}

func TestChildrenResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/test-worker" {
			t.Errorf("Expected to request '/test-worker', got: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Expected Accept: application/json header, got: %s", r.Header.Get("Accept"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"childrenDelayInSeconds":12,"children":[{"id":"T20C1","name":"Child1","worker":"testx"},{"id":"T20C2","name":"Child2","worker":"testx"}]}`))
	}))
	defer server.Close()

	client := NewHttpPostClient()
	client.UrlForTask = func(task *Task) (url string, err error) {
		return server.URL + "/test-worker", nil
	}

	task := Task{
		Id:                "T20",
		TaskGroupId:       "G20",
		Name:              "Http Task",
		Worker:            "test",
		Workgroup:         "",
		Key:               "",
		RemainingAttempts: 5,
		IsPaused:          false,
		IsComplete:        false,
		ParentIds:         []string{},
	}

	taskGroupController := NewTaskGroupController(NewMemoryTaskStorage())
	taskGroup := NewTaskGroup("HTTPG5", "HTTPG5", taskGroupController)

	response, postError := client.Post(&task, taskGroup)

	if postError != nil {
		t.Fatal("Recieved an unexpected response error", postError)
	}
	if len(response.Children) != 2 {
		t.Fatalf(`len(response.Children) = %v, want %v`, len(response.Children), 2)
	}
	if response.ChildrenDelayInSeconds != 12 {
		t.Fatalf(`response.ChildrenDelayInSeconds = %v, want %v`, response.ChildrenDelayInSeconds, 12)
	}
	if response.Children[0].Id != "T20C1" {
		t.Fatalf(`response.Children[0].Id = %v, want %v`, response.Children[0].Id, "T20C1")
	}
}
