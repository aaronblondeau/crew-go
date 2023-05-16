package crew

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// A ThrottlePushQuery is a request to the throttler to see if there is enough bandwidth for a worker to run.
type ThrottlePushQuery struct {
	TaskId string
	Worker string
	Resp   chan bool
}

// ThrottlePopQuery is a request to the throttler to notify that a worker is done.
type ThrottlePopQuery struct {
	TaskId string
	Worker string
}

type ShutdownPoolMessage struct {
	Done chan bool
}

type APIError struct {
	Message string
	Code    int
}

type APIGetGroupsRequest struct {
	Resp     chan APIGetGroupsResponse
	Search   string
	Page     int
	PageSize int
}

type APIGetGroupsResponse struct {
	Error  *APIError
	Groups []*TaskGroup
	Total  int
}

type APIGetGroupRequest struct {
	Resp chan APIGetGroupResponse
	Id   string
}

type APIGetGroupResponse struct {
	Group *TaskGroup
	Error *APIError
}

type APIGetTasksRequest struct {
	Resp     chan APIGetTasksResponse
	GroupId  string
	Search   string
	Page     int
	PageSize int
}

type APIGetTasksResponse struct {
	Error *APIError
	Tasks []*Task
	Total int
}

type APIGetGroupProgressRequest struct {
	Resp    chan APIGetGroupProgressResponse
	GroupId string
}

type APIGetGroupProgressResponse struct {
	Error            *APIError
	CompletedPercent float64
}

type APIGetTaskRequest struct {
	Resp chan APIGetTaskResponse
	Id   string
}

type APIGetTaskResponse struct {
	Task  *Task
	Error *APIError
}

type APICreateGroupRequest struct {
	Resp  chan APICreateGroupResponse
	Group *TaskGroup
}

type APICreateGroupResponse struct {
	Group *TaskGroup
	Error *APIError
}

type APICreateTaskRequest struct {
	Resp chan APICreateTaskResponse
	Task *Task
}

type APICreateTaskResponse struct {
	Task  *Task
	Error *APIError
}

type APIDeleteGroupRequest struct {
	Resp chan APIDeleteGroupResponse
	Id   string
}

type APIDeleteGroupResponse struct {
	Error *APIError
}

type APIDeleteTaskRequest struct {
	Resp chan APIDeleteTaskResponse
	Id   string
}

type APIDeleteTaskResponse struct {
	Error *APIError
}

type APIResetGroupRequest struct {
	Resp              chan APIResetGroupResponse
	Id                string
	RemainingAttempts int
}

type APIResetGroupResponse struct {
	Error *APIError
}

type APIRetryGroupRequest struct {
	Resp              chan APIRetryGroupResponse
	Id                string
	RemainingAttempts int
}

type APIRetryGroupResponse struct {
	Error *APIError
}

type APIPauseResumeGroupRequest struct {
	Resp     chan APIPauseResumeGroupResponse
	Id       string
	IsPaused bool
}

type APIPauseResumeGroupResponse struct {
	Error *APIError
}

type APIResetTaskRequest struct {
	Resp              chan APIResetTaskResponse
	Id                string
	RemainingAttempts int
}

type APIResetTaskResponse struct {
	Error *APIError
	Task  *Task
}

type APIRetryTaskRequest struct {
	Resp              chan APIRetryTaskResponse
	Id                string
	RemainingAttempts int
}

type APIRetryTaskResponse struct {
	Error *APIError
	Task  *Task
}

type APIUpdateGroupRequest struct {
	Resp   chan APIUpdateGroupResponse
	Id     string
	Update map[string]interface{}
}

type APIUpdateGroupResponse struct {
	Error *APIError
	Group *TaskGroup
}

type APIUpdateTaskRequest struct {
	Resp   chan APIUpdateTaskResponse
	Id     string
	Update map[string]interface{}
}

type APIUpdateTaskResponse struct {
	Error *APIError
	Task  *Task
}

// A TaskGroupUpdateEvent notifies listeners of changes to a TaskGroup's state.
type TaskGroupUpdateEvent struct {
	Event     string     `json:"type"`
	TaskGroup *TaskGroup `json:"task_group"`
}

