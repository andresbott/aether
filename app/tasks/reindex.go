// app/tasks/reindex.go
package tasks

import (
	"context"
	"log/slog"

	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
)

// LibraryWriteExclusionGroup serializes every task that writes to the library
// index over SQLite's single write lock. Both the scan and reindex tasks join
// it, so an edit-triggered reindex never runs while a full/incremental scan is
// in progress (and vice versa). See app/cmd/server.go registrations.
const LibraryWriteExclusionGroup = "library-writes"

const ReindexTaskName = "reindex"

// ReindexParams carries the targeted re-index of the files a metadata edit just
// wrote: the library they live in and their absolute paths. Enqueued by the
// /api/v0 metadata write handlers after the file write lands on disk.
type ReindexParams struct {
	LibraryID uint     `json:"library_id"`
	Paths     []string `json:"paths"`
}

var ReindexTaskDef = TaskDef{
	ID:          ReindexTaskName,
	Name:        "Metadata Re-index",
	Description: "Re-read and reconcile specific files after a metadata-editor write, without scanning the whole library. Enqueued automatically by the editor; not meant to be scheduled.",
}

// NewReindexTaskFn builds the reindex task body. It reuses one Scanner (as
// NewScanTaskFn does) and calls RescanPaths, which re-reads the given files'
// tags, reconciles them, and prunes only the aggregates the edit could have
// emptied — never the whole-library Cleanup.
func NewReindexTaskFn(cfg scanner.Config, s *store.Store, tagReader tags.Reader) func(ctx context.Context, log *slog.Logger, p ReindexParams) error {
	sc := scanner.New(cfg, s, tagReader)
	return func(ctx context.Context, log *slog.Logger, p ReindexParams) error {
		log.Info("starting metadata re-index",
			slog.Uint64("library", uint64(p.LibraryID)),
			slog.Int("paths", len(p.Paths)))

		stats, err := sc.RescanPaths(ctx, p.LibraryID, p.Paths)
		if err != nil {
			log.Error("re-index failed", slog.String("error", err.Error()))
			return err
		}

		log.Info("re-index complete",
			slog.Int("processed", stats.TracksProcessed),
			slog.Int("new", stats.TracksNew),
			slog.Int("updated", stats.TracksUpdated),
			slog.Int("skipped", stats.TracksSkipped),
			slog.Int("failed", stats.TracksFailed))
		if len(stats.Errors) > 0 {
			// Per-file tag-read failures are logged, not fatal (mirrors the scan task).
			log.Info("re-index had tag reading errors", slog.Int("count", len(stats.Errors)))
		}
		// Tracks that could not be saved even after a retry are a real shortfall,
		// surfaced the same way the scheduled scan surfaces them — this path
		// shares reconcile, so it shares the visibility.
		if stats.TracksFailed > 0 {
			log.Warn("re-index could not save some tracks; they were skipped and left unindexed",
				slog.Int("failed", stats.TracksFailed))
		}
		return nil
	}
}
