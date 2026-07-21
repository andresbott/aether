// internal/scanner/scanner_test.go
package scanner_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func testScanStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return store.New(db)
}

func seedLibrary(t *testing.T, s *store.Store, path string, excludes []string) *model.Library {
	t.Helper()
	excludeJSON := ""
	if len(excludes) > 0 {
		b, _ := json.Marshal(excludes)
		excludeJSON = string(b)
	}
	lib := &model.Library{
		Name:            filepath.Base(path) + "-lib",
		Path:            path,
		FollowSymlinks:  true,
		ExcludePatterns: excludeJSON,
	}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	return lib
}

type fakeTagReader struct{}

func (fakeTagReader) CanRead(absPath string) bool { return scanner.IsAudioFile(absPath) }
func (fakeTagReader) Read(absPath string) (tags.Metadata, error) {
	name := filepath.Base(absPath)
	dir := filepath.Base(filepath.Dir(absPath))
	return tags.Metadata{
		Title:       name,
		Artist:      []string{"Test Artist"},
		AlbumArtist: []string{"Test Artist"},
		Album:       dir,
		Genre:       []string{"Rock"},
		Year:        2020,
		TrackNumber: 1,
		Duration:    180,
		Bitrate:     320,
	}, nil
}

func TestScannerEmptyLibraries(t *testing.T) {
	st := testScanStore(t)
	s := scanner.New(scanner.Config{}, st, fakeTagReader{})
	stats, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true})
	if err != nil {
		t.Fatal(err)
	}
	if stats.TracksProcessed != 0 {
		t.Fatalf("expected 0 tracks processed, got %d", stats.TracksProcessed)
	}
}

func TestScannerFullScan(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Test Artist/Album One/01-track.mp3",
		"Test Artist/Album One/02-track.mp3",
		"Test Artist/Album Two/01-track.flac",
	})
	lib := seedLibrary(t, st, dir, nil)

	s := scanner.New(scanner.Config{}, st, fakeTagReader{})
	stats, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true})
	if err != nil {
		t.Fatal(err)
	}
	if stats.TracksProcessed != 3 {
		t.Fatalf("expected 3 tracks processed, got %d", stats.TracksProcessed)
	}

	var trackCount int64
	st.DB().Model(&model.Track{}).Count(&trackCount)
	if trackCount != 3 {
		t.Fatalf("expected 3 tracks in DB, got %d", trackCount)
	}

	// Every track should carry the library ID.
	var withLib int64
	st.DB().Model(&model.Track{}).Where("library_id = ?", lib.ID).Count(&withLib)
	if withLib != 3 {
		t.Fatalf("expected 3 tracks attached to library, got %d", withLib)
	}
}

func TestScannerCleanupOrphans(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Artist/Album/01.mp3",
		"Artist/Album/02.mp3",
	})
	seedLibrary(t, st, dir, nil)

	s := scanner.New(scanner.Config{}, st, fakeTagReader{})
	_, _ = s.Scan(context.Background(), scanner.ScanOptions{IsFull: true})

	_ = os.Remove(filepath.Join(dir, "Artist/Album/02.mp3"))
	_, _ = s.Scan(context.Background(), scanner.ScanOptions{IsFull: true})

	var trackCount int64
	st.DB().Model(&model.Track{}).Count(&trackCount)
	if trackCount != 1 {
		t.Fatalf("expected 1 track after cleanup, got %d", trackCount)
	}
}

func TestScannerIncrementalSkipsUnchanged(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Artist/Album/01.mp3"})
	seedLibrary(t, st, dir, nil)

	s := scanner.New(scanner.Config{}, st, fakeTagReader{})
	stats1, _ := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true})
	if stats1.TracksProcessed != 1 {
		t.Fatalf("first scan: expected 1 processed, got %d", stats1.TracksProcessed)
	}
	stats2, _ := s.Scan(context.Background(), scanner.ScanOptions{IsFull: false})
	if stats2.TracksProcessed != 0 {
		t.Fatalf("incremental scan: expected 0 processed, got %d", stats2.TracksProcessed)
	}
}

func TestScannerMultipleLibraries(t *testing.T) {
	st := testScanStore(t)
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	createTestFiles(t, dir1, []string{"A/A/01.mp3"})
	createTestFiles(t, dir2, []string{"B/B/01.flac"})
	libA := seedLibrary(t, st, dir1, nil)
	libB := seedLibrary(t, st, dir2, nil)

	s := scanner.New(scanner.Config{}, st, fakeTagReader{})
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var aCount, bCount int64
	st.DB().Model(&model.Track{}).Where("library_id = ?", libA.ID).Count(&aCount)
	st.DB().Model(&model.Track{}).Where("library_id = ?", libB.ID).Count(&bCount)
	if aCount != 1 || bCount != 1 {
		t.Fatalf("expected one track per library, got A=%d B=%d", aCount, bCount)
	}
}

type multiTagReader struct{}

func (multiTagReader) CanRead(absPath string) bool { return scanner.IsAudioFile(absPath) }
func (multiTagReader) Read(absPath string) (tags.Metadata, error) {
	return tags.Metadata{
		Title:       filepath.Base(absPath),
		Artist:      []string{"Artist A", "Artist B"},
		AlbumArtist: []string{"Artist A"},
		Album:       "Album",
		Genre:       []string{"Rock", "Jazz"},
		Duration:    180 * time.Second,
	}, nil
}

func TestScannerKeepsAllTagValues(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Album/01.mp3"})
	seedLibrary(t, st, dir, nil)

	s := scanner.New(scanner.Config{}, st, multiTagReader{})
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var artistCount int64
	st.DB().Model(&model.Artist{}).Count(&artistCount)
	if artistCount != 2 {
		t.Fatalf("expected 2 artists (multi-value kept as-is), got %d", artistCount)
	}
	var genreCount int64
	st.DB().Model(&model.Genre{}).Count(&genreCount)
	if genreCount != 2 {
		t.Fatalf("expected 2 genres, got %d", genreCount)
	}
}
