package crew

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type ChildTask struct {
	Id                  string      `json:"id"`
	Name                string      `json:"name"`
	Worker              string      `json:"worker"`
	Workgroup           string      `json:"workgroup"`
	Key                 string      `json:"key"`
	RemainingAttempts   int         `json:"remainingAttempts"`
	IsPaused            bool        `json:"isPaused"`
	RunAfter            time.Time   `json:"runAfter"`
	ErrorDelayInSeconds int         `json:"errorDelayInSeconds"`
	Input               interface{} `json:"input"`
	ParentIds           []string    `json:"parentIds"`
}

// WorkerResponse defines the schema of output returned from workers.
type WorkerResponse struct {
	Output                  interface{}  `json:"output"`
	Children                []*ChildTask `json:"children"`
	WorkgroupDelayInSeconds int          `json:"workgroupDelayInSeconds"`
	ChildrenDelayInSeconds  int          `json:"childrenDelayInSeconds"`
	Error                   interface{}  `json:"error"`
}

// TaskClient defines the interface for delivering tasks to workers.
type TaskClient interface {
	Post(task *Task, parents []*Task) (response WorkerResponse, err error)
}

// HttpPostClient delivers tasks to workers via http post.
type HttpPostClient struct {
	UrlForTask func(task *Task) (url string, err error) `json:"-"`
}

// NewHttpPostClient creates a new HttpPostClient.
func NewHttpPostClient() *HttpPostClient {
	urlGenerator := func(task *Task) (url string, err error) {
		baseUrl := os.Getenv("CREW_WORKER_BASE_URL")
		if baseUrl == "" {
			port := os.Getenv("PORT")
			if port == "" {
				port = "8090"
			}
			baseUrl = "http://localhost:" + port + "/demo/"
			log.Println("CREW_WORKER_BASE_URL not set, defaulting to " + baseUrl)

		}
		return baseUrl + task.Worker, nil
	}
	client := HttpPostClient{
		UrlForTask: urlGenerator,
	}
	return &client
}

// WorkerPayload defines the input sent to a worker (post body).
type WorkerPayload struct {
	Input   interface{}                 `json:"input"`
	Worker  string                      `json:"worker"`
	Parents []WorkerPayloadParentResult `json:"parents"`
	TaskId  string                      `json:"taskId"`
}

// WorkerPayloadParentResult defines the schema for output from a worker.
type WorkerPayloadParentResult struct {
	TaskId string      `json:"taskId"`
	Worker string      `json:"worker"`
	Input  interface{} `json:"input"`
	Output interface{} `json:"output"`
}

// Post delivers a task to a worker.
func (client *HttpPostClient) Post(task *Task, parents []*Task) (response WorkerResponse, err error) {
	// Start preparing the task input by gathering info from parents
	payloadParents := []WorkerPayloadParentResult{}

	// Get each parent and add result
	for _, parent := range parents {
		// error, output, children
		parentResult := WorkerPayloadParentResult{
			TaskId: parent.Id,
			Worker: parent.Worker,
			Input:  parent.Input,
			Output: parent.Output,
		}
		payloadParents = append(payloadParents, parentResult)
	}

	payload := WorkerPayload{
		Input:   task.Input,
		Parents: payloadParents,
		Worker:  task.Worker,
		TaskId:  task.Id,
	}

	payloadJsonStr, buildPayloadErr := json.Marshal(payload)
	if buildPayloadErr != nil {
		return WorkerResponse{}, buildPayloadErr
	}
	payloadBytes := []byte(payloadJsonStr)

	url, urlError := client.UrlForTask(task)
	if urlError != nil {
		return WorkerResponse{}, urlError
	}

	// fmt.Println("~~ Sending task to", url, task.Worker)

	// Build the request
	req, reqSetupErr := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if reqSetupErr != nil {
		return WorkerResponse{}, reqSetupErr
	}
	authHeader, ok := os.LookupEnv("CREW_WORKER_AUTHORIZATION_HEADER")
	if ok {
		req.Header.Set("Authorization", authHeader)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// fmt.Println("~~ Worker Request", string(payloadJsonStr))

	// Send the request
	httpClient := &http.Client{}
	// Set a very generous timeout (crew docs recommend tasks should complete in 60 seconds)
	// TODO - set this with an env var?
	httpClient.Timeout = 300 * time.Second
	resp, err := httpClient.Do(req)
	if err != nil {
		return WorkerResponse{}, err
	}

	// Read the response
	defer resp.Body.Close()
	bodyBytes, bodyErr := io.ReadAll(resp.Body)
	if bodyErr != nil {
		return WorkerResponse{}, bodyErr
	}

	// Non 200 response => return response body via call error
	if resp.StatusCode != http.StatusOK {
		errorMessage := fmt.Sprintf("Http call to worker returned non 200 status code: %d, body: %v", resp.StatusCode, string(bodyBytes))
		return WorkerResponse{}, errors.New(errorMessage)
	}

	// bodyString := string(bodyBytes)
	// fmt.Println("~~ Worker Response", bodyString)

	// Parse the response
	workerResp := WorkerResponse{}
	jsonErr := json.Unmarshal(bodyBytes, &workerResp) // when logging code above is no longer needed : json.NewDecoder(resp.Body).Decode(&workerResp)
	if jsonErr != nil {
		return WorkerResponse{}, jsonErr
	}

	return workerResp, nil
}
