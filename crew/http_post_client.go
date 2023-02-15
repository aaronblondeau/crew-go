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

type HttpPostClient struct {
	UrlForTask func(task *Task) (url string, err error) `json:"-"`
}

func NewHttpPostClient() *HttpPostClient {
	urlGenerator := func(task *Task) (url string, err error) {
		baseUrl, ok := os.LookupEnv("CREW_WORKER_BASE_URL")
		if ok {
			return baseUrl + task.Worker, nil
		}
		return "", errors.New("CREW_WORKER_BASE_URL environment variable is not set")
	}
	client := HttpPostClient{
		UrlForTask: urlGenerator,
	}
	return &client
}

type WorkerPayload struct {
	Input   interface{}                 `json:"input"`
	Worker  interface{}                 `json:"worker"`
	Parents []WorkerPayloadParentResult `json:"parents"`
	TaskId  string                      `json:"taskId"`
}

type WorkerPayloadParentResult struct {
	TaskId string      `json:"taskId"`
	Worker string      `json:"worker"`
	Input  interface{} `json:"input"`
	Output interface{} `json:"output"`
}

func (client *HttpPostClient) Post(task *Task, taskGroup *TaskGroup) (response WorkerResponse, err error) {
	// Post body should be
	// input, parents, taskId
	// where parents = {taskId, worker, input, output}

	payloadParents := []WorkerPayloadParentResult{}

	// Get each parent and add result
	for _, parentId := range task.ParentIds {
		// error, output, children
		parentOp, found := taskGroup.TaskOperators[parentId]
		if found {
			parentResult := WorkerPayloadParentResult{
				TaskId: parentOp.Task.Id,
				Worker: parentOp.Task.Worker,
				Input:  parentOp.Task.Input,
				Output: parentOp.Task.Output,
			}
			payloadParents = append(payloadParents, parentResult)
		}
	}

	payload := WorkerPayload{
		Input:   task.Input,
		Parents: payloadParents,
		Worker:  task.Worker,
		TaskId:  task.Id,
	}

	payloadJsonStr, _ := json.Marshal(payload)
	payloadBytes := []byte(payloadJsonStr)

	url, urlError := client.UrlForTask(task)
	if urlError != nil {
		return WorkerResponse{}, urlError
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))

	authHeader, ok := os.LookupEnv("CREW_WORKER_AUTHORIZATION_HEADER")
	if ok {
		req.Header.Set("Authorization", authHeader)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return WorkerResponse{}, err
	}

	defer resp.Body.Close()
	bodyBytes, bodyErr := io.ReadAll(resp.Body)
	if bodyErr != nil {
		return WorkerResponse{}, bodyErr
	}

	// Non 200 response => return response body via call error
	if resp.StatusCode != http.StatusOK {
		return WorkerResponse{}, errors.New(string(bodyBytes))
	}

	bodyString := string(bodyBytes)
	fmt.Println("Worker Response", bodyString)

	workerResp := WorkerResponse{}
	jsonErr := json.Unmarshal(bodyBytes, &workerResp) // when logging code above is no longer needed : json.NewDecoder(resp.Body).Decode(&workerResp)
	if jsonErr != nil {
		return WorkerResponse{}, jsonErr
	}

	return workerResp, nil
}