// A TaskUpdateEvent notifies listeners of changes to a Task's state.
type TaskUpdateEvent struct {
	Event string `json:"type"`
	Task  *Task  `json:"task"`
}

type Throttler struct {
	Push chan ThrottlePushQuery
	Pop  chan ThrottlePopQuery
}

type TaskPool struct {
	Inbox            chan interface{}
	Tasks            map[string]*Task
	Groups           map[string]*TaskGroup
	TasksByGroup     map[string][]*Task
	TasksByKey       map[string][]*Task
	TasksByWorkgroup map[string][]*Task
	LostAndFound     map[string][]interface{}
	Client           TaskClient
	Storage          TaskStorage
	Throttler        Throttler
	TaskGroupUpdates chan TaskGroupUpdateEvent
	TaskUpdates      chan TaskUpdateEvent
}

func NewTaskPool(client TaskClient, storage TaskStorage, throttler Throttler) *TaskPool {
	pool := TaskPool{
		Inbox:            make(chan interface{}, 32),
		Tasks:            make(map[string]*Task),
		Groups:           make(map[string]*TaskGroup),
		TasksByGroup:     make(map[string][]*Task),
		TasksByKey:       make(map[string][]*Task),
		TasksByWorkgroup: make(map[string][]*Task),
		LostAndFound:     make(map[string][]interface{}),
		Client:           client,
		Storage:          storage,
		Throttler:        throttler,
		TaskGroupUpdates: make(chan TaskGroupUpdateEvent, 32),
		TaskUpdates:      make(chan TaskUpdateEvent, 32),
	}
	return &pool
}

func (pool *TaskPool) Start() {
	// First, load all groups and tasks from storage
	pool.Storage.Bootstrap(pool)

	// Build initial indexes of tasks by group, key, workgroup
	for _, task := range pool.Tasks {
		pool.AddTaskToIndexes(task)
	}

	// Process messages - that's all we do!
	go func() {
		for {
			msg := <-pool.Inbox
			switch msg.(type) {
			case UpdateTaskMessage:
				// Send to recipient
				pool.ForwardMessage(msg.(UpdateTaskMessage).ToTaskId, msg)
			case RefreshTaskIndexesMessage:
				// Remove task from indexes
				pool.RemoveTaskFromIndexes(msg.(RefreshTaskIndexesMessage).Task)

				// Add back to indexes
				pool.AddTaskToIndexes(msg.(RefreshTaskIndexesMessage).Task)
			case ExecutedMessage:
				pool.HandleExecutedMessage(msg.(ExecutedMessage))
			case ExecuteMessage:
				// Send to recipient
				pool.ForwardMessage(msg.(ExecuteMessage).ToTaskId, msg)
			case ParentCompletedMessage:
				// Send to recipient
				pool.ForwardMessage(msg.(ParentCompletedMessage).ToTaskId, msg)
			case DeleteTaskMessage:
				// Send to recipient
				pool.ForwardMessage(msg.(DeleteTaskMessage).ToTaskId, msg)
			case ContinuationMessage:
				pool.CreateTask(msg.(ContinuationMessage).Task)
			case ChildIntroductionMessage:
				// Send to recipient
				pool.ForwardMessage(msg.(ChildIntroductionMessage).ToTaskId, msg)
			case ChildIntroductionAcknowledgementMessage:
				// Send to recipient
				pool.ForwardMessage(msg.(ChildIntroductionAcknowledgementMessage).ToTaskId, msg)
			case TaskUpdatedMessage:
				pool.EmitTaskUpdate(msg.(TaskUpdatedMessage).Event, msg.(TaskUpdatedMessage).Task)
			case ShutdownPoolMessage:
				pool.Stop(msg.(ShutdownPoolMessage).Done)
			case APIGetGroupsRequest:
				pool.GetGroups(msg.(APIGetGroupsRequest))
			case APIGetGroupRequest:
				pool.GetGroup(msg.(APIGetGroupRequest))
			case APIGetTasksRequest:
				pool.GetTasks(msg.(APIGetTasksRequest))
			case APIGetGroupProgressRequest:
				pool.GetGroupProgress(msg.(APIGetGroupProgressRequest))
			case APIGetTaskRequest:
				pool.GetTask(msg.(APIGetTaskRequest))
			case APICreateGroupRequest:
				pool.CreateGroup(msg.(APICreateGroupRequest))
			case APICreateTaskRequest:
				pool.CreateTaskMsg(msg.(APICreateTaskRequest))
			case APIDeleteGroupRequest:
				pool.DeleteGroup(msg.(APIDeleteGroupRequest))
			case APIDeleteTaskRequest:
				pool.DeleteTask(msg.(APIDeleteTaskRequest))
			case APIResetGroupRequest:
				pool.ResetGroup(msg.(APIResetGroupRequest))
			case APIRetryGroupRequest:
				pool.RetryGroup(msg.(APIRetryGroupRequest))
			case APIPauseResumeGroupRequest:
				pool.PauseResumeGroup(msg.(APIPauseResumeGroupRequest))
			case APIResetTaskRequest:
				pool.ResetTaskMsg(msg.(APIResetTaskRequest))
			case APIRetryTaskRequest:
				pool.RetryTask(msg.(APIRetryTaskRequest))
			case APIUpdateGroupRequest:
				pool.UpdateGroup(msg.(APIUpdateGroupRequest))
			case APIUpdateTaskRequest:
				pool.UpdateTask(msg.(APIUpdateTaskRequest))
			default:
				// TODO - error
			}
		}
	}()

	// Once message queue is running, start boostrapped tasks
	for _, task := range pool.Tasks {
		task.Start(pool.Client, pool.Storage, &pool.Throttler, pool.Inbox)
	}
}

