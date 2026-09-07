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
const ScanFullTaskName = "scan-full"

var ScanTaskDef = TaskDef{
	ID:          ScanTaskName,
	Name:        "Library Scan",
	Description: "Incremental scan -- only re-reads tracks modified since last scan",
}

var ScanFullTaskDef = TaskDef{
	ID:          ScanFullTaskName,
	Name:        "Full Library Scan",
	Description: "Full scan -- re-reads all tracks regardless of modification time",
}

func NewScanTaskFn(cfg scanner.Config, s *store.Store, tagReader tags.Reader, isFull bool) func(ctx context.Context, log *slog.Logger) error {
	sc := scanner.New(cfg, s, tagReader)
	return func(ctx context.Context, log *slog.Logger) error {
		mode := "incremental"
		if isFull {
			mode = "full"
		}
		log.Info("starting library scan", slog.String("mode", mode))

		stats, err := sc.Scan(ctx, scanner.ScanOptions{IsFull: isFull, Log: log})
		if err != nil {
			log.Error("scan failed", slog.String("error", err.Error()))
			return err
		}

		log.Info("scan complete",
			slog.Int("processed", stats.TracksProcessed),
			slog.Int("new", stats.TracksNew),
			slog.Int("updated", stats.TracksUpdated))

		if len(stats.Errors) > 0 {
			log.Info("scan had tag reading errors", slog.Int("count", len(stats.Errors)))
		}
		return nil
	}
}
