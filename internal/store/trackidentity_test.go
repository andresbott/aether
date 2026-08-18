package store_test

import (
	"testing"
	"time"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

// seedIdentityTrack inserts a track row with the columns the move proof reads.
func seedIdentityTrack(t *testing.T, s *store.Store, albumID uint, path string, size int64, mod time.Time, title string) model.Track {
	t.Helper()
	track := model.Track{
		AlbumID:     albumID,
		LibraryID:   1,
		Filename:    "old.mp3",
		FilePath:    path,
		FileSize:    size,
		FileModTime: mod,
		Duration:    180,
		Title:       title,
	}
	if err := s.DB().Create(&track).Error; err != nil {
		t.Fatal(err)
	}
	return track
}

func TestKnownTrackPaths(t *testing.T) {
	s := testStore(t)
	album, _, _ := createTestAlbumAndArtist(t, s)
	mod := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	seedIdentityTrack(t, s, album.ID, "/music/a.mp3", 4, mod, "A")

	known, err := s.KnownTrackPaths([]string{"/music/a.mp3", "/music/b.mp3"})
	if err != nil {
		t.Fatal(err)
	}
	if !known["/music/a.mp3"] {
		t.Error("expected the indexed path to be reported as known")
	}
	if known["/music/b.mp3"] {
		t.Error("an unindexed path must not be reported as known: it is a move candidate")
	}
}

func TestTracksByFileSizes(t *testing.T) {
	s := testStore(t)
	album, _, _ := createTestAlbumAndArtist(t, s)
	mod := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	want := seedIdentityTrack(t, s, album.ID, "/music/a.mp3", 4, mod, "A")
	seedIdentityTrack(t, s, album.ID, "/music/b.mp3", 99, mod, "B")

	rows, err := s.TracksByFileSizes([]int64{4})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row of size 4, got %d: %+v", len(rows), rows)
	}
	got := rows[0]
	if got.ID != want.ID || got.FilePath != "/music/a.mp3" || got.Title != "A" ||
		got.Duration != 180 || got.FileSize != 4 || got.LibraryID != 1 {
		t.Fatalf("row does not carry the columns the proof needs: %+v", got)
	}
	if got.FileModTime.Unix() != mod.Unix() {
		t.Fatalf("FileModTime = %v, want %v (the tiebreak compares whole seconds)", got.FileModTime, mod)
	}
}

func TestRelinkTrackKeepsTheRow(t *testing.T) {
	s := testStore(t)
	album, _, _ := createTestAlbumAndArtist(t, s)
	mod := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	track := seedIdentityTrack(t, s, album.ID, "/music/old/a.mp3", 4, mod, "A")

	relinked, err := s.RelinkTrack(track.ID, "/music/old/a.mp3", "/music/new/b.mp3", 7)
	if err != nil {
		t.Fatal(err)
	}
	if !relinked {
		t.Fatal("expected the re-link to apply")
	}

	var after model.Track
	if err := s.DB().First(&after, track.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.FilePath != "/music/new/b.mp3" {
		t.Fatalf("FilePath = %q, want the new path", after.FilePath)
	}
	if after.Filename != "b.mp3" {
		t.Fatalf("Filename = %q, want the new basename", after.Filename)
	}
	if after.LibraryID != 7 {
		t.Fatalf("LibraryID = %d, want 7 (a move can cross libraries)", after.LibraryID)
	}
	if !after.CreatedAt.Equal(track.CreatedAt) {
		t.Fatalf("created_at changed (%v -> %v)", track.CreatedAt, after.CreatedAt)
	}
}

// The caller proves the move with filesystem reads it cannot hold a transaction
// across, so the update is the check: a row whose path changed underneath must
// be skipped, not overwritten.
func TestRelinkTrackReportsNoChangeWhenThePathMoved(t *testing.T) {
	s := testStore(t)
	album, _, _ := createTestAlbumAndArtist(t, s)
	mod := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	track := seedIdentityTrack(t, s, album.ID, "/music/current.mp3", 4, mod, "A")

	relinked, err := s.RelinkTrack(track.ID, "/music/stale.mp3", "/music/new.mp3", 1)
	if err != nil {
		t.Fatal(err)
	}
	if relinked {
		t.Fatal("expected no change: the row no longer holds the path the proof was built on")
	}

	var after model.Track
	if err := s.DB().First(&after, track.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.FilePath != "/music/current.mp3" {
		t.Fatalf("FilePath = %q, want it untouched", after.FilePath)
	}
}
