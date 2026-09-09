// app/tasks/scan.go
package tasks

import (
	"context"
	"log/slog"

	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
)

const ScanTaskName = "scan"

type ScanParams struct {
	Full bool `json:"full"`
}

var ScanTaskDef = TaskDef{
	ID:          ScanTaskName,
	Name:        "Library Scan",
	Description: "Scan the music library. Pass full to re-read every track regardless of modification time; otherwise only tracks modified since the last scan are re-read.",
}

func NewScanTaskFn(cfg scanner.Config, s *store.Store, tagReader tags.Reader) func(ctx context.Context, log *slog.Logger, p ScanParams) error {
	sc := scanner.New(cfg, s, tagReader)
	return func(ctx context.Context, log *slog.Logger, p ScanParams) error {
		mode := "incremental"
		if p.Full {
			mode = "full"
		}
		log.Info("starting library scan", slog.String("mode", mode))

		stats, err := sc.Scan(ctx, scanner.ScanOptions{IsFull: p.Full, Log: log})
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
