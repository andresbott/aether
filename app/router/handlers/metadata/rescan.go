package metadata

import "context"

// Reindexer enqueues a background re-index of the files a metadata write just
// touched and returns the job's execution id, which the client polls to know
// when the library index has caught up. Satisfied by the router's
// reindexEnqueuer (over the task runner). It exists so the editor makes an edit
// durable without re-indexing inside the request. Shared by the tag and picture
// handlers (the identify handler never writes, so it has no reindexer).
type Reindexer interface {
	EnqueueReindex(ctx context.Context, scanFolder string, absPaths []string) (executionID string, err error)
}

// reindexRef is the write handlers' pointer to the enqueued re-index job. The
// SPA polls the execution to know when the music-UI index reflects the edit.
type reindexRef struct {
	ExecutionID string `json:"execution_id"`
}

// enqueueReindex enqueues a re-index of absPaths, returning nil when
// re-indexing is disabled (reindexer nil) or there is nothing to do (empty
// list), in which case the response carries no "reindex" field. A failure to
// enqueue is logged by the reindexer (the router's adapter over the task
// runner) — this handler has no logger of its own. Either way the response
// just omits "reindex": the file write already landed, and the next
// scheduled scan reconciles it, so callers never fail the write on it.
func enqueueReindex(ctx context.Context, reindexer Reindexer, scanFolder string, absPaths []string) *reindexRef {
	if reindexer == nil || len(absPaths) == 0 {
		return nil
	}
	id, err := reindexer.EnqueueReindex(ctx, scanFolder, absPaths)
	if err != nil || id == "" {
		return nil
	}
	return &reindexRef{ExecutionID: id}
}
