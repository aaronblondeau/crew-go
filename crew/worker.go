package crew

// A Worker describes what url tasks are POSTed to.
// When a task is being executed, the worker who's id matches
// the task's WorkerId is found and it the task is submitted
// to the Worker's url.
type Worker struct {
	Id  string `json:"id"`
	Url string `json:"url"`
}