func (pool *TaskPool) ForwardMessage(taskId string, msg interface{}) {
	task, found := pool.Tasks[taskId]
	if !found {
		pool.LostAndFound[taskId] = append(pool.LostAndFound[taskId], msg)
	} else {
		task.Inbox <- msg
	}
}

func (pool *TaskPool) Stop(done chan bool) {
	// Shutdown all tasks
	for _, task := range pool.Tasks {
		task.Stop()
	}

	// Close pool inbox
	close(pool.Inbox)

	// Signal done
	done <- true
}

func (pool *TaskPool) AddTaskToIndexes(task *Task) {
	pool.TasksByGroup[task.GroupId] = append(pool.TasksByGroup[task.GroupId], task)
	if task.Key != "" {
		pool.TasksByKey[task.Key] = append(pool.TasksByKey[task.Key], task)
	}
	if task.Workgroup != "" {
		pool.TasksByWorkgroup[task.Workgroup] = append(pool.TasksByWorkgroup[task.Workgroup], task)
	}
}

func (pool *TaskPool) HandleExecutedMessage(msg ExecutedMessage) {
	// Process children delays
	if msg.ChildrenDelayInSeconds > 0 {
		task, hasTask := pool.Tasks[msg.TaskId]
		if hasTask {
			for _, childId := range task.ChildIds {
				pool.ForwardMessage(childId, DelayTaskMessage{
					ToTaskId:       childId,
					DelayInSeconds: msg.ChildrenDelayInSeconds,
				})
			}
		}
	}

	// Process workgroup delays
	if msg.WorkgroupDelayInSeconds > 0 && msg.Workgroup != "" {
		tasks, hasWorkgroupTasks := pool.TasksByWorkgroup[msg.Workgroup]
		if hasWorkgroupTasks {
			for _, task := range tasks {
				pool.ForwardMessage(task.Id, DelayTaskMessage{
					ToTaskId:       task.Id,
					DelayInSeconds: msg.WorkgroupDelayInSeconds,
				})
			}
		}
	}

	// De-duplicate
	if msg.IsComplete && msg.Key != "" {
		tasks, hasKeyMatches := pool.TasksByKey[msg.Key]
		if hasKeyMatches {
			for _, task := range tasks {
				pool.ForwardMessage(task.Id, DeduplicateTaskMessage{
					ToTaskId: task.Id,
					Output:   task.Output,
				})
			}
		}
	}
}

