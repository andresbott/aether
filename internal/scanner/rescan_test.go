// internal/scanner/rescan_test.go
package scanner_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/tags"
)

// stubReader serves canned metadata per absolute path so a test can change a
// file's "tags" between two rescans.
type stubReader struct {
	meta map[string]tags.Metadata
}

func (r stubReader) CanRead(absPath string) bool { return scanner.IsAudioFile(absPath) }

func (r stubReader) Read(absPath string) (tags.Metadata, error) {
	m, ok := r.meta[absPath]
	if !ok {
		return tags.Metadata{}, tags.ErrUnsupported
	}
	return m, nil
}

func meta(title, artist, album string) tags.Metadata {
	return tags.Metadata{
		Title:       title,
		Artist:      []string{artist},
		AlbumArtist: []string{artist},
		Album:       album,
		Genre:       []string{"Rock"},
		Year:        2020,
		TrackNumber: 1,
		Duration:    180 * time.Second,
		Bitrate:     320,
	}
}

// The load-bearing invariant: a targeted rescan must never run the scan
// cleanup, which deletes every track whose last_seen_at predates the run.
func TestRescanPathsKeepsUntouchedTracks(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Artist/Album/01.mp3",
		"Artist/Album/02.mp3",
		"Artist/Album/03.mp3",
	})
	lib := seedLibrary(t, st, dir, nil)

	full := scanner.New(scanner.Config{}, st, fakeTagReader{})
	if _, err := full.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	edited := filepath.Join(dir, "Artist/Album/01.mp3")
	rescanner := scanner.New(scanner.Config{}, st, stubReader{meta: map[string]tags.Metadata{
		edited: meta("Edited Title", "Test Artist", "Album"),
	}})
	stats, err := rescanner.RescanPaths(context.Background(), lib.ID, []string{edited})
	if err != nil {
		t.Fatal(err)
	}
	if stats.TracksProcessed != 1 {
		t.Fatalf("expected 1 track processed, got %d", stats.TracksProcessed)
	}

	var count int64
	st.DB().Model(&model.Track{}).Count(&count)
	if count != 3 {
		t.Fatalf("expected the other tracks to survive, got %d rows", count)
	}

	var got model.Track
	if err := st.DB().Where("file_path = ?", edited).First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.Title != "Edited Title" {
		t.Fatalf("expected the rescanned title, got %q", got.Title)
	}
}

