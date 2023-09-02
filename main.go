package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/aaronblondeau/crew-go/crew"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
)

func main() {
	godotenv.Load(".env")

	fmt.Println("")
	fmt.Println("   __________  _______       __")
	fmt.Println("  / ____/ __ \\/ ____/ |     / /")
	fmt.Println(" / /   / /_/ / __/  | | /| / / ")
	fmt.Println("/ /___/ _, _/ /___  | |/ |/ /  ")
	fmt.Println("\\____/_/ |_/_____/  |__/|__/   ")
	fmt.Println("")

	storage := crew.NewMemoryTaskStorage()

	client := crew.NewHttpPostClient()

	throttlePush := make(chan crew.ThrottlePushQuery, 8)
	throttlePop := make(chan crew.ThrottlePopQuery, 8)
	throttler := &crew.Throttler{
		Push: throttlePush,
		Pop:  throttlePop,
	}

	// No throttling
	go func() {
		for {
			select {
			case pushQuery := <-throttlePush:
				// Default behavior = immediate response => no throttling
				fmt.Println("~~ Would throttle", pushQuery.Worker, pushQuery.TaskId)
				pushQuery.Resp <- true
			case popQuery := <-throttlePop:
				fmt.Println("~~ Would unthrottle", popQuery.Worker, popQuery.TaskId)
			}
		}
	}()

	// Create the task controller (call to startup is further down)
	controller := crew.NewTaskController(storage, client, throttler)

	httpServerExitDone := &sync.WaitGroup{}
	httpServerExitDone.Add(1)

	// Validates each api call's Authorization header
	authMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// For systems requiring no auth, just return next(c)
			return next(c)
		}
	}

	// Create the rest api server
	srv, e := crew.ServeRestApi(httpServerExitDone, controller, authMiddleware, nil)

	// Example adding a new worker to the rest api
	e.POST("/demo/worker-d", func(c echo.Context) error {
		log.Println("Demo worker D has been called!")
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
				"message": "Worker D was here!",
				"at":      time.Now().Format(time.RFC3339),
			},
		})
	})

	// Controller startup is performed after rest api is launched
	// This is in case we switch TaskController.TriggerEvaluate to happen via an http call in scaled environments.
	startupError := controller.Startup()
	if startupError != nil {
		panic(startupError)
	}

	// Hook into the shutdown signal
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-done
		// sigint caught, start graceful shutdown
		log.Print("Process Terminating...")
		controller.Shutdown()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}()
	httpServerExitDone.Wait()

	log.Print("Crew Stopped")
}
