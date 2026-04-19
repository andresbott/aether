// internal/scanner/scanner_test.go
package scanner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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

func TestScannerFullScan(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()

	createTestFiles(t, dir, []string{
		"Test Artist/Album One/01-track.mp3",
		"Test Artist/Album One/02-track.mp3",
		"Test Artist/Album Two/01-track.flac",
	})

	cfg := scanner.Config{
		MusicPaths: []scanner.MusicPath{{Path: dir}},
	}
	s := scanner.New(cfg, st, fakeTagReader{})

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

	var albumCount int64
	st.DB().Model(&model.Album{}).Count(&albumCount)
	if albumCount != 2 {
		t.Fatalf("expected 2 albums, got %d", albumCount)
	}

	var artistCount int64
	st.DB().Model(&model.Artist{}).Count(&artistCount)
	if artistCount != 1 {
		t.Fatalf("expected 1 artist, got %d", artistCount)
	}

	var genreCount int64
	st.DB().Model(&model.Genre{}).Count(&genreCount)
	if genreCount != 1 {
		t.Fatalf("expected 1 genre, got %d", genreCount)
	}
}

func TestScannerCleanupOrphans(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()

	createTestFiles(t, dir, []string{
		"Artist/Album/01.mp3",
		"Artist/Album/02.mp3",
	})

	cfg := scanner.Config{
		MusicPaths: []scanner.MusicPath{{Path: dir}},
	}
	s := scanner.New(cfg, st, fakeTagReader{})

	_, _ = s.Scan(context.Background(), scanner.ScanOptions{IsFull: true})

	// Remove one file
	_ = os.Remove(filepath.Join(dir, "Artist/Album/02.mp3"))

	// Rescan
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

	createTestFiles(t, dir, []string{
		"Artist/Album/01.mp3",
	})

	cfg := scanner.Config{
		MusicPaths: []scanner.MusicPath{{Path: dir}},
	}
	s := scanner.New(cfg, st, fakeTagReader{})

	stats1, _ := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true})
	if stats1.TracksProcessed != 1 {
		t.Fatalf("first scan: expected 1 processed, got %d", stats1.TracksProcessed)
	}

	stats2, _ := s.Scan(context.Background(), scanner.ScanOptions{IsFull: false})
	if stats2.TracksProcessed != 0 {
		t.Fatalf("incremental scan: expected 0 processed (unchanged), got %d", stats2.TracksProcessed)
	}
}
