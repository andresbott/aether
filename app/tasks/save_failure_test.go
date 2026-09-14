// app/tasks/save_failure_test.go
package tasks

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"gorm.io/gorm"
)

// quietGlobalLogs silences the package-global slog for a test's duration, so the
// reconcile-failure warning these tests deliberately trigger (emitted via the
// global logger, not the task's own logger) does not pollute test output.
func quietGlobalLogs(t *testing.T) {
	t.Helper()
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.DiscardHandler))
	t.Cleanup(func() { slog.SetDefault(prev) })
}

// failAllTrackWrites makes every track INSERT on db fail, simulating a save that
// keeps failing even after reconcile's retry — so a task test can prove the
// resulting shortfall is surfaced rather than swallowed.
func failAllTrackWrites(t *testing.T, db *gorm.DB) {
	t.Helper()
	err := db.Callback().Create().Before("gorm:create").Register("test:fail_all_tracks", func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*model.Track); ok {
			_ = tx.AddError(fmt.Errorf("injected track write failure"))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A scheduled scan whose tracks cannot be saved must not report a bare success:
// the job still completes (one bad file does not fail the scan), but the count
// of tracks it could not save is surfaced in the task log.
func TestNewScanTaskFnReportsUnsaveableTracks(t *testing.T) {
	quietGlobalLogs(t)
	cfg, st, reader, libID, _ := setupReindexFixture(t)
	failAllTrackWrites(t, st.DB())

	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	fn := NewScanTaskFn(cfg, st, reader, true)
	if err := fn(context.Background(), log, nil); err != nil {
		t.Fatalf("an unsaveable track must not fail the scan job: %v", err)
	}
	if n := countTracks(t, st, libID); n != 0 {
		t.Fatalf("expected the track to be unsaved, got %d tracks", n)
	}
	if out := buf.String(); !strings.Contains(out, `"failed":1`) {
		t.Fatalf("scan task did not surface the failed-track count; log:\n%s", out)
	}
}

// The editor-triggered reindex shares reconcile with the scheduled scan, so it
// must surface the same shortfall the same way.
func TestNewReindexTaskFnReportsUnsaveableTracks(t *testing.T) {
	quietGlobalLogs(t)
	cfg, st, reader, libID, audioPath := setupReindexFixture(t)
	failAllTrackWrites(t, st.DB())

	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	fn := NewReindexTaskFn(cfg, st, reader)
	if err := fn(context.Background(), log, ReindexParams{LibraryID: libID, Paths: []string{audioPath}}); err != nil {
		t.Fatalf("an unsaveable track must not fail the reindex job: %v", err)
	}
	if n := countTracks(t, st, libID); n != 0 {
		t.Fatalf("expected the track to be unsaved, got %d tracks", n)
	}
	if out := buf.String(); !strings.Contains(out, `"failed":1`) {
		t.Fatalf("reindex task did not surface the failed-track count; log:\n%s", out)
	}
}
