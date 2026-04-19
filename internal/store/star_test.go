package store_test

import (
	"testing"

	"github.com/andresbott/aether/internal/model"
)

func TestStarAndUnstar(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	artist := model.Artist{Name: "A", NameNorm: "a"}
	db.Create(&artist)
	if err := s.Star("artist", artist.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Star("artist", artist.ID); err != nil {
		t.Fatal(err)
	} // idempotent
	starred, err := s.GetStarred()
	if err != nil {
		t.Fatal(err)
	}
	if len(starred.Artists) != 1 {
		t.Fatalf("expected 1 starred artist, got %d", len(starred.Artists))
	}
	if err := s.Unstar("artist", artist.ID); err != nil {
		t.Fatal(err)
	}
	starred, _ = s.GetStarred()
	if len(starred.Artists) != 0 {
		t.Fatal("expected 0 starred artists after unstar")
	}
}

func TestGetStarredAll(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	artist := model.Artist{Name: "A", NameNorm: "a"}
	db.Create(&artist)
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "a"}
	db.Create(&album)
	track := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3"}
	db.Create(&track)
	s.Star("artist", artist.ID)
	s.Star("album", album.ID)
	s.Star("track", track.ID)
	starred, err := s.GetStarred()
	if err != nil {
		t.Fatal(err)
	}
	if len(starred.Artists) != 1 || len(starred.Albums) != 1 || len(starred.Tracks) != 1 {
		t.Fatalf("unexpected: artists=%d albums=%d tracks=%d", len(starred.Artists), len(starred.Albums), len(starred.Tracks))
	}
}
