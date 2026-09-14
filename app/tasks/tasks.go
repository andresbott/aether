// app/tasks/tasks.go
package tasks

type TaskDef struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// AvailableTasks is the user-facing task catalogue backing listTasks, the
// trigger endpoint and schedules. reindex is deliberately absent: it is enqueued
// by the metadata editor's write handlers, not triggered or scheduled by hand.
// It stays registered on the runner (see server.go) so the editor can enqueue it.
var AvailableTasks = []TaskDef{ScanTaskDef, ScanFullTaskDef, FetchArtistImagesTaskDef, PruneTaskDef}

func TaskNameExists(taskName string) bool {
	for _, t := range AvailableTasks {
		if t.ID == taskName {
			return true
		}
	}
	return false
}
