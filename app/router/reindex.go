package router

import (
	"context"
	"log/slog"

	metadataHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	apptasks "github.com/andresbott/aether/app/tasks"
	"github.com/andresbott/aether/internal/taskrunner"
)

// reindexEnqueuer adapts the task runner to the metadata handlers' Reindexer
// interface: it enqueues a typed reindex job and returns its execution id.
type reindexEnqueuer struct {
	runner *taskrunner.Runner
	// logger records a dropped enqueue (see EnqueueReindex); nil is safe,
	// it just leaves that failure unlogged.
	logger *slog.Logger
}

var _ metadataHandler.Reindexer = reindexEnqueuer{}

// metadataReindexer builds the metadata handlers' Reindexer over h's task
// runner; nil when no task runner is configured, which disables re-indexing
// but leaves the file writes themselves working.
func (h *MainAppHandler) metadataReindexer() metadataHandler.Reindexer {
	if h.taskRunner == nil {
		return nil
	}
	return reindexEnqueuer{runner: h.taskRunner, logger: h.logger}
}

func (e reindexEnqueuer) EnqueueReindex(_ context.Context, libraryID uint, absPaths []string) (string, error) {
	id, _, err := taskrunner.Enqueue[apptasks.ReindexParams](
		e.runner, apptasks.ReindexTaskName,
		apptasks.ReindexParams{LibraryID: libraryID, Paths: absPaths},
	)
	if err != nil {
		// The caller (enqueueReindex) only drops the "reindex" field from the
		// response and lets the next scheduled scan reconcile the edit — log
		// here so a dropped enqueue (e.g. a queue-full burst behind a long
		// scan) still leaves an operator-visible trace.
		if e.logger != nil {
			e.logger.Warn("reindex enqueue failed; edit will be reconciled by the next scan",
				"library", libraryID, "paths", len(absPaths), "error", err)
		}
		return "", err
	}
	return id.String(), nil
}
