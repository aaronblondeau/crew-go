# Crew-Go

Crew is a task management system. It is designed to be used as a library in your own Go project, or as a standalone service. Crew workers are simply webooks so they can be written in any language.

Crew supports the following features:
- Complex parent / child task structure (directed acyclic graph)
- Pause and resume groups of tasks
- Continuations (tasks can create child tasks)
- Delayed, scheduled tasks
- Can manage rate limit errors by automatically pausing all tasks in same workgroup
- Automatically merges output of duplicate tasks
- Workers can be written in any language

## Running from source

To run the service

```
go get
go run main.go
```

Note, to run service while serving UI from local filesystem, use:

```
go run main.go live
```

To run the UI (built with [quasar](https://quasar.dev/)))

```
cd crew-go-ui
yarn
yarn dev
```

To run tests

```
cd crew
go test
```

## Running demo tasks

[![Demo](https://cdn.loom.com/sessions/thumbnails/853e0cf55e514df99a8dc6eeddcc3c00-with-play.gif)](https://www.loom.com/share/853e0cf55e514df99a8dc6eeddcc3c00)

## Persistence

Crew currently uses the local filesystem to store tasks. Redis and other databases are on the roadmap.

## Customizing

You can use the following environment variables to customize the service:
CREW_AUTH_USERNAME: Username for login (defaults to admin)
CREW_AUTH_PASSWORD: Password for login (defaults to crew)
CREW_AUTH_TOKEN: Token for api access (only use this if not using the UI to login)
CREW_WORKER_BASE_URL: Base url for workers (defaults to http://localhost:8080).  Example : https://us-central1-my-project.cloudfunctions.net/
CREW_WORKER_AUTHORIZATION_HEADER: Auth header that crew will send with requests to workers.

Note, when embedding crew in your own Go project you can supply a login function and an authentication middleware to override the default authentication behavior. See main.go for examples.

## Scaling

Crew is designed so that you can scale horizontally by partitioning on taskGroupId.
1) Every API call starts with /task_group/<taskGroupId>
2) You can supply your own task and task group ids

TODO - Caddyfile example of reverse proxy to two different crew instances, one handling taskGroups with Ids that start with A-M and another N-Z.

### About Tree Structure

The tasks in Crew can be composed to form a tree structure.  Each task can have zero or many parents. Tasks can also have zero or many children.  The parentIds field on Tasks is used to form these relationships.  *A task will never be assigned to a worker until all of it's parent tasks have completed successfully.*

Task Groups are used to break large tasks down into many small tasks.  Every task belongs to a group.

### About Task Group Reset / Seed Jobs

Task Groups can be re-set which will allow them to be re-executed.  This should only be used for developing / debugging workers. 

Tasks flagged with isSeed=True, are the only tasks that are retained when a task group is reset.  If a task group doesn't have any seed tasks, all tasks will be retained when the task group is reset.

### About Continuations

A continuation occurs when execution of a task results in additional tasks.  Continuation tasks are always children of the task that created them. See the /demo/worker-c route in rest_api.go for an example of how a worker should return child tasks.

### About Duplication Merge

Crew can automatically complete tasks that are identical.  The primary use case for this feature is when a large volume of nightly tasks fails to complete before the next night's run.  Instead of creating an even larger bottleneck duplicate tasks can be merged instead of repeated.

The "key" field is used for duplication merge.  Whenever Crew assigns a task it will find any other tasks that have the same key in the same task group. Whenever Crew is completing a task it will look for any other tasks that have the same key in the same task group.  The matching tasks will receive the same output or error.

### About Workers

Workers are simply webhooks that are called by Crew.  Workers are responsible for executing the task and returning the a result.  Workers are called with a POST request that contains the task in the body. Workers should respond with a 200 status code if the task was successfully completed.  Workers should respond with an http error code if the task failed. Please see the /demo/worker-* routes in rest_api.go for examples.

Workers should follow these rules:
* Workers take json input and return json output. See below for schemas.
* Workers return a non-200 error code if they fail to complete the task.
* Workers should be designed so that they cause no harm if the same job is repeated.
* Workers should be designed so that they always complete in under 60 seconds, workers that cannot complete in this amount of time should break the work into smaller continuations.

Worker post body schema (json)

```
{
    "input": {"any": "json"},
    "parents": [{
        "taskId": "Id of parent task",
        "worker": "Name of parent worker",
        "input":  {"any": "json given to parent as input"},
        "output": ["any output from parent"],
    }],
    "worker":  "Name of worker",
    "taskId":  "Id of worker",
}
```

Worker response schema (json)

```
{
    "output": {"any": "json"},
    "children": [{
        "id": "Id of child task",
        "parentIds": "Id of parent task (must be another child in this output array)",
        "worker": "Name of child worker",
        "input":  {"any": "json given to child as input"},
    }],
	"workgroupDelayInSeconds": 0,
    "childrenDelayInSeconds": 0,
    "error": "Any error message or json (note that worker response must also be non-200)",
}
```

If workgroupDelayInSeconds is included in response, all tasks in the same workgroup will be paused for the specified amount of time.  This is useful for rate limiting errors.
If childrenDelayInSeconds is included in response, all children will be delayed for the specified amount of time.

### About Workgroups

Crew is designed to help manage rate limit errors via workgroups.  When a rate limit error is encountered all the tasks within a workgroup can be delayed by a specific amount of time by including "workgroupDelayInSeconds" in the response.  Since workgroups will often be organized around a specific API key it is recommended that you use an md5 hash of the API key instead of the key itself when creating workgroup names.

### About Throttling

If you need to restrict how many tasks are concurrently executing for a specific worker you can implement a throttler.  See main.go.example for a simple example.  Below is an example that restricts the total numnber of tasks on a per-worker basis.

Throttler is a simple interface that requires two channels, Push and Pop. Whenever crew is ready to execute a task it sends a message on Push that contains a Resp channel. When your throttler is ready to allow the task to execute, send a true on Resp. As tasks complete (or error) crew will send a message to Pop to notify your throttler that the task is no longer pending.


```go
// Max X concurrent task per worker
defaultMaxConcurrentTasks := 1
maxConcurrentTasksByWorker := map[string]int{
	"worker-a": 3,
	"worker-b": 2,
	"worker-c": 1,
}
go func() {
	executingTasks := make(map[string][]crew.ThrottlePushQuery)
	pendingTasks := make(map[string][]crew.ThrottlePushQuery)
	for {
		select {
		case pushQuery := <-throttlePush:
			// Get count of tasks currently executing for this worker
			executingTasksCount := len(executingTasks[pushQuery.Worker])

			maxConcurrentTasks := defaultMaxConcurrentTasks
			if val, exists := maxConcurrentTasksByWorker[pushQuery.Worker]; exists {
				maxConcurrentTasks = val
			}

			if executingTasksCount < maxConcurrentTasks {
				// Push this query on the executing queue
				queue, exists := executingTasks[pushQuery.Worker]
				if !exists {
					// Create queue if it doesn't exist
					queue = make([]crew.ThrottlePushQuery, 0)
				}
				queue = append(queue, pushQuery)
				executingTasks[pushQuery.Worker] = queue

				// Send message to immediately run this
				pushQuery.Resp <- true
			} else {
				// Push this query on the pending queue
				queue, exists := pendingTasks[pushQuery.Worker]
				if !exists {
					// Create queue if it doesn't exist
					queue = make([]crew.ThrottlePushQuery, 0)
				}
				queue = append(queue, pushQuery)
				pendingTasks[pushQuery.Worker] = queue
			}

		case popQuery := <-throttlePop:
			// Remove this task from the executing queue
			queue := make([]crew.ThrottlePushQuery, 0)
			for _, task := range executingTasks[popQuery.Worker] {
				if task.TaskId != popQuery.TaskId {
					queue = append(queue, task)
				}
			}

			// If there are pending tasks, move the first one to the executing queue
			pendingQueue, pendingExists := pendingTasks[popQuery.Worker]
			if pendingExists && len(pendingQueue) > 0 {
				// Remove from pending queue
				pendingQuery := pendingQueue[0]
				pendingQueue = pendingQueue[1:]
				pendingTasks[popQuery.Worker] = pendingQueue

				// Add to executing queue
				queue = append(queue, pendingQuery)

				// Send message to execute task
				pendingQuery.Resp <- true
			}

			// Store updates to executing
			executingTasks[popQuery.Worker] = queue
		}
	}
}()

taskGroupsOperator, bootstrapError := storage.Bootstrap(true, client, &throttler)
```

### About Persistence

Crew provides two storage mechanisms out of the box: local filesystem or redis.  You can also implement the very simple TaskStorage interface to use your own storage mechanism.

To use redis

```go
storage := crew.NewRedisTaskStorage("localhost:6379", "", 0)
defer storage.Client.Close()
//...
taskGroupsOperator, bootstrapError := storage.Bootstrap(true, client, &throttler)
```

To use filesystem (local JSON files)

```go
storage := crew.NewJsonFilesystemTaskStorage("./storage")
//...
taskGroupsOperator, bootstrapError := storage.Bootstrap(true, client, &throttler)
```

#### Dev Todos

TODO : When a task with children is reset, should all ancestors also be reset?
TODO : verify that workgroup pause works across all task groups in the node (???)
TODO : Make sure duplicate merge (via key) works across all task groups in the node (and update readme above)
TODO : Add expiration to messages in pool.LostAndFound
TODO : Use correct case for all type fields (we have everything public so far)