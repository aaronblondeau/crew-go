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

	task := NewTask()
	task.Id = "task11"
	task.Name = "task11"
	task.Worker = "worker-a"
	parents := make([]*Task, 0)

	response, postError := client.Post(task, parents)

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
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`I am confused...`))
	}))
	defer server.Close()

	client := NewHttpPostClient()
	client.UrlForTask = func(task *Task) (url string, err error) {
		return server.URL + "/test-worker", nil
	}

	task := NewTask()
	task.Id = "task12"
	task.Name = "task12"
	task.Worker = "worker-a"
	parents := make([]*Task, 0)

	_, postError := client.Post(task, parents)

	if postError == nil || postError.Error() != "Http call to worker returned non 200 status code: 500, body: I am confused..." {
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

	task := NewTask()
	task.Id = "task13"
	task.Name = "task13"
	task.Worker = "worker-a"
	parents := make([]*Task, 0)

	response, postError := client.Post(task, parents)

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

		if payload.TaskId != "task15" {
			t.Fatalf(`payload.TaskId = %v, want %v`, payload.TaskId, "task15")
		}
		if payload.Worker != "worker-a" {
			t.Fatalf(`payload.Worker = %v, want %v`, payload.Worker, "worker-a")
		}
		if len(payload.Parents) != 1 {
			t.Fatalf(`len(payload.Parents) = %v, want %v`, len(payload.Parents), 1)
		}
		if payload.Parents[0].TaskId != "task14" {
			t.Fatalf(`payload.Parents[0].TaskId = %v, want %v`, payload.Parents[0].TaskId, "task14")
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

	task := NewTask()
	task.Id = "task14"
	task.Name = "task14"
	task.Worker = "worker-a"
	task.IsComplete = true
	task.Input = map[string]int{"in": 42}
	task.Output = map[string]int{"foo": 1, "bar": 2}

	child := NewTask()
	child.Id = "task15"
	child.Name = "task15"
	child.Worker = "worker-a"
	child.ParentIds = []string{"task14"}

	parents := make([]*Task, 0)
	parents = append(parents, task)

	_, postError := client.Post(child, parents)
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
		w.Write([]byte(`{"childrenDelayInSeconds":12,"children":[{"id":"task17","name":"Child1","worker":"testx"},{"id":"T20C2","name":"Child2","worker":"testx"}]}`))
	}))
	defer server.Close()

	client := NewHttpPostClient()
	client.UrlForTask = func(task *Task) (url string, err error) {
		return server.URL + "/test-worker", nil
	}

	task := NewTask()
	task.Id = "task16"
	task.Name = "task16"
	task.Worker = "worker-a"
	parents := make([]*Task, 0)

	response, postError := client.Post(task, parents)

	if postError != nil {
		t.Fatal("Recieved an unexpected response error", postError)
	}
	if len(response.Children) != 2 {
		t.Fatalf(`len(response.Children) = %v, want %v`, len(response.Children), 2)
	}
	if response.ChildrenDelayInSeconds != 12 {
		t.Fatalf(`response.ChildrenDelayInSeconds = %v, want %v`, response.ChildrenDelayInSeconds, 12)
	}
	if response.Children[0].Id != "task17" {
		t.Fatalf(`response.Children[0].Id = %v, want %v`, response.Children[0].Id, "task17")
	}
}
