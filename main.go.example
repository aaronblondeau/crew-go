package main

import (
	"context"
	"crypto/md5"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/crew-go/crew"
	"github.com/labstack/echo/v4"
)

type LoginCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

//go:embed crew-go-ui/dist/spa
var embededFiles embed.FS

func main() {
	cwd, _ := os.Getwd()
	storage := crew.NewJsonFilesystemTaskStorage(cwd + "/main_demo")
	client := crew.NewHttpPostClient()

	taskGroupsOperator, bootstrapError := storage.Bootstrap(true, client)
	if bootstrapError != nil {
		panic(bootstrapError)
	}

	httpServerExitDone := &sync.WaitGroup{}
	httpServerExitDone.Add(1)

	// Get username from env var
	authUsername := os.Getenv("CREW_AUTH_USERNAME")
	if authUsername == "" {
		authUsername = "admin"
	}
	authPassword := os.Getenv("CREW_AUTH_PASSWORD")
	if authPassword == "" {
		authPassword = "crew"
	}
	authToken := os.Getenv("CREW_AUTH_TOKEN")
	if authToken == "" {
		// Generate token from username and password
		authToken = fmt.Sprintf("%x", md5.Sum([]byte(authUsername+authPassword)))
	}

	// Validates each api call's Authorization header
	authMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// For systems requiring no auth, just return next(c)
			// return next(c)

			// Checking a hardcoded token (returned by loginFunc)

			// Look in token param first (used for websockets)
			token := c.Param("token")

			if token == "" {
				// Look in Authorization header (all other requests)
				token = c.Request().Header.Get("Authorization")
			}
			// If token contains Bearer, remove it
			if len(token) > 7 && token[:7] == "Bearer " {
				token = token[7:]
			}

			if token == authToken {
				return next(c)
			}
			return echo.ErrUnauthorized
		}
	}

	// Generates an auth token given an username (email) and password
	loginFunc := func(c echo.Context) error {
		creds := LoginCredentials{}
		inflate_err := json.NewDecoder(c.Request().Body).Decode(&creds)
		if inflate_err != nil {
			return c.String(http.StatusBadRequest, inflate_err.Error())
		}

		// Verify a hardcoded password (and return a hashed token)
		if creds.Username == authUsername && creds.Password == authPassword {
			payload := LoginResponse{
				Token: authToken,
			}
			return c.JSON(http.StatusOK, payload)
		} else {
			return c.String(http.StatusUnauthorized, "Invalid Credentials")
		}
	}

	srv := crew.ServeRestApi(httpServerExitDone, taskGroupsOperator, client, embededFiles, authMiddleware, loginFunc)
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
