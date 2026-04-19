package store_test

import (
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

func TestFindOrCreateAlbum(t *testing.T) {
	s := testStore(t)
	var album *model.Album
	err := s.Transaction(func(tx *store.Store) error {
		var txErr error
		album, txErr = tx.FindOrCreateAlbum("Kid A", "radiohead", "")
		return txErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if album.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	var same *model.Album
	err = s.Transaction(func(tx *store.Store) error {
		var txErr error
		same, txErr = tx.FindOrCreateAlbum("Kid A", "radiohead", "")
		return txErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if same.ID != album.ID {
		t.Fatal("expected same album ID")
	}
}

func TestGetAlbum(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	artist := model.Artist{Name: "Radiohead", NameNorm: "radiohead"}
	db.Create(&artist)
	album := model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead", Year: 2000}
	db.Create(&album)
	db.Model(&album).Association("Artists").Replace([]*model.Artist{&artist})
	rock := model.Genre{Name: "Rock"}
	db.Create(&rock)
	db.Model(&album).Association("Genres").Replace([]*model.Genre{&rock})
	track := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3", Title: "Everything", TrackNumber: 1, Duration: 250}
	db.Create(&track)
	found, err := s.GetAlbum(album.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found.Name != "Kid A" {
		t.Fatalf("unexpected: %s", found.Name)
	}
	if len(found.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(found.Tracks))
	}
	if len(found.Artists) != 1 {
		t.Fatalf("expected 1 artist, got %d", len(found.Artists))
	}
}

func TestGetAlbumNotFound(t *testing.T) {
	s := testStore(t)
	_, err := s.GetAlbum(9999)
	if err == nil {
		t.Fatal("expected error for missing album")
	}
}

func TestGetAlbumListAlphabetical(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	db.Create(&model.Album{Name: "Zebra", NameNorm: "zebra", AlbumArtistNorm: "x"})
	db.Create(&model.Album{Name: "Alpha", NameNorm: "alpha", AlbumArtistNorm: "x"})
	albums, err := s.GetAlbumList("alphabeticalByName", 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(albums) != 2 {
		t.Fatalf("expected 2 albums, got %d", len(albums))
	}
	if albums[0].NameNorm != "alpha" {
		t.Fatalf("expected alpha first, got %s", albums[0].NameNorm)
	}
}

func TestGetAlbumListNewest(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	db.Create(&model.Album{Name: "Old", NameNorm: "old", AlbumArtistNorm: "x"})
	db.Create(&model.Album{Name: "New", NameNorm: "new", AlbumArtistNorm: "x"})
	albums, err := s.GetAlbumList("newest", 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(albums) != 2 {
		t.Fatalf("expected 2 albums, got %d", len(albums))
	}
	if albums[0].Name != "New" {
		t.Fatalf("expected 'New' first, got %s", albums[0].Name)
	}
}

func TestSearchAlbums(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	db.Create(&model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead"})
	db.Create(&model.Album{Name: "OK Computer", NameNorm: "ok computer", AlbumArtistNorm: "radiohead"})
	results, err := s.SearchAlbums("kid", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}
