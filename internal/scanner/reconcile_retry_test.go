// internal/scanner/reconcile_retry_test.go
package scanner_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanner"
	"gorm.io/gorm"
)

// failTrackWrites makes the DB reject the INSERT of the track whose Title is
// title for its first failTimes attempts (failTimes < 0 = every attempt),
// simulating a transient write failure (a lost SQLite write lock, a constraint
// hiccup) inside reconcile's per-track transaction. It returns a pointer to the
// number of times that track's insert was attempted, so a test can prove a
// retry happened.
func failTrackWrites(t *testing.T, db *gorm.DB, title string, failTimes int) *int {
	t.Helper()
	attempts := 0
	err := db.Callback().Create().Before("gorm:create").Register("test:fail_"+title, func(tx *gorm.DB) {
		trk, ok := tx.Statement.Dest.(*model.Track)
		if !ok || trk.Title != title {
			return
		}
		attempts++
		if failTimes < 0 || attempts <= failTimes {
			_ = tx.AddError(fmt.Errorf("injected write failure for %q (attempt %d)", title, attempts))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	return &attempts
}

// quietReconcileLogs silences the package-global slog for the duration of a test
// so an expected reconcile-failure warning does not pollute test output.
func quietReconcileLogs(t *testing.T) {
	t.Helper()
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.DiscardHandler))
	t.Cleanup(func() { slog.SetDefault(prev) })
}

// A per-track save that fails once must be retried, and the retry's success must
// be reflected in the track landing in the DB and counted as processed — not
// silently dropped on the first error.
func TestScanRetriesTransientTrackSaveFailure(t *testing.T) {
	quietReconcileLogs(t)
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Artist/Album/01.mp3"})
	seedLibrary(t, st, dir, nil)

	attempts := failTrackWrites(t, st.DB(), "01.mp3", 1) // fail once, succeed on retry

	s := scanner.New(scanner.Config{}, st, fakeTagReader{})
	stats, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true})
	if err != nil {
		t.Fatalf("scan returned error: %v", err)
	}
	if *attempts != 2 {
		t.Fatalf("expected the track insert to be attempted twice (one retry), got %d", *attempts)
	}
	if stats.TracksProcessed != 1 {
		t.Fatalf("expected 1 track processed after retry, got %d", stats.TracksProcessed)
	}
	var count int64
	st.DB().Model(&model.Track{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected the retried track to be persisted, got %d rows", count)
	}
}

// When a per-track save keeps failing after its retry, the scan must not error
// or silently drop the track from every count: the failure is counted in
// TracksFailed while the remaining tracks still process, so the scan can report
// how many tracks it could not save.
func TestScanCountsUnrecoverableTrackSaveFailure(t *testing.T) {
	quietReconcileLogs(t)
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Artist/Album/01.mp3", // saves fine
		"Artist/Album/02.mp3", // save always fails
	})
	seedLibrary(t, st, dir, nil)

	attempts := failTrackWrites(t, st.DB(), "02.mp3", -1) // fail every attempt

	s := scanner.New(scanner.Config{}, st, fakeTagReader{})
	stats, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true})
	if err != nil {
		t.Fatalf("a single unsaveable track must not fail the whole scan: %v", err)
	}
	if *attempts != 2 {
		t.Fatalf("expected the failing track to be attempted twice (one retry), got %d", *attempts)
	}
	if stats.TracksFailed != 1 {
		t.Fatalf("expected 1 track counted as failed, got %d", stats.TracksFailed)
	}
	if stats.TracksProcessed != 1 {
		t.Fatalf("expected the other track to still process, got %d", stats.TracksProcessed)
	}
	var count int64
	st.DB().Model(&model.Track{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected only the saveable track persisted, got %d rows", count)
	}
}
