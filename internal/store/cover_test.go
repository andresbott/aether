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

func TestSetAlbumCoverPath(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead"}
	db.Create(&album)

	if err := s.SetAlbumCoverPath(album.ID, "/music/kida/cover.jpg"); err != nil {
		t.Fatalf("SetAlbumCoverPath: %v", err)
	}
	got, err := s.GetAlbum(album.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CoverPath != "/music/kida/cover.jpg" {
		t.Fatalf("want cover path set, got %q", got.CoverPath)
	}
}

func TestSetTrackHasEmbeddedCover(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead"}
	db.Create(&album)
	track := model.Track{AlbumID: album.ID, Filename: "01.flac", FilePath: "/music/kida/01.flac"}
	db.Create(&track)

	if err := s.SetTrackHasEmbeddedCover("/music/kida/01.flac", true); err != nil {
		t.Fatalf("SetTrackHasEmbeddedCover: %v", err)
	}

	var gotTrack model.Track
	db.First(&gotTrack, track.ID)
	if !gotTrack.HasEmbeddedCover {
		t.Fatal("track flag not set")
	}
	var gotAlbum model.Album
	db.First(&gotAlbum, album.ID)
	if !gotAlbum.HasEmbeddedCover {
		t.Fatal("album flag not set")
	}

	if err := s.SetTrackHasEmbeddedCover("/nope.flac", true); err == nil {
		t.Fatal("expected error for unknown path")
	}
}
