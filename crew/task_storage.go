package crew

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type TaskStorage interface {
	SaveTask(group *TaskGroup, task *Task) (err error)
	DeleteTask(group *TaskGroup, task *Task) (err error)
	SaveTaskGroup(group *TaskGroup) (err error)
	DeleteTaskGroup(group *TaskGroup) (err error)
	Bootstrap(shouldOperate bool, client TaskClient) (taskGroupController *TaskGroupController, err error)
}

// Filesystem Storage (JSON)

type JsonFilesystemTaskStorage struct {
	BasePath string
}

func (storage *JsonFilesystemTaskStorage) TaskPath(group *TaskGroup, task *Task) string {
	return storage.TaskGroupDir(group) + "/" + task.Id + ".json"
}

func (storage *JsonFilesystemTaskStorage) TaskGroupDir(group *TaskGroup) string {
	return storage.BasePath + "/task_groups/" + group.Id
}

func (storage *JsonFilesystemTaskStorage) TaskGroupPath(group *TaskGroup) string {
	return storage.TaskGroupDir(group) + "/group.json"
}

func (storage *JsonFilesystemTaskStorage) SaveTask(group *TaskGroup, task *Task) (err error) {
	taskJson, jsonErr := json.Marshal(task)
	if jsonErr != nil {
		return jsonErr
	}
	taskBytes := []byte(taskJson)
	writeErr := os.WriteFile(storage.TaskPath(group, task), taskBytes, 0644)
	return writeErr
}

func (storage *JsonFilesystemTaskStorage) DeleteTask(group *TaskGroup, task *Task) (err error) {
	filePath := storage.TaskPath(group, task)
	_, statError := os.Stat(filePath)
	if statError != nil {
		// Stat error => file didn't exist
		return nil
	}
	removeError := os.Remove(filePath)
	return removeError
}

func (storage *JsonFilesystemTaskStorage) SaveTaskGroup(group *TaskGroup) (err error) {
	// Make sure task group dir exists
	groupDir := storage.TaskGroupDir(group)
	if _, err := os.Stat(groupDir); os.IsNotExist(err) {
		os.MkdirAll(groupDir, os.ModeDir)
	}

	groupJson, jsonErr := json.Marshal(group)
	if jsonErr != nil {
		return jsonErr
	}
	groupBytes := []byte(groupJson)
	writeErr := os.WriteFile(storage.TaskGroupPath(group), groupBytes, 0644)
	return writeErr
}

func (storage *JsonFilesystemTaskStorage) DeleteTaskGroup(group *TaskGroup) (err error) {
	// Since we're using os.RemoveAll(), make sure that there is a BasePath set
	if (storage.BasePath == "") || (storage.BasePath == "/") {
		panic("BasePath not set for storage!")
	}
	groupDir := storage.TaskGroupDir(group)
	if (groupDir == "") || (groupDir == "/") {
		panic("Bad group directory - could delete everything!")
	}
	// fmt.Println("~~ Would os.RemoveAll", groupDir)
	// return nil

	removeError := os.RemoveAll(groupDir)
	return removeError
}

func (storage *JsonFilesystemTaskStorage) Bootstrap(shouldOperate bool, client TaskClient) (taskGroupController *TaskGroupController, err error) {
	taskGroupController = NewTaskGroupController()

	entries, readDirError := os.ReadDir(storage.BasePath + "/task_groups")
	if readDirError != nil {
		return taskGroupController, readDirError
	}

	for _, groupEntry := range entries {
		if groupEntry.IsDir() {
			// Look for dir/group.json
			groupDir := storage.BasePath + "/task_groups/" + groupEntry.Name()
			fmt.Println("~~ Bootstrap reading group dir", groupDir)
			groupJsonPath := groupDir + "/group.json"
			_, groupFileExistError := os.Stat(groupJsonPath)
			if !os.IsNotExist(groupFileExistError) {
				// group.json exists => this is a task group directory

				groupData, readGroupErr := os.ReadFile(groupJsonPath)
				if readGroupErr != nil {
					fmt.Println("~~ Skipping group - failed to read group.json", readGroupErr)
					continue
				}

				group := NewTaskGroup("", "", taskGroupController)
				groupParseError := json.Unmarshal(groupData, &group)
				if groupParseError != nil {
					fmt.Println("~~ Skipping group - failed to parse group.json", groupParseError)
					continue
				}
				// Make sure group uses this storage
				group.Storage = storage

				// group ok, look for tasks
				taskGroupTasks := make([]*Task, 0)

				taskEntries, readGroupDirError := os.ReadDir(groupDir)
				if readGroupDirError != nil {
					fmt.Println("~~ Failed to scan for for tasks", readGroupDirError)
					continue
				}

				for _, taskEntry := range taskEntries {
					// Make sure entry is a file that ends with .json
					if !taskEntry.IsDir() && taskEntry.Name() != "group.json" && strings.HasSuffix(taskEntry.Name(), ".json") {
						taskFilePath := groupDir + "/" + taskEntry.Name()
						fmt.Println("~~ Bootstrap reading task file", taskFilePath)
						taskData, taskDataErr := os.ReadFile(taskFilePath)
						if taskDataErr != nil {
							fmt.Println("~~ Skipping task - failed to read .json", taskFilePath, taskDataErr)
							continue
						}

						task := Task{
							// Errors:    make([]interface{}, 0),
							// ParentIds: make([]string, 0),
							// Children:  make([]*Task, 0),
						}
						taskParseError := json.Unmarshal(taskData, &task)
						// Make sure group id matches the group being loaded
						task.TaskGroupId = group.Id
						if taskParseError != nil {
							fmt.Println("~~ Skipping task - failed to parse .json", taskFilePath, taskParseError)
							continue
						}

						taskGroupTasks = append(taskGroupTasks, &task)
					}
				}

				taskGroupController.AddGroup(group)
				group.PreloadTasks(taskGroupTasks, client)
			} else {
				fmt.Println("~~ Cannot find group.json")
			}
		}
	}

	if shouldOperate {
		taskGroupController.Operate()
	}
	return
}

func NewJsonFilesystemTaskStorage(basePath string) *JsonFilesystemTaskStorage {
	storage := JsonFilesystemTaskStorage{
		BasePath: basePath,
	}
	return &storage
}

// Memory Storage

type MemoryTaskStorage struct {
}

func (storage *MemoryTaskStorage) SaveTask(group *TaskGroup, task *Task) (err error) {
	// Do nothing
	return nil
}

func (storage *MemoryTaskStorage) DeleteTask(group *TaskGroup, task *Task) (err error) {
	// Do nothing
	return nil
}

func (storage *MemoryTaskStorage) SaveTaskGroup(group *TaskGroup) (err error) {
	// Do nothing
	return nil
}

func (storage *MemoryTaskStorage) DeleteTaskGroup(group *TaskGroup) (err error) {
	// Do nothing
	return nil
}

func (storage *MemoryTaskStorage) Bootstrap(shouldOperate bool, client TaskClient) (taskGroupController *TaskGroupController, err error) {
	controller := NewTaskGroupController()
	return controller, nil
}

func NewMemoryTaskStorage() *MemoryTaskStorage {
	storage := MemoryTaskStorage{}
	return &storage
}
