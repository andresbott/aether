package store_test

import (
	"testing"
	"time"

	"github.com/andresbott/aether/internal/model"
)

func TestRecordPlaylistPlayAndStats(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	pl1 := model.Playlist{Name: "One"}
	pl2 := model.Playlist{Name: "Two"}
	db.Create(&pl1)
	db.Create(&pl2)

	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	if err := s.RecordPlaylistPlay(pl1.ID, base); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordPlaylistPlay(pl1.ID, base.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordPlaylistPlay(pl2.ID, base); err != nil {
		t.Fatal(err)
	}

	stats, err := s.PlaylistStats([]uint{pl1.ID, pl2.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got := stats[pl1.ID].PlayCount; got != 2 {
		t.Fatalf("pl1 PlayCount = %d, want 2", got)
	}
	if got := stats[pl1.ID].LastPlayed; !got.Equal(base.Add(time.Hour)) {
		t.Fatalf("pl1 LastPlayed = %v, want %v", got, base.Add(time.Hour))
	}
	if got := stats[pl2.ID].PlayCount; got != 1 {
		t.Fatalf("pl2 PlayCount = %d, want 1", got)
	}
}

func TestPlaylistStatsOmitsNeverPlayed(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	pl := model.Playlist{Name: "Never"}
	db.Create(&pl)

	stats, err := s.PlaylistStats([]uint{pl.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := stats[pl.ID]; ok {
		t.Fatal("never-played playlist should be absent from the stats map")
	}
}

func TestPlaylistStatsEmptyInput(t *testing.T) {
	s := testStore(t)
	stats, err := s.PlaylistStats(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(stats))
	}
}

func TestOrphanedPlaylistPlaysAreRemoved(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	pl := model.Playlist{Name: "Gone"}
	db.Create(&pl)
	if err := s.RecordPlaylistPlay(pl.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	db.Delete(&model.Playlist{}, pl.ID)

	if err := s.DeleteOrphanedAggregates(); err != nil {
		t.Fatal(err)
	}
	var n int64
	db.Model(&model.PlaylistPlay{}).Count(&n)
	if n != 0 {
		t.Fatalf("expected orphaned playlist plays removed, %d remain", n)
	}
}

// TestPlaylistStatsFarFutureTimestamp verifies that a far-future scrobble
// timestamp round-trips without error. The datetime() normalization and graceful
// parse fallback must handle any epoch-ms that Go's time package cannot represent.
func TestPlaylistStatsFarFutureTimestamp(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	pl := model.Playlist{Name: "Future"}
	db.Create(&pl)

	// Year 10000, well beyond the range Go's time.Parse normally handles (5-digit year).
	farFuture := time.Date(10000, 1, 1, 1, 0, 0, 0, time.UTC)
	if err := s.RecordPlaylistPlay(pl.ID, farFuture); err != nil {
		t.Fatal(err)
	}

	// PlaylistStats must not error and must return the play count. If the timestamp
	// is unparseable, LastPlayed is left zero rather than failing the whole query.
	stats, err := s.PlaylistStats([]uint{pl.ID})
	if err != nil {
		t.Fatalf("PlaylistStats failed on far-future timestamp: %v", err)
	}
	if got := stats[pl.ID].PlayCount; got != 1 {
		t.Fatalf("PlayCount = %d, want 1", got)
	}
	// We accept zero LastPlayed as graceful degradation.
}
