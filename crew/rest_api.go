package crew

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"
)

func ServeRestApi(wg *sync.WaitGroup, taskGroupController *TaskGroupController) *http.Server {
	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})
	e.GET("/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "Healthy!")
	})
	e.GET("/task_groups/:id", func(c echo.Context) error {
		taskGroupId := c.Param("id")
		group, found := taskGroupController.TaskGroups[taskGroupId]
		if !found {
			return c.String(http.StatusNotFound, fmt.Sprintf("Task group with id %v not found.", taskGroupId))
		}
		return c.JSON(http.StatusOK, group)
	})
	e.GET("/task_groups/:task_group_id/tasks/:task_id", func(c echo.Context) error {
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