func (pool *TaskPool) DestroyTask(task *Task) {
	task.Inbox <- DeleteTaskMessage{
		ToTaskId: task.Id,
	}

	// Remove task from indexes
	pool.RemoveTaskFromIndexes(task)

	// Remove task from pool
	delete(pool.Tasks, task.Id)
}

func (pool *TaskPool) ResetTask(task *Task, remainingAttempts int) {
	// Send update to task that resets it

	task.Inbox <- UpdateTaskMessage{
		ToTaskId: task.Id,
		Update: map[string]interface{}{
			"remainingAttempts": remainingAttempts,
			"isComplete":        false,
			"output":            nil,
			"errors":            make([]string, 0),
			"runAfter":          time.Now(),
		},
	}
}

func (pool *TaskPool) RemoveTaskFromIndexes(task *Task) {
	// Remove task from TasksByGroup
	groupTasks, foundGroup := pool.TasksByGroup[task.GroupId]
	if foundGroup {
		for i, t := range groupTasks {
			if t.Id == task.Id {
				pool.TasksByGroup[task.GroupId] = append(groupTasks[:i], groupTasks[i+1:]...)
				break
			}
		}
	}

	// Remove task from TasksByKey
	if task.Key != "" {
		keyTasks, foundKey := pool.TasksByKey[task.Key]
		if foundKey {
			for i, t := range keyTasks {
				if t.Id == task.Id {
					pool.TasksByKey[task.Key] = append(keyTasks[:i], keyTasks[i+1:]...)
					break
				}
			}
		}
	}

	// Remove task from TasksByWorkgroup
	if task.Workgroup != "" {
		workgroupTasks, foundWorkgroup := pool.TasksByWorkgroup[task.Workgroup]
		if foundWorkgroup {
			for i, t := range workgroupTasks {
				if t.Id == task.Id {
					pool.TasksByWorkgroup[task.Workgroup] = append(workgroupTasks[:i], workgroupTasks[i+1:]...)
					break
				}
			}
		}
	}
}

func (pool *TaskPool) EmitTaskGroupUpdate(event string, group *TaskGroup) {
	// There may not be a listener to these so send with channel select
	select {
	case pool.TaskGroupUpdates <- TaskGroupUpdateEvent{
		Event:     event,
		TaskGroup: group,
	}:
	default:
	}
}

func (pool *TaskPool) EmitTaskUpdate(event string, group *Task) {
	// There may not be a listener to these so send with channel select
	select {
	case pool.TaskUpdates <- TaskUpdateEvent{
		Event: event,
		Task:  group,
	}:
	default:
	}
}

func (pool *TaskPool) GetGroup(msg APIGetGroupRequest) {
	group, found := pool.Groups[msg.Id]
	if !found {
		msg.Resp <- APIGetGroupResponse{
			Error: &APIError{
				Code:    404,
				Message: fmt.Sprintf("Task group with id %v not found.", msg.Id),
			},
		}
	} else {
		msg.Resp <- APIGetGroupResponse{
			Group: group,
		}
	}
}

func (pool *TaskPool) GetGroups(msg APIGetGroupsRequest) {
	// create an all groups slice (while performing search)
	groups := make([]*TaskGroup, 0)
	for _, group := range pool.Groups {
		if msg.Search != "" {
			if strings.Contains(strings.ToLower(group.Name), strings.ToLower(msg.Search)) {
				groups = append(groups, group)
			}
		} else {
			groups = append(groups, group)
		}
	}

	// sort all groups slice
	sort.Slice(groups, func(a, b int) bool {
		return groups[a].CreatedAt.Before(groups[b].CreatedAt)
	})

	pageSize := 20
	if msg.PageSize == 0 {
		pageSize = len(groups)
	}

	// pagninate groups slice
	slice_start := (msg.Page - 1) * pageSize
	slice_end := slice_start + pageSize
	slice_count := len(groups)
	if slice_start < 0 {
		slice_start = 0
	}
	if slice_end < slice_start {
		slice_end = slice_start
	}
	if slice_start > slice_count {
		slice_start = slice_count
	}
	if slice_end > slice_count {
		slice_end = slice_count
	}
	sliced := groups[slice_start:slice_end]

	msg.Resp <- APIGetGroupsResponse{
		Groups: sliced,
		Total:  slice_count,
	}
}

