package crew

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

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
			fmt.Println("CREW_WORKER_BASE_URL not set, defaulting to " + baseUrl)

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
	Input       interface{}                 `json:"input"`
	Worker      interface{}                 `json:"worker"`
	Parents     []WorkerPayloadParentResult `json:"parents"`
	TaskAddress string                      `json:"taskAddress"`
}

// WorkerPayloadParentResult defines the schema for output from a worker.
type WorkerPayloadParentResult struct {
	TaskAddress string      `json:"taskAddress"`
	Worker      string      `json:"worker"`
	Input       interface{} `json:"input"`
	Output      interface{} `json:"output"`
}

// Post delivers a task to a worker.
func (client *HttpPostClient) Post(task *Task) (response WorkerResponse, err error) {
	// Start preparing the task input by gathering info from parents
	payloadParents := []WorkerPayloadParentResult{}

	// Get each parent and add result
	for _, parent := range task.Parents {
		// error, output, children

		parentResult := WorkerPayloadParentResult{
			TaskAddress: parent.Address,
			Worker:      parent.Worker,
			Input:       parent.Input,
			Output:      parent.Output,
		}
		payloadParents = append(payloadParents, parentResult)

	}

	payload := WorkerPayload{
		Input:       task.Input,
		Parents:     payloadParents,
		Worker:      task.Worker,
		TaskAddress: task.Address,
	}

	payloadJsonStr, _ := json.Marshal(payload)
	payloadBytes := []byte(payloadJsonStr)

	url, urlError := client.UrlForTask(task)
	if urlError != nil {
		return WorkerResponse{}, urlError
	}

	// Build the request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	authHeader, ok := os.LookupEnv("CREW_WORKER_AUTHORIZATION_HEADER")
	if ok {
		req.Header.Set("Authorization", authHeader)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// fmt.Println("~~ Worker Request", string(payloadJsonStr))

	// Send the request
	httpClient := &http.Client{}
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
		return WorkerResponse{}, errors.New(string(bodyBytes))
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
