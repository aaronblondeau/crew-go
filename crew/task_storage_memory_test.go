package crew

import (
	"testing"
)

func TestCreateTaskGroup(t *testing.T) {
	storage := NewMemoryTaskStorage()
	taskGroup := NewTaskGroup("group1", "group1")

	storage.SaveTaskGroup(taskGroup, true)
	found, err := storage.FindTaskGroup("group1")
	if err != nil {
		t.Fatal(err)
	}
	if found.Id != "group1" {
		t.Fatal("TaskGroup not found")
	}
}

func TestUpdateTaskGroup(t *testing.T) {
	storage := NewMemoryTaskStorage()
	taskGroup := NewTaskGroup("group2", "group2")
	storage.SaveTaskGroup(taskGroup, true)

	taskGroup.Name = "group2 edited"
	storage.SaveTaskGroup(taskGroup, false)

	found, err := storage.FindTaskGroup("group2")
	if err != nil {
		t.Fatal(err)
	}
	if found.Name != "group2 edited" {
		t.Fatalf("TaskGroup update failed, exepected 'group2 edited', got %v", found.Name)
	}
}

func TestDeleteTaskGroup(t *testing.T) {
	storage := NewMemoryTaskStorage()
	taskGroup := NewTaskGroup("group3", "group3")
	storage.SaveTaskGroup(taskGroup, true)

	storage.DeleteTaskGroup("group3")

	_, err := storage.FindTaskGroup("group3")
	if err == nil {
		t.Fatal("TaskGroup not deleted")
	}
}

func TestCreateTask(t *testing.T) {
	storage := NewMemoryTaskStorage()
	task := NewTask()
	task.Id = "task1"
	task.Name = "task1"
	task.Worker = "worker-a"
	storage.SaveTask(task, true)

	found, err := storage.FindTask("task1")
	if err != nil {
		t.Fatal(err)
	}
	if found.Id != "task1" {
		t.Fatal("TaskGroup not found")
	}
}

func TestUpdateTask(t *testing.T) {
	storage := NewMemoryTaskStorage()
	task := NewTask()
	task.Id = "task2"
	task.Name = "task2"
	task.Worker = "worker-a"
	storage.SaveTask(task, true)

	task.Name = "task2 edited"
	storage.SaveTask(task, false)

	found, err := storage.FindTask("task2")
	if err != nil {
		t.Fatal(err)
	}
	if found.Name != "task2 edited" {
		t.Fatalf("Task update failed, exepected 'task2 edited', got %v", found.Name)
	}
}

func TestDeleteTask(t *testing.T) {
	storage := NewMemoryTaskStorage()
	task := NewTask()
	task.Id = "task3"
	task.Name = "task3"
	task.Worker = "worker-a"
	storage.SaveTask(task, true)

	storage.DeleteTask("task3")

	_, err := storage.FindTask("task3")
	if err == nil {
		t.Fatal("Task not deleted")
	}
}

func TestFindTasksByWorkgroup(t *testing.T) {
	storage := NewMemoryTaskStorage()
	task4 := NewTask()
	task4.Id = "task4"
	task4.Name = "task4"
	task4.Worker = "worker-a"
	task4.Workgroup = "group-a"
	storage.SaveTask(task4, true)

	task5 := NewTask()
	task5.Id = "task5"
	task5.Name = "task5"
	task5.Worker = "worker-a"
	task5.Workgroup = "group-a"
	storage.SaveTask(task5, true)

	found, _ := storage.GetTasksInWorkgroup("group-a")
	if len(found) != 2 {
		t.Fatalf("Expected 2 tasks, got %v", len(found))
	}

	storage.DeleteTask("task4")

	found, _ = storage.GetTasksInWorkgroup("group-a")
	if len(found) != 1 {
		t.Fatalf("Expected 1 task, got %v", len(found))
	}
}

func TestFindTasksWithKey(t *testing.T) {
	storage := NewMemoryTaskStorage()
	task6 := NewTask()
	task6.Id = "task6"
	task6.Name = "task6"
	task6.Worker = "worker-a"
	task6.Workgroup = "group-a"
	task6.Key = "key-a"
	storage.SaveTask(task6, true)

	task7 := NewTask()
	task7.Id = "task7"
	task7.Name = "task7"
	task7.Worker = "worker-a"
	task7.Workgroup = "group-a"
	task7.Key = "key-a"
	storage.SaveTask(task7, true)

	found, _ := storage.GetTasksWithKey("key-a")
	if len(found) != 2 {
		t.Fatalf("Expected 2 tasks, got %v", len(found))
	}

	storage.DeleteTask("task6")

	found, _ = storage.GetTasksWithKey("key-a")
	if len(found) != 1 {
		t.Fatalf("Expected 1 task, got %v", len(found))
	}
}

func TestGetTaskChildrenAndParents(t *testing.T) {
	storage := NewMemoryTaskStorage()
	task8 := NewTask()
	task8.Id = "task8"
	task8.Name = "task8"
	task8.Worker = "worker-a"
	storage.SaveTask(task8, true)

	task9 := NewTask()
	task9.Id = "task9"
	task9.Name = "task9"
	task9.Worker = "worker-a"
	task9.ParentIds = []string{"task8"}
	storage.SaveTask(task9, true)

	task10 := NewTask()
	task10.Id = "task10"
	task10.Name = "task10"
	task10.Worker = "worker-a"
	task10.ParentIds = []string{"task8"}
	storage.SaveTask(task10, true)

	task8Children, _ := storage.GetTaskChildren("task8")
	if len(task8Children) != 2 {
		t.Fatalf("Expected 2 children, got %v", len(task8Children))
	}
	if task8Children[0].Id != "task9" {
		t.Fatalf("Expected task9, got %v", task8Children[0].Id)
	}

	task9Parents, _ := storage.GetTaskParents("task9")
	if len(task9Parents) != 1 {
		t.Fatalf("Expected 1 parent, got %v", len(task9Parents))
	}
	if task9Parents[0].Id != "task8" {
		t.Fatalf("Expected task8, got %v", task9Parents[0].Id)
	}
}
