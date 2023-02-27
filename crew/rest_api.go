package crew

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func ServeRestApi(wg *sync.WaitGroup, taskGroupController *TaskGroupController, taskClient TaskClient) *http.Server {
	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})
	e.GET("/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "Healthy!")
	})
	e.GET("/api/v1/task_groups", func(c echo.Context) error {
		page := 1
		if c.QueryParams().Has("page") {
			qpage, err := strconv.Atoi(c.QueryParam("page"))
			if err == nil {
				page = qpage
			}
		}
		page_size := 20
		if c.QueryParams().Has("page_size") {
			qpage_size, err := strconv.Atoi(c.QueryParam("page_size"))
			if err == nil {
				page_size = qpage_size
			}
		}

		// create an all groups slice
		groups := make([]*TaskGroup, 0)
		for _, group := range taskGroupController.TaskGroups {
			groups = append(groups, group)
		}

		// sort all groups slice
		sort.Slice(groups, func(a, b int) bool {
			return groups[a].CreatedAt.Before(groups[b].CreatedAt)
		})

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
			"task_groups": sliced,
			"count":       slice_count,
		})
	})
	e.GET("/api/v1/task_group/:id", func(c echo.Context) error {
		taskGroupId := c.Param("id")
		group, found := taskGroupController.TaskGroups[taskGroupId]
		if !found {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}
		return c.JSON(http.StatusOK, group)
	})
	e.GET("/api/v1/task_group/:task_group_id/tasks", func(c echo.Context) error {
		page := 1
		if c.QueryParams().Has("page") {
			qpage, err := strconv.Atoi(c.QueryParam("page"))
			if err == nil {
				page = qpage
			}
		}
		page_size := 20
		if c.QueryParams().Has("page_size") {
			qpage_size, err := strconv.Atoi(c.QueryParam("page_size"))
			if err == nil {
				page_size = qpage_size
			}
		}

		taskGroupId := c.Param("task_group_id")
		group, found := taskGroupController.TaskGroups[taskGroupId]
		if !found {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}

		// create an all tasks slice
		tasks := make([]*Task, 0)
		for _, operator := range group.TaskOperators {
			tasks = append(tasks, operator.Task)
		}

		// sort all tasks slice
		sort.Slice(tasks, func(a, b int) bool {
			return tasks[a].CreatedAt.Before(tasks[b].CreatedAt)
		})

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
	})
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
	})
	e.POST("/api/v1/task_groups", func(c echo.Context) error {
		// Create a task group
		group := NewTaskGroup("", "", taskGroupController)
		if group.Id == "" {
			group.Id = uuid.New().String()
		}
		inflate_err := json.NewDecoder(c.Request().Body).Decode(&group)
		if inflate_err != nil {
			return c.String(http.StatusBadRequest, inflate_err.Error())
		}
		group_add_err := taskGroupController.AddGroup(group)
		if group_add_err != nil {
			return c.String(http.StatusBadRequest, group_add_err.Error())
		}
		return c.JSON(http.StatusOK, group)
	})
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
		task.TaskGroupId = group.Id

		err := group.AddTask(&task, taskClient)
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		return c.JSON(http.StatusOK, task)
	})
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
	})
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
	})
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
	})
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
	})
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
	})
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
	})
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
	})
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
	})
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
	})
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
	})

	srv := &http.Server{
		Addr:    "localhost:8090",
		Handler: e,
	}

	go func() {
		defer wg.Done()
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("ServeRestApi(): %v", err)
		}
		log.Println("ServeRestApi Stopped")
	}()

	// TODO emit SSE (or websocket) events from taskGroupController.TaskUpdates

	return srv
}