func (pool *TaskPool) GetTasks(msg APIGetTasksRequest) {
	// Make sure group exists
	_, found := pool.Groups[msg.GroupId]
	if !found {
		msg.Resp <- APIGetTasksResponse{
			Error: &APIError{
				Code:    404,
				Message: fmt.Sprintf("Task group with id %v not found.", msg.GroupId),
			},
		}
		return
	}

	// create an all tasks slice
	tasks := make([]*Task, 0)
	for _, task := range pool.TasksByGroup[msg.GroupId] {
		if msg.Search != "" {
			if strings.Contains(strings.ToLower(task.Name), strings.ToLower(msg.Search)) {
				tasks = append(tasks, task)
			}
		} else {
			tasks = append(tasks, task)
		}
	}

	// sort all tasks slice
	sort.Slice(tasks, func(a, b int) bool {
		return tasks[a].CreatedAt.Before(tasks[b].CreatedAt)
	})

	pageSize := 20
	if msg.PageSize == 0 {
		pageSize = len(tasks)
	}

	// pagninate tasks slice
	slice_start := (msg.Page - 1) * pageSize
	slice_end := slice_start + pageSize
	slice_count := len(tasks)
	if slice_start < 0 {
		slice_start = 0
	}
	if slice_end < slice_start {
		slice_end = slice_start
	}
	if slice_start > slice_count {
		slice_start = slice_count
	}
	if slice_end > slice_count {
		slice_end = slice_count
	}
	sliced := tasks[slice_start:slice_end]

	msg.Resp <- APIGetTasksResponse{
		Tasks: sliced,
		Total: slice_count,
	}
}

func (pool *TaskPool) GetGroupProgress(msg APIGetGroupProgressRequest) {
	// Make sure group exists
	_, found := pool.Groups[msg.GroupId]
	if !found {
		msg.Resp <- APIGetGroupProgressResponse{
			Error: &APIError{
				Code:    404,
				Message: fmt.Sprintf("Task group with id %v not found.", msg.GroupId),
			},
		}
		return
	}

	total := 0
	completed := 0
	for _, task := range pool.TasksByGroup[msg.GroupId] {
		if task.IsComplete {
			completed++
		}
		total++
	}
	completedPercent := 0.0
	if total > 0 {
		completedPercent = float64(completed) / float64(total)
	}
	msg.Resp <- APIGetGroupProgressResponse{
		CompletedPercent: completedPercent,
	}
}

func (pool *TaskPool) GetTask(msg APIGetTaskRequest) {
	task, found := pool.Tasks[msg.Id]
	if !found {
		msg.Resp <- APIGetTaskResponse{
			Error: &APIError{
				Code:    404,
				Message: fmt.Sprintf("Task with id %v not found.", msg.Id),
			},
		}
	} else {
		msg.Resp <- APIGetTaskResponse{
			Task: task,
		}
	}
}

func (pool *TaskPool) CreateGroup(msg APICreateGroupRequest) {
	// Make sure group does not exist already
	group, found := pool.Groups[msg.Group.Id]
	if found {
		msg.Resp <- APICreateGroupResponse{
			Error: &APIError{
				Code:    409,
				Message: fmt.Sprintf("Task group with id %v already exists.", msg.Group.Id),
			},
		}
		return
	}

	// Add group to pool
	pool.Groups[msg.Group.Id] = msg.Group

	// Setup group's index
	pool.TasksByGroup[msg.Group.Id] = make([]*Task, 0)

	// Save group
	pool.Storage.SaveTaskGroup(msg.Group)

	// Send response
	msg.Resp <- APICreateGroupResponse{
		Group: msg.Group,
	}

	pool.EmitTaskGroupUpdate("create", group)
}

