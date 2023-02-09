package crew

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type HttpPostClient struct{}

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

func (client HttpPostClient) Post(URL string, task *Task, taskGroup *TaskGroup) (response WorkerResponse, err error) {
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
	req, err := http.NewRequest("POST", URL, bytes.NewBuffer(payloadBytes))

	authHeader, ok := os.LookupEnv("CREW_WORKER_AUTHORIZATION_HEADER")
	if ok {
		req.Header.Set("Authorization", authHeader)
	}

	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return WorkerResponse{}, err
	}
	defer resp.Body.Close()

	body, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		return WorkerResponse{}, readErr
	}

	workerResp := WorkerResponse{}
	fmt.Println("Worker response", string(body))
	jsonErr := json.Unmarshal(body, &workerResp)
	if readErr != nil {
		return WorkerResponse{}, jsonErr
	}

	return workerResp, nil
}
