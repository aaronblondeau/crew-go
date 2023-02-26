TODO : https://goswagger.io/

Note, use this to exclude from JSON `json:"-"` : https://pkg.go.dev/encoding/json#Marshal

// Running tests

cd crew
go test

TODO : update godoc
TODO : ensure task/group ids are url friendly?
TODO : Implement API / Server Sent Events
TODO : redis persistence : https://redis.com/blog/go-redis-official-redis-client/
TODO : Figure out good value for TaskUpdates channel queue size
TODO : Implement an (configurably optional) cleanup "cron" for removing old taskgroups

Scaling Note:

Crew is designed so that:
1) Every API call starts with /task_group/<taskGroupId>
2) You supply your own task and task group ids

This allows you to scale horizontally by partitioning on taskGroupId.
TODO - Caddyfile example of reverse proxy to two different crew instances, one handling taskGroups with Ids that start with A-M and another N-Z.
