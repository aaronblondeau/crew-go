TODO : https://goswagger.io/

Note, use this to exclude from JSON `json:"-"` : https://pkg.go.dev/encoding/json#Marshal

// Running tests

cd models
go test

TODO : Tests for task delete
TODO : Tesks for task add
TODO : Tests with TaskClient

TODO : cleanup "cron" for removing old tasks



Scaling Note:

Crew is designed so that:
1) Every API call starts with /task_group/<taskGroupId>
2) You supply your own task and task group ids

This allows you to scale horizontally by partitioning on taskGroupId.
TODO - Caddyfile example of reverse proxy to two different crew instances, one handling taskGroups with Ids that start with A-M and another N-Z.
