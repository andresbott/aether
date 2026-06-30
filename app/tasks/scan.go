// app/tasks/scan.go
package tasks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
	"github.com/go-bumbu/tempo"
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

func NewScanTaskFn(cfg scanner.Config, s *store.Store, tagReader tags.Reader, logger *slog.Logger, isFull bool) func(ctx context.Context) error {
	sc := scanner.New(cfg, s, tagReader)
	return func(ctx context.Context) error {
		mode := "incremental"
		if isFull {
			mode = "full"
		}
		tempo.Info(ctx, fmt.Sprintf("starting %s library scan", mode))

		stats, err := sc.Scan(ctx, scanner.ScanOptions{IsFull: isFull})
		if err != nil {
			tempo.Error(ctx, fmt.Sprintf("scan failed: %v", err))
			return err
		}

		tempo.Info(ctx, fmt.Sprintf("scan complete: %d processed (%d new, %d updated)",
			stats.TracksProcessed, stats.TracksNew, stats.TracksUpdated))

		if len(stats.Errors) > 0 {
			tempo.Info(ctx, fmt.Sprintf("scan had %d tag reading errors", len(stats.Errors)))
		}
		return nil
	}
}
