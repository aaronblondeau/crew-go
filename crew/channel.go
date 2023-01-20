package crew

// A Channel describes what url tasks are POSTed to.
// When a task is being executed, the channel who's id matches
// the task's Channel is found and it the task is submitted
// to the Channel's url.
type Channel struct {
	Id  string `json:"id"`
	Url string `json:"url"`
}
