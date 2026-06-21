package store_test

import (
	"testing"
	"time"

	"github.com/andresbott/aether/internal/model"
)

func TestRecordPlay(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	track := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3"}
	db.Create(&track)
	if err := s.RecordPlay(track.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&model.PlayHistory{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 play record, got %d", count)
	}
}

func TestGetNowPlaying(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	t1 := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3", Title: "Recent"}
	t2 := model.Track{AlbumID: album.ID, Filename: "02.mp3", FilePath: "/02.mp3", Title: "Old"}
	db.Create(&t1)
	db.Create(&t2)
	_ = s.RecordPlay(t1.ID, time.Now())
	_ = s.RecordPlay(t2.ID, time.Now().Add(-10*time.Minute))
	entries, err := s.GetNowPlaying()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 now playing, got %d", len(entries))
	}
	if entries[0].Title != "Recent" {
		t.Fatalf("expected 'Recent', got %s", entries[0].Title)
	}
}
