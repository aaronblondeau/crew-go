package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/crew-go/crew"
)

func main() {
	os.Setenv("CREW_WORKER_BASE_URL", "https://us-central1-dose-board-aaron-dev.cloudfunctions.net/")

	cwd, _ := os.Getwd()
	storage := crew.NewJsonFilesystemTaskStorage(cwd + "/main_demo")
	client := crew.NewHttpPostClient()

	taskGroupsOperator, bootstrapError := storage.Bootstrap(true, client)
	if bootstrapError != nil {
		panic(bootstrapError)
	}

	httpServerExitDone := &sync.WaitGroup{}
	httpServerExitDone.Add(1)
	srv := crew.ServeRestApi(httpServerExitDone, taskGroupsOperator, client)
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-done
		log.Print("Process Terminating...")
		close(taskGroupsOperator.TaskUpdates)
		close(taskGroupsOperator.TaskGroupUpdates)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}()
	httpServerExitDone.Wait()

	log.Print("Crew Stopped")
}
