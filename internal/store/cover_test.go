package store_test

import (
	"testing"

	"github.com/andresbott/aether/internal/model"
)

func TestGetAlbumByTrackPath(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead"}
	db.Create(&album)
	track := model.Track{AlbumID: album.ID, Filename: "01.flac", FilePath: "/music/kida/01.flac"}
	db.Create(&track)

	got, err := s.GetAlbumByTrackPath("/music/kida/01.flac")
	if err != nil {
		t.Fatalf("GetAlbumByTrackPath: %v", err)
	}
	if got.ID != album.ID {
		t.Fatalf("want album %d, got %d", album.ID, got.ID)
	}

	if _, err := s.GetAlbumByTrackPath("/nope.flac"); err == nil {
		t.Fatal("expected error for unknown path")
	}
}