func TestRescanPathsIndexesANewFile(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Artist/Album/01.mp3"})
	lib := seedLibrary(t, st, dir, nil)

	abs := filepath.Join(dir, "Artist/Album/01.mp3")
	s := scanner.New(scanner.Config{}, st, stubReader{meta: map[string]tags.Metadata{
		abs: meta("Fresh", "Test Artist", "Album"),
	}})
	if _, err := s.RescanPaths(context.Background(), lib.ID, []string{abs}); err != nil {
		t.Fatal(err)
	}

	var count int64
	st.DB().Model(&model.Track{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected the file to be indexed, got %d rows", count)
	}
}

func TestRescanPathsPrunesOrphanedArtist(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Artist/Album/01.mp3"})
	lib := seedLibrary(t, st, dir, nil)
	abs := filepath.Join(dir, "Artist/Album/01.mp3")

	before := scanner.New(scanner.Config{}, st, stubReader{meta: map[string]tags.Metadata{
		abs: meta("Song", "Old Artist", "Album"),
	}})
	if _, err := before.RescanPaths(context.Background(), lib.ID, []string{abs}); err != nil {
		t.Fatal(err)
	}

	after := scanner.New(scanner.Config{}, st, stubReader{meta: map[string]tags.Metadata{
		abs: meta("Song", "New Artist", "Album"),
	}})
	if _, err := after.RescanPaths(context.Background(), lib.ID, []string{abs}); err != nil {
		t.Fatal(err)
	}

	var stale int64
	st.DB().Model(&model.Artist{}).Where("name = ?", "Old Artist").Count(&stale)
	if stale != 0 {
		t.Fatal("expected the orphaned artist to be pruned")
	}
	var fresh int64
	st.DB().Model(&model.Artist{}).Where("name = ?", "New Artist").Count(&fresh)
	if fresh != 1 {
		t.Fatalf("expected the new artist, got %d rows", fresh)
	}
}

// Admission — not a failed tag read — must be what rejects these paths, so the
// stub serves perfectly good metadata for every one of them: a path that slips
// through the guard would be *successfully* indexed and fail the assertions.
// An admission rejection is also not an error: it is a caller passing something
// the library does not cover, which must not show up as a rescan failure.
func TestRescanPathsSkipsInadmissiblePaths(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Artist/Album/01.mp3",
		"Artist/Album/notes.txt",
		"Artist/Album/skipme.mp3",
		"Artist/Live/01.mp3",
	})
	// "^Live$" is anchored: it matches the *directory* name only, which is how
	// Walk prunes whole subtrees.
	lib := seedLibrary(t, st, dir, []string{"skipme", "^Live$"})

	notes := filepath.Join(dir, "Artist/Album/notes.txt")
	skipme := filepath.Join(dir, "Artist/Album/skipme.mp3")
	gone := filepath.Join(dir, "Artist/Album/gone.mp3")
	outside := "/etc/passwd.mp3"
	inPrunedDir := filepath.Join(dir, "Artist/Live/01.mp3")

	s := scanner.New(scanner.Config{}, st, stubReader{meta: map[string]tags.Metadata{
		notes:       meta("Notes", "Test Artist", "Album"),
		skipme:      meta("Skipped", "Test Artist", "Album"),
		gone:        meta("Gone", "Test Artist", "Album"),
		outside:     meta("Outside", "Test Artist", "Album"),
		inPrunedDir: meta("Live", "Test Artist", "Live"),
	}})
	stats, err := s.RescanPaths(context.Background(), lib.ID, []string{
		notes,       // not audio
		skipme,      // excluded by filename
		gone,        // does not exist
		outside,     // outside the library
		inPrunedDir, // inside a directory the walk prunes
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats.TracksProcessed != 0 {
		t.Fatalf("expected nothing processed, got %d", stats.TracksProcessed)
	}
	if len(stats.Errors) != 0 {
		t.Fatalf("an admission rejection must not be reported as an error: %v", stats.Errors)
	}
	// Skipped-by-design must be countable: callers cannot otherwise tell it
	// apart from "reconcile failed", and the editor's file listing is wider
	// than the scanner's admission rules on purpose.
	if stats.TracksSkipped != 5 {
		t.Fatalf("expected all 5 paths counted as skipped, got %d", stats.TracksSkipped)
	}
	var count int64
	st.DB().Model(&model.Track{}).Count(&count)
	if count != 0 {
		t.Fatalf("expected no rows written, got %d", count)
	}
}

// Admission must reject exactly what Walk prunes: an anchored directory pattern
// removes the whole subtree from a scan, so a rescan that admitted a track
// inside it would index a row the very next scan deletes.
func TestRescanPathsMatchesWalkForAnchoredDirectoryExcludes(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Artist/Album/01.mp3",
		"Artist/Live/01.mp3",
	})
	lib := seedLibrary(t, st, dir, []string{"^Live$"})

	// A full scan is the reference: whatever it indexes is admissible.
	full := scanner.New(scanner.Config{}, st, fakeTagReader{})
	fullStats, err := full.Scan(context.Background(), scanner.ScanOptions{IsFull: true})
	if err != nil {
		t.Fatal(err)
	}
	// A full scan's paths all come from its own walk, which already applied the
	// excludes, so it never skips anything at admission.
	if fullStats.TracksSkipped != 0 {
		t.Fatalf("a full scan must not report skipped paths, got %d", fullStats.TracksSkipped)
	}
	var scanned []string
	st.DB().Model(&model.Track{}).Order("file_path").Pluck("file_path", &scanned)
	want := []string{filepath.Join(dir, "Artist/Album/01.mp3")}
	if len(scanned) != 1 || scanned[0] != want[0] {
		t.Fatalf("scan baseline unexpected: %v", scanned)
	}

	// The rescan is handed both paths; it must index the same set the scan did.
	rescanner := scanner.New(scanner.Config{}, st, fakeTagReader{})
	stats, err := rescanner.RescanPaths(context.Background(), lib.ID, []string{
		filepath.Join(dir, "Artist/Album/01.mp3"),
		filepath.Join(dir, "Artist/Live/01.mp3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats.TracksProcessed != 1 {
		t.Fatalf("expected only the admissible track processed, got %d", stats.TracksProcessed)
	}
	if stats.TracksSkipped != 1 {
		t.Fatalf("expected the pruned-directory track counted as skipped, got %d", stats.TracksSkipped)
	}
	var after []string
	st.DB().Model(&model.Track{}).Order("file_path").Pluck("file_path", &after)
	if len(after) != 1 || after[0] != want[0] {
		t.Fatalf("rescan indexed a track the scan prunes: %v", after)
	}
}

// A rescan must never lower a track's liveness marker. A scheduled scan that
// started *after* the rescan has already stamped last_seen_at with its own,
// later scanStart and will call store.Cleanup with it; if the rescan then wrote
// its own earlier timestamp, that Cleanup would delete a track that is very
// much alive — cascading its playlist memberships, play history and stars.
func TestRescanPathsDoesNotLowerLastSeenAt(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Artist/Album/01.mp3", "Artist/Album/02.mp3"})
	lib := seedLibrary(t, st, dir, nil)

	full := scanner.New(scanner.Config{}, st, fakeTagReader{})
	if _, err := full.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	edited := filepath.Join(dir, "Artist/Album/01.mp3")
	other := filepath.Join(dir, "Artist/Album/02.mp3")

	// A scheduled scan starts later than the rescan below and marks every walked
	// path with its own scanStart.
	laterScanStart := time.Now().Add(time.Hour)
	if err := st.BulkUpdateLastSeen([]string{edited, other}, laterScanStart); err != nil {
		t.Fatal(err)
	}

	// The rescan runs with an earlier scanStart (its own time.Now()).
	rescanner := scanner.New(scanner.Config{}, st, stubReader{meta: map[string]tags.Metadata{
		edited: meta("Edited Title", "Test Artist", "Album"),
	}})
	if _, err := rescanner.RescanPaths(context.Background(), lib.ID, []string{edited}); err != nil {
		t.Fatal(err)
	}

	// The invariant: the later scan's cleanup must not consider the rescanned
	// track stale.
	if err := st.Cleanup(laterScanStart); err != nil {
		t.Fatal(err)
	}

	var got model.Track
	if err := st.DB().Where("file_path = ?", edited).First(&got).Error; err != nil {
		t.Fatalf("the rescanned track was deleted by the concurrent scan's cleanup: %v", err)
	}
	if got.Title != "Edited Title" {
		t.Fatalf("expected the rescanned title, got %q", got.Title)
	}
}

func TestRescanPathsUnknownLibrary(t *testing.T) {
	st := testScanStore(t)
	s := scanner.New(scanner.Config{}, st, fakeTagReader{})
	if _, err := s.RescanPaths(context.Background(), 999, []string{"/x/y.mp3"}); err == nil {
		t.Fatal("expected an error for an unknown library")
	}
}

func TestRescanPathsEmptyListIsANoop(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	lib := seedLibrary(t, st, dir, nil)
	s := scanner.New(scanner.Config{}, st, fakeTagReader{})
	stats, err := s.RescanPaths(context.Background(), lib.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TracksProcessed != 0 {
		t.Fatalf("expected 0 processed, got %d", stats.TracksProcessed)
	}
}
