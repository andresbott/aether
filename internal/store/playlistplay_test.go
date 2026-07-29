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
