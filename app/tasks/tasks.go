// app/tasks/tasks.go
package tasks

type TaskDef struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

var AvailableTasks = []TaskDef{ScanTaskDef, ScanFullTaskDef, FetchArtistImagesTaskDef}

func TaskNameExists(taskName string) bool {
	for _, t := range AvailableTasks {
		if t.ID == taskName {
			return true
		}
	}
	return false
}
