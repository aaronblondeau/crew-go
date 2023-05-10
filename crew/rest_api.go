package crew

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/net/websocket"
)

func getFileSystem(useOS bool, embededFiles embed.FS) http.FileSystem {
	if useOS {
		log.Print("~~ using live mode for static files")
		return http.FS(os.DirFS("crew-go-ui/dist/spa"))
	}

	log.Print("~~ using embed mode for static files")
	fsys, err := fs.Sub(embededFiles, "crew-go-ui/dist/spa")
	if err != nil {
		panic(err)
	}

	return http.FS(fsys)
}

// ServeRestApi starts the REST API server.
// wg: A waitgroup that the server can use to signal when it is done.
// taskGroupController: The root task group controller to use to manage all tasks and task groups.
// taskClient: The client to use to execute tasks.
// authMiddleware: The echo middleware function that will be used to authenticate API calls.
// loginFunc: The function that will be used to handle login requests.
func ServeRestApi(wg *sync.WaitGroup, pool *TaskPool, taskClient TaskClient, embededFiles embed.FS, authMiddleware echo.MiddlewareFunc, loginFunc func(c echo.Context) error) *http.Server {
	e := echo.New()
	e.Use(middleware.CORS())

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
		pageSize := 20
		if c.QueryParams().Has("pageSize") {
			qPageSize, err := strconv.Atoi(c.QueryParam("pageSize"))
			if err == nil {
				pageSize = qPageSize
			}
		}
		search := ""
		if c.QueryParams().Has("search") {
			search = c.QueryParam("search")
		}

		msg := APIGetGroupsRequest{
			Search:   search,
			Page:     page,
			PageSize: pageSize,
			Resp:     make(chan APIGetGroupsResponse),
		}
		pool.Inbox <- msg

		resp := <-msg.Resp
		if resp.Error != nil {
			return c.String(resp.Error.Code, resp.Error.Message)
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"taskGroups": resp.Groups,
			"count":      resp.Total,
		})
	}, authMiddleware)
	e.GET("/api/v1/task_group/:id", func(c echo.Context) error {
		taskGroupId := c.Param("id")
		msg := APIGetGroupRequest{
			Id:   taskGroupId,
			Resp: make(chan APIGetGroupResponse),
		}
		pool.Inbox <- msg
		resp := <-msg.Resp

		if resp.Error != nil {
			return c.String(resp.Error.Code, resp.Error.Message)
		}
		return c.JSON(http.StatusOK, resp.Group)
	}, authMiddleware)
	e.GET("/api/v1/task_group/:task_group_id/tasks", func(c echo.Context) error {
		page := 1
		if c.QueryParams().Has("page") {
			qpage, err := strconv.Atoi(c.QueryParam("page"))
			if err == nil {
				page = qpage
			}
		}
		pageSize := 20
		if c.QueryParams().Has("pageSize") {
			qPageSize, err := strconv.Atoi(c.QueryParam("pageSize"))
			if err == nil {
				pageSize = qPageSize
			}
		}

		taskGroupId := c.Param("task_group_id")

		search := ""
		if c.QueryParams().Has("search") {
			search = c.QueryParam("search")
		}

		msg := APIGetTasksRequest{
			Search:   search,
			Page:     page,
			PageSize: pageSize,
			GroupId:  taskGroupId,
			Resp:     make(chan APIGetTasksResponse),
		}
		pool.Inbox <- msg

		resp := <-msg.Resp

		if resp.Error != nil {
			return c.String(resp.Error.Code, resp.Error.Message)
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"tasks": resp.Tasks,
			"count": resp.Total,
		})
	}, authMiddleware)
	e.GET("/api/v1/task_group/:task_group_id/progress", func(c echo.Context) error {
		taskGroupId := c.Param("task_group_id")

		msg := APIGetGroupProgressRequest{
			GroupId: taskGroupId,
			Resp:    make(chan APIGetGroupProgressResponse),
		}
		resp := <-msg.Resp

		if resp.Error != nil {
			return c.String(resp.Error.Code, resp.Error.Message)
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"completedPercent": resp.CompletedPercent,
		})
	}, authMiddleware)
	e.GET("/api/v1/task_group/:task_group_id/task/:task_id", func(c echo.Context) error {
		taskId := c.Param("task_id")
		msg := APIGetTaskRequest{
			Id:   taskId,
			Resp: make(chan APIGetTaskResponse),
		}
		pool.Inbox <- msg
		resp := <-msg.Resp

		if resp.Error != nil {
			return c.String(resp.Error.Code, resp.Error.Message)
		}
		return c.JSON(http.StatusOK, resp.Task)
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

		msg := APICreateGroupRequest{
			Group: group,
			Resp:  make(chan APICreateGroupResponse),
		}
		pool.Inbox <- msg
		resp := <-msg.Resp

		if resp.Error != nil {
			return c.String(resp.Error.Code, resp.Error.Message)
		}

		return c.JSON(http.StatusOK, resp.Group)
	}, authMiddleware)
	e.POST("/api/v1/task_group/:task_group_id/tasks", func(c echo.Context) error {
		// Create a task
		task := NewTask()
		inflate_err := json.NewDecoder(c.Request().Body).Decode(&task)
		if inflate_err != nil {
			return c.String(http.StatusBadRequest, inflate_err.Error())
		}

		msg := APICreateTaskRequest{
			Task: task,
			Resp: make(chan APICreateTaskResponse),
		}
		pool.Inbox <- msg
		resp := <-msg.Resp

		if resp.Error != nil {
			return c.String(resp.Error.Code, resp.Error.Message)
		}

		return c.JSON(http.StatusOK, msg.Task)
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
		group.OperatorsMutex.RLock()
		operator, taskFound := group.TaskOperators[taskId]
		group.OperatorsMutex.RUnlock()
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
		group.OperatorsMutex.RLock()
		operator, taskFound := group.TaskOperators[taskId]
		group.OperatorsMutex.RUnlock()
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
		group.OperatorsMutex.RLock()
		operator, taskFound := group.TaskOperators[taskId]
		group.OperatorsMutex.RUnlock()
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
							group.OperatorsMutex.RLock()
							_, parentFound := group.TaskOperators[parentId]
							group.OperatorsMutex.RUnlock()
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

	// Non-embed, serve UI
	// e.Static("/", "crew-go-ui/dist/spa")

	// https://echo.labstack.com/cookbook/embed-resources/
	useOS := len(os.Args) > 1 && os.Args[1] == "live"
	assetHandler := http.FileServer(getFileSystem(useOS, embededFiles))
	e.GET("/*", echo.WrapHandler(assetHandler))

	// For troublehsooting: http://localhost:8090/static/icons/favicon-128x128.png
	// e.GET("/static/*", echo.WrapHandler(http.StripPrefix("/static/", assetHandler)))

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

	log.Println("Server started at " + host + ":" + port)

	return srv
}

// TaskGroupWatcher is used to collect events from the task group controller and deliver them to a websocket.
type TaskGroupWatcher struct {
	TaskGroupId string
	Channel     chan string
	RequestId   string
	Socket      *websocket.Conn
}
