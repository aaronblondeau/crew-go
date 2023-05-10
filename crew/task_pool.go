package crew

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// NOTE - task pool must perform throttling by monitoring ExecuteMessage and ExecutedMessage

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
			case UpdateMessage:
				// Send to recipient
				pool.Tasks[msg.(UpdateMessage).ToTaskId].Inbox <- msg

				// TODO - if update includes group, key, or workgroup, update indexes

			case ExecutedMessage:
				// TODO - process workgroup delays, process child delays, send to throttler
			case ExecuteMessage:
				// Send to recipient
				pool.Tasks[msg.(ExecuteMessage).ToTaskId].Inbox <- msg
			case ParentCompletedMessage:
				// Send to recipient
				pool.Tasks[msg.(ParentCompletedMessage).ToTaskId].Inbox <- msg
			case DeleteMessage:
				// TODO Send to recpient
				// TODO Update indexes
			case ContinuationMessage:
				// TODO Add task to pool
				// TODO Deliver lost and found messages to task
				// TODO Update index
				// TODO Start operating on task
			case ChildIntroductionMessage:
				// Send to recipient
				pool.Tasks[msg.(ChildIntroductionMessage).ToTaskId].Inbox <- msg
			case ChildIntroductionAcknowledgementMessage:
				// Send to recipient
				pool.Tasks[msg.(ChildIntroductionAcknowledgementMessage).ToTaskId].Inbox <- msg
			case TaskUpdatedMessage:
				// TODO Send to websocket for UI
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
				pool.CreateTask(msg.(APICreateTaskRequest))
			default:
				// TODO - error
			}
		}
	}()

	// Once message queue is running, start boostrapped tasks
	for _, task := range pool.Tasks {
		task.Start(pool.Client, pool.Storage, pool.Inbox)
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
	// Close all task inboxes
	for _, task := range pool.Tasks {
		close(task.Inbox)
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
	_, found := pool.Groups[msg.Group.Id]
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
}

func (pool *TaskPool) CreateTask(msg APICreateTaskRequest) {
	// Ensure task has an id
	if msg.Task.Id == "" {
		msg.Task.Id = uuid.New().String()
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

	// Add task to pool
	pool.Tasks[msg.Task.Id] = msg.Task

	// Update indexes
	pool.AddTaskToIndexes(msg.Task)

	// Start task
	msg.Task.Start(pool.Client, pool.Storage, pool.Inbox)

	// Send response
	msg.Resp <- APICreateTaskResponse{
		Task: msg.Task,
	}

	// Send pending messages to task
	pendingMessages, hasPendingMessages := pool.LostAndFound[msg.Task.Id]
	if hasPendingMessages {
		for _, pendingMessage := range pendingMessages {
			msg.Task.Inbox <- pendingMessage
		}
		delete(pool.LostAndFound, msg.Task.Id)
	}
}
