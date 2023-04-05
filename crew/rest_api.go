package crew

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/net/websocket"
)

// ServeRestApi starts the REST API server.
// wg: A waitgroup that the server can use to signal when it is done.
// taskGroupController: The root task group controller to use to manage all tasks and task groups.
// taskClient: The client to use to execute tasks.
// authMiddleware: The echo middleware function that will be used to authenticate API calls.
// loginFunc: The function that will be used to handle login requests.
func ServeRestApi(wg *sync.WaitGroup, taskGroupController *TaskGroupController, taskClient TaskClient, authMiddleware echo.MiddlewareFunc, loginFunc func(c echo.Context) error) *http.Server {
	e := echo.New()
	e.Use(middleware.CORS())
	e.Static("/", "crew-go-ui/dist/spa")
	e.GET("/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "Healthy!")
	})
	e.POST("/login", loginFunc)
	e.GET("/authcheck", func(c echo.Context) error {
		return c.String(http.StatusOK, "Authenticated!")
	}, authMiddleware)
	e.GET("/api/v1/task_groups", func(c echo.Context) error {
		page := 1
		if c.QueryParams().Has("page") {
			qpage, err := strconv.Atoi(c.QueryParam("page"))
			if err == nil {
				page = qpage
			}
		}
		page_size := 20
		if c.QueryParams().Has("pageSize") {
			qpage_size, err := strconv.Atoi(c.QueryParam("pageSize"))
			if err == nil {
				page_size = qpage_size
			}
		}
		search := ""
		if c.QueryParams().Has("search") {
			search = c.QueryParam("search")
		}

		// create an all groups slice (while performing search)
		groups := make([]*TaskGroup, 0)
		for _, group := range taskGroupController.TaskGroups {
			if search != "" {
				if strings.Contains(strings.ToLower(group.Name), strings.ToLower(search)) {
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

		if page_size == 0 {
			page_size = len(groups)
		}

		// pagninate groups slice
		slice_start := (page - 1) * page_size
		slice_end := slice_start + page_size
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

		return c.JSON(http.StatusOK, map[string]interface{}{
			"taskGroups": sliced,
			"count":      slice_count,
		})
	}, authMiddleware)
	e.GET("/api/v1/task_group/:id", func(c echo.Context) error {
		taskGroupId := c.Param("id")
		group, found := taskGroupController.TaskGroups[taskGroupId]
		if !found {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}
		return c.JSON(http.StatusOK, group)
	}, authMiddleware)
	e.GET("/api/v1/task_group/:task_group_id/tasks", func(c echo.Context) error {
		page := 1
		if c.QueryParams().Has("page") {
			qpage, err := strconv.Atoi(c.QueryParam("page"))
			if err == nil {
				page = qpage
			}
		}
		page_size := 20
		if c.QueryParams().Has("pageSize") {
			qpage_size, err := strconv.Atoi(c.QueryParam("pageSize"))
			if err == nil {
				page_size = qpage_size
			}
		}

		taskGroupId := c.Param("task_group_id")
		group, found := taskGroupController.TaskGroups[taskGroupId]
		if !found {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}

		search := ""
		if c.QueryParams().Has("search") {
			search = c.QueryParam("search")
		}

		// create an all tasks slice
		tasks := make([]*Task, 0)
		for _, operator := range group.TaskOperators {
			if search != "" {
				if strings.Contains(strings.ToLower(operator.Task.Name), strings.ToLower(search)) {
					tasks = append(tasks, operator.Task)
				}
			} else {
				tasks = append(tasks, operator.Task)
			}
		}

		// sort all tasks slice
		sort.Slice(tasks, func(a, b int) bool {
			return tasks[a].CreatedAt.Before(tasks[b].CreatedAt)
		})

		if page_size == 0 {
			page_size = len(tasks)
		}

		// pagninate tasks slice
		slice_start := (page - 1) * page_size
		slice_end := slice_start + page_size
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

		return c.JSON(http.StatusOK, map[string]interface{}{
			"tasks": sliced,
			"count": slice_count,
		})
	}, authMiddleware)
	e.GET("/api/v1/task_group/:task_group_id/progress", func(c echo.Context) error {
		taskGroupId := c.Param("task_group_id")
		group, found := taskGroupController.TaskGroups[taskGroupId]
		if !found {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}

		total := len(group.TaskOperators)
		completed := 0

		// Iterate all tasks
		for _, operator := range group.TaskOperators {
			if operator.Task.IsComplete {
				completed++
			}
		}

		completedPercent := 0.0
		if total > 0 {
			completedPercent = float64(completed) / float64(total)
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"completedPercent": completedPercent,
		})
	}, authMiddleware)
	e.GET("/api/v1/task_group/:task_group_id/task/:task_id", func(c echo.Context) error {
		taskGroupId := c.Param("task_group_id")
		group, groupFound := taskGroupController.TaskGroups[taskGroupId]
		if !groupFound {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}
		taskId := c.Param("task_id")
		operator, taskFound := group.TaskOperators[taskId]
		if !taskFound {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task with id %v not found in group %v.", taskId, taskGroupId))
		}
		return c.JSON(http.StatusOK, operator.Task)
	}, authMiddleware)
	e.POST("/api/v1/task_groups", func(c echo.Context) error {
		// Create a task group
		group := NewTaskGroup("", "", taskGroupController)
		inflate_err := json.NewDecoder(c.Request().Body).Decode(&group)
		if inflate_err != nil {
			return c.String(http.StatusBadRequest, inflate_err.Error())
		}
		if group.Id == "" {
			group.Id = uuid.New().String()
		}
		group_add_err := taskGroupController.AddGroup(group)
		if group_add_err != nil {
			return c.String(http.StatusBadRequest, group_add_err.Error())
		}
		return c.JSON(http.StatusOK, group)
	}, authMiddleware)
	e.POST("/api/v1/task_group/:task_group_id/tasks", func(c echo.Context) error {
		// Create a task
		taskGroupId := c.Param("task_group_id")
		group, found := taskGroupController.TaskGroups[taskGroupId]
		if !found {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}

		task := Task{}
		inflate_err := json.NewDecoder(c.Request().Body).Decode(&task)
		if inflate_err != nil {
			return c.String(http.StatusBadRequest, inflate_err.Error())
		}
		if task.Id == "" {
			task.Id = uuid.New().String()
		}
		if task.RemainingAttempts == 0 {
			task.RemainingAttempts = 5
		}
		task.CreatedAt = time.Now()
		task.TaskGroupId = group.Id

		// If parent ids are present validate that all parents exist
		if len(task.ParentIds) > 0 {
			for _, parentId := range task.ParentIds {
				_, found = group.TaskOperators[parentId]
				if !found {
					return c.String(http.StatusBadRequest, fmt.Sprintf("Parent %s does not exist", parentId))
				}
			}
		}

		err := group.AddTask(&task, taskClient)
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		return c.JSON(http.StatusOK, task)
	}, authMiddleware)
	e.DELETE("/api/v1/task_group/:task_group_id", func(c echo.Context) error {
		// Delete a task group
		taskGroupId := c.Param("task_group_id")
		_, groupFound := taskGroupController.TaskGroups[taskGroupId]
		if !groupFound {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}
		err := taskGroupController.RemoveGroup(taskGroupId)
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"id":      taskGroupId,
			"deleted": true,
		})
	}, authMiddleware)
	e.DELETE("/api/v1/task_group/:task_group_id/task/:task_id", func(c echo.Context) error {
		// Delete a task
		taskGroupId := c.Param("task_group_id")
		group, groupFound := taskGroupController.TaskGroups[taskGroupId]
		if !groupFound {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}

		taskId := c.Param("task_id")
		err := group.DeleteTask(taskId)
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"id":      taskId,
			"deleted": true,
		})
	}, authMiddleware)
	e.POST("/api/v1/task_group/:task_group_id/reset", func(c echo.Context) error {
		// Reset a task group.  If the group has seed tasks, all non-seed tasks are removed.  Then all remaining tasks within the group are reset.
		taskGroupId := c.Param("task_group_id")
		group, groupFound := taskGroupController.TaskGroups[taskGroupId]
		if !groupFound {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}

		remainingAttempts := 5
		body := make(map[string]interface{})
		parseErr := json.NewDecoder(c.Request().Body).Decode(&body)
		if parseErr == nil {
			bodyRemainingAttempts, found := body["remainingAttempts"]
			if found {
				remainingAttempts = int(bodyRemainingAttempts.(float64))
			}
		}

		resetErr := group.Reset(remainingAttempts, nil)
		if resetErr != nil {
			return c.String(http.StatusBadRequest, resetErr.Error())
		}
		return c.JSON(http.StatusOK, group)
	}, authMiddleware)
	e.POST("/api/v1/task_group/:task_group_id/retry", func(c echo.Context) error {
		// Force a retry of all incomplete tasks in a task group by incrementing their remainingAttempts value.
		taskGroupId := c.Param("task_group_id")
		group, groupFound := taskGroupController.TaskGroups[taskGroupId]
		if !groupFound {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}

		remainingAttempts := 5
		body := make(map[string]interface{})
		parseErr := json.NewDecoder(c.Request().Body).Decode(&body)
		if parseErr == nil {
			bodyRemainingAttempts, found := body["remainingAttempts"]
			if found {
				remainingAttempts = int(bodyRemainingAttempts.(float64))
			}
		}

		err := group.RetryAllTasks(remainingAttempts)
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		return c.JSON(http.StatusOK, group)
	}, authMiddleware)
	e.POST("/api/v1/task_group/:task_group_id/pause", func(c echo.Context) error {
		// Pause all tasks in group, fan-in updates?
		taskGroupId := c.Param("task_group_id")
		group, groupFound := taskGroupController.TaskGroups[taskGroupId]
		if !groupFound {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}

		err := group.PauseAllTasks()
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		return c.JSON(http.StatusOK, group)
	}, authMiddleware)
	e.POST("/api/v1/task_group/:task_group_id/resume", func(c echo.Context) error {
		// Resume all tasks in group, fan-in updates?
		taskGroupId := c.Param("task_group_id")
		group, groupFound := taskGroupController.TaskGroups[taskGroupId]
		if !groupFound {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}

		err := group.UnPauseAllTasks()
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		return c.JSON(http.StatusOK, group)
	}, authMiddleware)
	e.POST("/api/v1/task_group/:task_group_id/task/:task_id/reset", func(c echo.Context) error {
		// Reset a task as if it had never been run.  Reject if BusyExecuting.
		taskGroupId := c.Param("task_group_id")
		group, groupFound := taskGroupController.TaskGroups[taskGroupId]
		if !groupFound {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}

		taskId := c.Param("task_id")
		operator, taskFound := group.TaskOperators[taskId]
		if !taskFound {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task with id %v not found in group %v.", taskId, taskGroupId))
		}
		if operator.Task.BusyExecuting {
			return c.String(http.StatusConflict, "Task is busy executing, cannot update.")
		}

		remainingAttempts := 5
		body := make(map[string]interface{})
		parseErr := json.NewDecoder(c.Request().Body).Decode(&body)
		if parseErr == nil {
			bodyRemainingAttempts, found := body["remainingAttempts"]
			if found {
				remainingAttempts = int(bodyRemainingAttempts.(float64))
			}
		}

		updateComplete := make(chan error)
		operator.ResetTask(remainingAttempts, updateComplete)
		updateError := <-updateComplete
		if updateError != nil {
			return c.String(http.StatusBadRequest, updateError.Error())
		}
		return c.JSON(http.StatusOK, operator.Task)
	}, authMiddleware)
	e.POST("/api/v1/task_group/:task_group_id/task/:task_id/retry", func(c echo.Context) error {
		// Force a retry of a task by updating its remainingAttempts value.
		taskGroupId := c.Param("task_group_id")
		group, groupFound := taskGroupController.TaskGroups[taskGroupId]
		if !groupFound {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}

		taskId := c.Param("task_id")
		operator, taskFound := group.TaskOperators[taskId]
		if !taskFound {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task with id %v not found in group %v.", taskId, taskGroupId))
		}
		if operator.Task.BusyExecuting {
			return c.String(http.StatusConflict, "Task is busy executing, cannot update.")
		}

		remainingAttempts := 5
		body := make(map[string]interface{})
		parseErr := json.NewDecoder(c.Request().Body).Decode(&body)
		if parseErr == nil {
			bodyRemainingAttempts, found := body["remainingAttempts"]
			if found {
				remainingAttempts = int(bodyRemainingAttempts.(float64))
			}
		}

		updateComplete := make(chan error)
		operator.ExternalUpdates <- TaskUpdate{
			Update:         map[string]interface{}{"remainingAttempts": remainingAttempts},
			UpdateComplete: updateComplete,
		}
		updateError := <-updateComplete
		if updateError != nil {
			return c.String(http.StatusBadRequest, updateError.Error())
		}

		return c.JSON(http.StatusOK, operator.Task)
	}, authMiddleware)
	e.PUT("/api/v1/task_group/:task_group_id", func(c echo.Context) error {
		// Update a task group
		taskGroupId := c.Param("task_group_id")
		group, groupFound := taskGroupController.TaskGroups[taskGroupId]
		if !groupFound {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}
		update := make(map[string]interface{})
		parseErr := json.NewDecoder(c.Request().Body).Decode(&update)
		if parseErr != nil {
			return c.String(http.StatusBadRequest, parseErr.Error())
		}
		updateErr := taskGroupController.UpdateGroup(group, update)
		if updateErr != nil {
			return c.String(http.StatusBadRequest, updateErr.Error())
		}
		return c.JSON(http.StatusOK, group)
	}, authMiddleware)
	e.PUT("/api/v1/task_group/:task_group_id/task/:task_id", func(c echo.Context) error {
		// Update a task. Do not update and throw error if Task.BusyExecuting!
		taskGroupId := c.Param("task_group_id")
		group, groupFound := taskGroupController.TaskGroups[taskGroupId]
		if !groupFound {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}
		taskId := c.Param("task_id")
		operator, taskFound := group.TaskOperators[taskId]
		if !taskFound {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task with id %v not found in group %v.", taskId, taskGroupId))
		}
		if operator.Task.BusyExecuting {
			return c.String(http.StatusConflict, "Task is busy executing, cannot update.")
		}
		update := make(map[string]interface{})
		parseErr := json.NewDecoder(c.Request().Body).Decode(&update)
		if parseErr != nil {
			return c.String(http.StatusBadRequest, parseErr.Error())
		}

		newRunAfter, hasRunAfter := update["runAfter"]
		if hasRunAfter {
			if newRunAfter == "" {
				update["runAfter"] = nil
			} else {
				// Turn run after into a time.Time
				runAfter, runAfterError := time.Parse("2006-01-02T15:04:05.9999999-07:00", newRunAfter.(string))
				if runAfterError != nil {
					return c.String(http.StatusBadRequest, runAfterError.Error())
				}
				update["runAfter"] = runAfter
			}
		}

		// If parent ids are present validate that all parents exist
		newParentIds, hasParentIds := update["parentIds"]
		if hasParentIds {
			switch t := newParentIds.(type) {
			case []interface{}:
				if len(t) == 0 {
					update["parentIds"] = make([]string, 0)
				} else {
					for _, parentIdCandidate := range t {
						parentId, ok := parentIdCandidate.(string)
						if ok {
							// Make sure parent id exists (and isn't self)
							_, parentFound := group.TaskOperators[parentId]
							if !parentFound {
								return c.String(http.StatusBadRequest, fmt.Sprintf("Parent %s does not exist", parentId))
							}
							if parentId == taskId {
								return c.String(http.StatusBadRequest, "Task cannot be own parent")
							}
						}
					}
				}
			default:
				// Not expected input type for parentIds
			}
		}

		// Task updates happen in operator goroutine, use a channel to sync
		// update before we send response
		updateComplete := make(chan error)
		operator.ExternalUpdates <- TaskUpdate{
			Update:         update,
			UpdateComplete: updateComplete,
		}
		updateError := <-updateComplete
		if updateError != nil {
			return c.String(http.StatusBadRequest, updateError.Error())
		}

		return c.JSON(http.StatusOK, operator.Task)
	}, authMiddleware)

	// Demo worker endpoints
	e.POST("/demo/worker-a", func(c echo.Context) error {
		fmt.Println("~~ Demo worker A has been called!")
		time.Sleep(5 * time.Second)

		payload := map[string]interface{}{}
		json.NewDecoder(c.Request().Body).Decode(&payload)
		throw := ""
		if payload["input"] != nil && payload["input"].(map[string]interface{})["throw"] != nil {
			throw = payload["input"].(map[string]interface{})["throw"].(string)
		}
		if throw != "" {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": throw,
			})
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"output": map[string]interface{}{
				"message": "Worker A was here!",
				"at":      time.Now().Format(time.RFC3339),
			},
		})
	})
	// Worker B is pretty much identical to worker A
	e.POST("/demo/worker-b", func(c echo.Context) error {
		fmt.Println("~~ Demo worker B has been called!")
		time.Sleep(7 * time.Second)

		payload := map[string]interface{}{}
		json.NewDecoder(c.Request().Body).Decode(&payload)
		throw := ""
		if payload["input"] != nil && payload["input"].(map[string]interface{})["throw"] != nil {
			throw = payload["input"].(map[string]interface{})["throw"].(string)
		}
		if throw != "" {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": throw,
			})
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"output": map[string]interface{}{
				"message": "Worker B was here!",
				"at":      time.Now().Format(time.RFC3339),
			},
		})
	})
	// Worker C returns child (continuation) tasks
	e.POST("/demo/worker-c", func(c echo.Context) error {
		fmt.Println("~~ Demo worker C has been called!")
		time.Sleep(2 * time.Second)

		payload := map[string]interface{}{}
		json.NewDecoder(c.Request().Body).Decode(&payload)
		throw := ""
		if payload["input"] != nil && payload["input"].(map[string]interface{})["throw"] != nil {
			throw = payload["input"].(map[string]interface{})["throw"].(string)
		}
		if throw != "" {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": throw,
			})
		}

		// create string of random characters
		aId := fmt.Sprintf("A-%v", rand.Int())
		bId := fmt.Sprintf("B-%v", rand.Int())
		cId := fmt.Sprintf("C-%v", rand.Int())
		dId := fmt.Sprintf("D-%v", rand.Int())

		return c.JSON(http.StatusOK, map[string]interface{}{
			"output": map[string]interface{}{
				"message": "Worker C was here!",
				"at":      time.Now().Format(time.RFC3339),
			},
			"children": []map[string]interface{}{
				{
					"id":     aId,
					"worker": "worker-a",
					"name":   "Child A",
					// Input can go here too!
				},
				{
					"id":        bId,
					"parentIds": [1]string{aId},
					"worker":    "worker-a",
					"name":      "Child B",
				},
				{
					"id":        cId,
					"parentIds": [1]string{aId},
					"worker":    "worker-b",
					"name":      "Child C",
				},
				{
					"id":        dId,
					"parentIds": [2]string{bId, cId},
					"worker":    "worker-a",
					"name":      "Child D",
				},
			},
		})
	})

	inShutdown := false

	// Watch for updates in the task group contoller and deliver them to listening websockets
	watchers := make(map[string]TaskGroupWatcher, 0)
	go func() {
		for taskGroupUpdate := range taskGroupController.TaskGroupUpdates {
			for _, watcher := range watchers {
				if watcher.TaskGroupId == taskGroupUpdate.TaskGroup.Id {
					evtJson, jsonErr := json.Marshal(taskGroupUpdate)
					if jsonErr == nil && !inShutdown {
						watcher.Channel <- string(evtJson)
					}
				}
			}
		}
	}()
	go func() {
		for taskUpdate := range taskGroupController.TaskUpdates {
			for _, watcher := range watchers {
				if watcher.TaskGroupId == taskUpdate.Task.TaskGroupId {
					evtJson, jsonErr := json.Marshal(taskUpdate)
					if jsonErr == nil && !inShutdown {
						watcher.Channel <- string(evtJson)
					}
				}
			}
		}
	}()

	e.GET("/api/v1/task_group/:task_group_id/stream/:token", func(c echo.Context) error {
		taskGroupId := c.Param("task_group_id")
		group, groupFound := taskGroupController.TaskGroups[taskGroupId]
		if !groupFound {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}
		requestId := uuid.New().String()

		websocket.Handler(func(ws *websocket.Conn) {
			defer ws.Close()

			// Create a "watcher" to keep track of this websocket's request to watch a specific task group
			sink := make(chan string)
			watch := TaskGroupWatcher{
				TaskGroupId: group.Id,
				Channel:     sink,
				RequestId:   requestId,
				Socket:      ws,
			}
			watchers[requestId] = watch

			// Listen for messages (or close events) from the client
			go func() {
				for {
					msg := ""
					err := websocket.Message.Receive(ws, &msg)
					if err != nil {
						if err == io.EOF {
							delete(watchers, requestId)
							close(watch.Channel)
							return
						} else {
							c.Logger().Error(err)
						}
						return
					}
					// We don't do anything with messages received from the client
				}
			}()

			for msg := range sink {
				// When we get a message on this socket's output channel, write to the websocket
				err := websocket.Message.Send(ws, msg)
				if err != nil {
					// TODO - close channel, remove from watchers?
					c.Logger().Error(err)
				}
			}
		}).ServeHTTP(c.Response(), c.Request())
		delete(watchers, requestId)
		return nil
	}, authMiddleware)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	host := os.Getenv("HOST")
	if host == "" {
		// use localhost on windows, 0.0.0.0 elsewhere
		if runtime.GOOS == "windows" {
			host = "localhost"
		} else {
			host = "0.0.0.0"
		}
	}

	srv := &http.Server{
		Addr:    host + ":" + port,
		Handler: e,
	}

	go func() {
		defer wg.Done()
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("ServeRestApi(): %v", err)
		}
		inShutdown = true
		// Shutdown all watchers
		for _, watcher := range watchers {
			close(watcher.Channel)
		}
		log.Println("ServeRestApi Stopped")
	}()

	return srv
}

// TaskGroupWatcher is used to collect events from the task group controller and deliver them to a websocket.
type TaskGroupWatcher struct {
	TaskGroupId string
	Channel     chan string
	RequestId   string
	Socket      *websocket.Conn
}