func (pool *TaskPool) CreateTask(task *Task) {
	// Ensure task has an id
	if task.Id == "" {
		task.Id = uuid.New().String()
	}

	// Add task to pool
	pool.Tasks[task.Id] = task

	// Update indexes
	pool.AddTaskToIndexes(task)

	// Start task
	task.Start(pool.Client, pool.Storage, &pool.Throttler, pool.Inbox)

	pool.EmitTaskUpdate("create", task)

	// Send pending messages to task
	pendingMessages, hasPendingMessages := pool.LostAndFound[task.Id]
	if hasPendingMessages {
		for _, pendingMessage := range pendingMessages {
			// TODO - should this go through pool's message loop?
			task.Inbox <- pendingMessage
		}
		delete(pool.LostAndFound, task.Id)
	}
}

func (pool *TaskPool) CreateTaskMsg(msg APICreateTaskRequest) {
	// Make sure group exists
	_, groupFound := pool.Groups[msg.Task.GroupId]
	if !groupFound {
		msg.Resp <- APICreateTaskResponse{
			Error: &APIError{
				Code:    404,
				Message: fmt.Sprintf("Task group with id %v not found.", msg.Task.GroupId),
			},
		}
		return
	}

	// Make sure task does not exist already
	_, found := pool.Tasks[msg.Task.Id]
	if found {
		msg.Resp <- APICreateTaskResponse{
			Error: &APIError{
				Code:    409,
				Message: fmt.Sprintf("Task with id %v already exists.", msg.Task.Id),
			},
		}
		return
	}

	pool.CreateTask(msg.Task)
	// Send response
	msg.Resp <- APICreateTaskResponse{
		Task: msg.Task,
	}
}

func (pool *TaskPool) DeleteGroup(msg APIDeleteGroupRequest) {
	// Make sure group exists
	group, found := pool.Groups[msg.Id]
	if !found {
		msg.Resp <- APIDeleteGroupResponse{
			Error: &APIError{
				Code:    404,
				Message: fmt.Sprintf("Task group with id %v not found.", msg.Id),
			},
		}
		return
	}

	// Delete group's tasks
	tasks, hasTasks := pool.TasksByGroup[msg.Id]
	if hasTasks {
		for _, task := range tasks {
			pool.DestroyTask(task)
		}
	}

	// Delete group
	pool.Storage.DeleteTaskGroup(group)
	delete(pool.Groups, msg.Id)

	// Delete group's index
	delete(pool.TasksByGroup, msg.Id)

	// Send response
	msg.Resp <- APIDeleteGroupResponse{}

	pool.EmitTaskGroupUpdate("delete", group)
}

func (pool *TaskPool) DeleteTask(msg APIDeleteTaskRequest) {
	// Make sure group exists
	task, found := pool.Tasks[msg.Id]
	if !found {
		msg.Resp <- APIDeleteTaskResponse{
			Error: &APIError{
				Code:    404,
				Message: fmt.Sprintf("Task with id %v not found.", msg.Id),
			},
		}
		return
	}

	pool.DestroyTask(task)

	msg.Resp <- APIDeleteTaskResponse{}
}

func (pool *TaskPool) ResetGroup(msg APIResetGroupRequest) {
	// Make sure group exists
	_, found := pool.Groups[msg.Id]
	if !found {
		msg.Resp <- APIResetGroupResponse{
			Error: &APIError{
				Code:    404,
				Message: fmt.Sprintf("Task group with id %v not found.", msg.Id),
			},
		}
		return
	}

	// Fist pass to check for seed tasks
	hasSeedTasks := false
	tasks, hasTasks := pool.TasksByGroup[msg.Id]
	if hasTasks {
		for _, task := range tasks {
			if task.IsSeed {
				hasSeedTasks = true
				break
			}
		}
	}

	// Second pass to reset tasks
	if hasTasks {
		for _, task := range tasks {
			if hasSeedTasks && !task.IsSeed {
				pool.DestroyTask(task)
			} else {
				pool.ResetTask(task, msg.RemainingAttempts)
			}
		}
	}

	// Send response
	msg.Resp <- APIResetGroupResponse{}
}

