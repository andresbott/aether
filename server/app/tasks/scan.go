// app/tasks/scan.go
package tasks

import (
	"context"
	"log/slog"

	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
	"github.com/andresbott/aether/internal/taskrunner"
)

const (
	ScanTaskName     = "scan"
	ScanFullTaskName = "scan-full"
)

// scan and scan-full are two distinct tasks rather than one task with a `full`
// parameter: the task runner coalesces duplicate triggers by task name alone
// (ignoring params), so a single "scan" task would let a full run fold onto an
// in-flight incremental one and be silently dropped. Splitting the mode into the
// task identity gives each its own coalescing bucket; both share the
// library-writes exclusion group, so they still never scan concurrently.
var ScanTaskDef = TaskDef{
	ID:          ScanTaskName,
	Name:        "Catalog Scan",
	Description: "Scan the configured scan folders incrementally: only tracks modified since the last scan are re-read. Runs coalesce, so triggering it again while one is in flight joins the running scan.",
}

var ScanFullTaskDef = TaskDef{
	ID:          ScanFullTaskName,
	Name:        "Full Catalog Scan",
	Description: "Scan the configured scan folders in full: every track is re-read regardless of modification time, picking up re-derivations an incremental scan would skip. Distinct from the incremental scan so a full run is never dropped in favour of one.",
}

func NewScanTaskFn(cfg scanner.Config, s *store.Store, tagReader tags.Reader, full bool) func(ctx context.Context, log *slog.Logger, prog taskrunner.Progress) error {
	sc := scanner.New(cfg, s, tagReader)
	return func(ctx context.Context, log *slog.Logger, prog taskrunner.Progress) error {
		mode := "incremental"
		if full {
			mode = "full"
		}
		log.Info("starting scan", slog.String("mode", mode))

		stats, err := sc.Scan(ctx, scanner.ScanOptions{IsFull: full, Log: log, Progress: prog})
		if err != nil {
			log.Error("scan failed", slog.String("error", err.Error()))
			return err
		}

		log.Info("scan complete",
			slog.Int("processed", stats.TracksProcessed),
			slog.Int("new", stats.TracksNew),
			slog.Int("updated", stats.TracksUpdated),
			slog.Int("failed", stats.TracksFailed))

		if len(stats.Errors) > 0 {
			log.Info("scan had tag reading errors", slog.Int("count", len(stats.Errors)))
		}
		// A non-zero failed count is a genuine shortfall — tracks the scan meant
		// to index but could not save even after a retry. It does not fail the
		// job (one bad file must not abort a scan), but it must not be silent
		// either, or a scan reports success while quietly indexing fewer tracks.
		if stats.TracksFailed > 0 {
			log.Warn("scan could not save some tracks; they were skipped and left unindexed",
				slog.Int("failed", stats.TracksFailed))
		}
		return nil
	}
}
