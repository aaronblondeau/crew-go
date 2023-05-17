package crew

import (
	"testing"
)

func TestCanExecute(t *testing.T) {
	task := NewTask()
	task.Id = "task18"
	task.Name = "task18"
	task.Worker = "worker-a"
	parents := make([]*Task, 0)

	canExecute := task.CanExecute(parents)
	if !canExecute {
		t.Fatalf(`CanExecute() = false, want true`)
	}
}

func TestCannotExecuteIfTaskIsPaused(t *testing.T) {
	task := NewTask()
	task.Id = "task19"
	task.Name = "task19"
	task.Worker = "worker-a"
	task.IsPaused = true
	parents := make([]*Task, 0)

	canExecute := task.CanExecute(parents)
	if canExecute {
		t.Fatalf(`CanExecute() = true, want false (task is paused)`)
	}
}

func TestCannotExecuteIfParentsIncomplete(t *testing.T) {
	task := NewTask()
	task.Id = "task20"
	task.Name = "task20"
	task.Worker = "worker-a"
	task.IsComplete = false
	task.Input = map[string]int{"in": 42}
	task.Output = map[string]int{"foo": 1, "bar": 2}

	child := NewTask()
	child.Id = "task21"
	child.Name = "task21"
	child.Worker = "worker-a"
	child.ParentIds = []string{"task20"}

	parents := make([]*Task, 0)
	parents = append(parents, task)

	canExecute := child.CanExecute(parents)
	if canExecute {
		t.Fatalf(`CanExecute() = true, want false (parent not complete)`)
	}
}

func TestCanExecuteIfParentsComplete(t *testing.T) {
	task := NewTask()
	task.Id = "task22"
	task.Name = "task22"
	task.Worker = "worker-a"
	task.IsComplete = true
	task.Input = map[string]int{"in": 42}
	task.Output = map[string]int{"foo": 1, "bar": 2}

	child := NewTask()
	child.Id = "task23"
	child.Name = "task23"
	child.Worker = "worker-a"
	child.ParentIds = []string{"task22"}

	parents := make([]*Task, 0)
	parents = append(parents, task)

	canExecute := child.CanExecute(parents)
	if !canExecute {
		t.Fatalf(`CanExecute() = false, want true (parents are complete)`)
	}
}
