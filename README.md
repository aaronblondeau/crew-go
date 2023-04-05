// Running tests

cd crew
go test

TODO : add own worker endpoints in main.go?
TODO : update godoc
TODO : reset a task with children (resets all ancestors?)
TODO : redis (KeyDB) persistence : (https://docs.keydb.dev/) https://redis.com/blog/go-redis-official-redis-client/ 
TODO : Figure out good value for TaskUpdates channel queue size
TODO : Implement an (configurably optional) cleanup "cron" for removing old taskgroups
TODO : Polish Readme
TODO : Package for standalone use
TODO : Package for embedded use

Scaling Note:

Crew is designed so that you can scale horizontally by partitioning on taskGroupId.
1) Every API call starts with /task_group/<taskGroupId>
2) You supply your own task and task group ids

TODO - Caddyfile example of reverse proxy to two different crew instances, one handling taskGroups with Ids that start with A-M and another N-Z.

// os.Setenv("CREW_WORKER_BASE_URL", "https://us-central1-dose-board-aaron-dev.cloudfunctions.net/")