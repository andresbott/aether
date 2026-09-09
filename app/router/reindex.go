package router

import (
	"context"

	metadataHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	apptasks "github.com/andresbott/aether/app/tasks"
	"github.com/andresbott/aether/internal/taskrunner"
)

// reindexEnqueuer adapts the task runner to the metadata handlers' Reindexer
// interface: it enqueues a typed reindex job and returns its execution id.
type reindexEnqueuer struct {
	runner *taskrunner.Runner
}

var _ metadataHandler.Reindexer = reindexEnqueuer{}

// metadataReindexer builds the metadata handlers' Reindexer over h's task
// runner; nil when no task runner is configured, which disables re-indexing
// but leaves the file writes themselves working.
func (h *MainAppHandler) metadataReindexer() metadataHandler.Reindexer {
	if h.taskRunner == nil {
		return nil
	}
	return reindexEnqueuer{runner: h.taskRunner}
}

func (e reindexEnqueuer) EnqueueReindex(_ context.Context, libraryID uint, absPaths []string) (string, error) {
	id, _, err := taskrunner.Enqueue[apptasks.ReindexParams](
		e.runner, apptasks.ReindexTaskName,
		apptasks.ReindexParams{LibraryID: libraryID, Paths: absPaths},
	)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}
