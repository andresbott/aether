package store_test

import (
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

func TestFindOrCreateArtists(t *testing.T) {
	s := testStore(t)
	var artists []*model.Artist
	err := s.Transaction(func(tx *store.Store) error {
		var txErr error
		artists, txErr = tx.FindOrCreateArtists([]string{"Björk", "Radiohead"})
		return txErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(artists) != 2 {
		t.Fatalf("expected 2 artists, got %d", len(artists))
	}
	// Call again — should find existing
	var again []*model.Artist
	err = s.Transaction(func(tx *store.Store) error {
		var txErr error
		again, txErr = tx.FindOrCreateArtists([]string{"Björk"})
		return txErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if again[0].ID != artists[0].ID {
		t.Fatal("expected same artist ID on second call")
	}
}

func TestGetArtists(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	db.Create(&model.Artist{Name: "Björk", NameNorm: "bjork"})
	db.Create(&model.Artist{Name: "Radiohead", NameNorm: "radiohead"})
	artists, err := s.GetArtists()
	if err != nil {
		t.Fatal(err)
	}
	if len(artists) != 2 {
		t.Fatalf("expected 2 artists, got %d", len(artists))
	}
}

func TestGetArtist(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	artist := model.Artist{Name: "Radiohead", NameNorm: "radiohead"}
	db.Create(&artist)
	album := model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead"}
	db.Create(&album)
	db.Model(&album).Association("Artists").Replace([]*model.Artist{&artist})
	found, albums, err := s.GetArtist(artist.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found.Name != "Radiohead" {
		t.Fatalf("unexpected name: %s", found.Name)
	}
	if len(albums) != 1 {
		t.Fatalf("expected 1 album, got %d", len(albums))
	}
}

func TestGetArtistNotFound(t *testing.T) {
	s := testStore(t)
	_, _, err := s.GetArtist(9999)
	if err == nil {
		t.Fatal("expected error for missing artist")
	}
}

func TestSearchArtists(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	db.Create(&model.Artist{Name: "Radiohead", NameNorm: "radiohead"})
	db.Create(&model.Artist{Name: "Björk", NameNorm: "bjork"})
	results, err := s.SearchArtists("radio", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Name != "Radiohead" {
		t.Fatalf("unexpected results: %+v", results)
	}
}
