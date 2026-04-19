package store_test

import (
	"testing"
	"time"

	"github.com/andresbott/aether/internal/model"
)

func TestBulkUpdateLastSeen(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	t1 := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/music/01.mp3"}
	t2 := model.Track{AlbumID: album.ID, Filename: "02.mp3", FilePath: "/music/02.mp3"}
	db.Create(&t1)
	db.Create(&t2)
	now := time.Now()
	if err := s.BulkUpdateLastSeen([]string{"/music/01.mp3", "/music/02.mp3"}, now); err != nil {
		t.Fatal(err)
	}
	var track model.Track
	db.First(&track, t1.ID)
	if track.LastSeenAt.Before(now.Add(-time.Second)) {
		t.Fatal("expected LastSeenAt to be updated")
	}
}

func TestFilterChanged(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	track := model.Track{
		AlbumID: album.ID, Filename: "01.mp3", FilePath: "/music/01.mp3",
		FileModTime: old,
	}
	db.Create(&track)
	modMap, err := s.FilterChanged([]string{"/music/01.mp3", "/music/new.mp3"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := modMap["/music/01.mp3"]; !ok {
		t.Fatal("expected existing track in map")
	}
	if _, ok := modMap["/music/new.mp3"]; ok {
		t.Fatal("new file should not be in map")
	}
}

func TestCleanup(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	artist := model.Artist{Name: "A", NameNorm: "a"}
	db.Create(&artist)
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "a"}
	db.Create(&album)
	db.Model(&album).Association("Artists").Replace([]*model.Artist{&artist})
	now := time.Now()
	old := now.Add(-time.Hour)
	t1 := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3", LastSeenAt: now}
	t2 := model.Track{AlbumID: album.ID, Filename: "02.mp3", FilePath: "/02.mp3", LastSeenAt: old}
	db.Create(&t1)
	db.Create(&t2)
	db.Model(&t1).Association("Artists").Replace([]*model.Artist{&artist})
	db.Model(&t2).Association("Artists").Replace([]*model.Artist{&artist})
	if err := s.Cleanup(now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&model.Track{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 track after cleanup, got %d", count)
	}
}