func (pool *TaskPool) RetryGroup(msg APIRetryGroupRequest) {
	// Make sure group exists
	_, found := pool.Groups[msg.Id]
	if !found {
		msg.Resp <- APIRetryGroupResponse{
			Error: &APIError{
				Code:    404,
				Message: fmt.Sprintf("Task group with id %v not found.", msg.Id),
			},
		}
		return
	}

	// Update remaining attempts in all incomplete tasks
	tasks, hasTasks := pool.TasksByGroup[msg.Id]
	if hasTasks {
		for _, task := range tasks {
			if !task.IsComplete {
				task.Inbox <- UpdateTaskMessage{
					ToTaskId: task.Id,
					Update:   map[string]interface{}{"remainingAttempts": msg.RemainingAttempts},
				}
			}
		}
	}

	// Send response
	msg.Resp <- APIRetryGroupResponse{}
}

func (pool *TaskPool) PauseResumeGroup(msg APIPauseResumeGroupRequest) {
	// Make sure group exists
	_, found := pool.Groups[msg.Id]
	if !found {
		msg.Resp <- APIPauseResumeGroupResponse{
			Error: &APIError{
				Code:    404,
				Message: fmt.Sprintf("Task group with id %v not found.", msg.Id),
			},
		}
		return
	}

	// Update pause state in tasks
	tasks, hasTasks := pool.TasksByGroup[msg.Id]
	if hasTasks {
		for _, task := range tasks {
			task.Inbox <- UpdateTaskMessage{
				ToTaskId: task.Id,
				Update:   map[string]interface{}{"isPaused": msg.IsPaused},
			}
		}
	}

	// Send response
	msg.Resp <- APIPauseResumeGroupResponse{}
}

func (pool *TaskPool) ResetTaskMsg(msg APIResetTaskRequest) {
	// Make sure task exists
	task, found := pool.Tasks[msg.Id]
	if !found {
		msg.Resp <- APIResetTaskResponse{
			Error: &APIError{
				Code:    404,
				Message: fmt.Sprintf("Task with id %v not found.", msg.Id),
			},
		}
		return
	}

	pool.ResetTask(task, msg.RemainingAttempts)

	msg.Resp <- APIResetTaskResponse{
		Task: task,
	}
}

func (pool *TaskPool) RetryTask(msg APIRetryTaskRequest) {
	// Make sure task exists
	task, found := pool.Tasks[msg.Id]
	if !found {
		msg.Resp <- APIRetryTaskResponse{
			Error: &APIError{
				Code:    404,
				Message: fmt.Sprintf("Task with id %v not found.", msg.Id),
			},
		}
		return
	}

	task.Inbox <- UpdateTaskMessage{
		ToTaskId: task.Id,
		Update:   map[string]interface{}{"remainingAttempts": msg.RemainingAttempts},
	}

	msg.Resp <- APIRetryTaskResponse{
		Task: task,
	}
}

func (pool *TaskPool) UpdateGroup(msg APIUpdateGroupRequest) {
	// Make sure group exists
	group, found := pool.Groups[msg.Id]
	if !found {
		msg.Resp <- APIUpdateGroupResponse{
			Error: &APIError{
				Code:    404,
				Message: fmt.Sprintf("Task group with id %v not found.", msg.Id),
			},
		}
		return
	}

	// Only editable field is Name
	newName, hasNewName := msg.Update["name"].(string)
	if hasNewName {
		group.Name = newName
	}

	msg.Resp <- APIUpdateGroupResponse{
		Group: group,
	}
}

func (pool *TaskPool) UpdateTask(msg APIUpdateTaskRequest) {
	// Make sure task exists
	task, found := pool.Tasks[msg.Id]
	if !found {
		msg.Resp <- APIUpdateTaskResponse{
			Error: &APIError{
				Code:    404,
				Message: fmt.Sprintf("Task with id %v not found.", msg.Id),
			},
		}
		return
	}

	task.Inbox <- UpdateTaskMessage{
		ToTaskId: task.Id,
		Update:   msg.Update,
		// TODO - send update complete signal channel here
	}

	// TODO - wait for update complete signal

	msg.Resp <- APIUpdateTaskResponse{
		Task: task,
	}
}
