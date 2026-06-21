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
	artists, err := s.GetArtists(nil)
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
	_ = db.Model(&album).Association("Artists").Replace([]*model.Artist{&artist})
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
	results, err := s.SearchArtists("radio", 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Name != "Radiohead" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestGetArtistsByLibrary(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	lib1 := model.Library{Name: "L1", Path: "/l1"}
	lib2 := model.Library{Name: "L2", Path: "/l2"}
	db.Create(&lib1)
	db.Create(&lib2)

	a1 := model.Artist{Name: "Alpha", NameNorm: "alpha"}
	a2 := model.Artist{Name: "Beta", NameNorm: "beta"}
	a3 := model.Artist{Name: "Gamma", NameNorm: "gamma"}
	db.Create(&a1)
	db.Create(&a2)
	db.Create(&a3)

	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "x"}
	db.Create(&album)
	t1 := model.Track{AlbumID: album.ID, LibraryID: lib1.ID, Filename: "1.mp3", FilePath: "/l1/1.mp3"}
	t2 := model.Track{AlbumID: album.ID, LibraryID: lib2.ID, Filename: "2.mp3", FilePath: "/l2/2.mp3"}
	t3 := model.Track{AlbumID: album.ID, LibraryID: lib1.ID, Filename: "3.mp3", FilePath: "/l1/3.mp3"}
	db.Create(&t1)
	db.Create(&t2)
	db.Create(&t3)
	_ = db.Model(&t1).Association("Artists").Replace([]*model.Artist{&a1})
	_ = db.Model(&t2).Association("Artists").Replace([]*model.Artist{&a2})
	_ = db.Model(&t3).Association("Artists").Replace([]*model.Artist{&a3, &a1})

	id1 := lib1.ID
	got, err := s.GetArtists(&store.ArtistsFilter{LibraryID: &id1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected Alpha + Gamma, got %d: %+v", len(got), got)
	}

	all, err := s.GetArtists(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 artists with nil filter, got %d", len(all))
	}
}

func TestGetArtistAlbumCountsByLibrary(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	lib1 := model.Library{Name: "L1", Path: "/l1"}
	lib2 := model.Library{Name: "L2", Path: "/l2"}
	db.Create(&lib1)
	db.Create(&lib2)

	artist := model.Artist{Name: "Alpha", NameNorm: "alpha"}
	db.Create(&artist)

	alb1 := model.Album{Name: "A1", NameNorm: "a1", AlbumArtistNorm: "alpha"}
	alb2 := model.Album{Name: "A2", NameNorm: "a2", AlbumArtistNorm: "alpha"}
	db.Create(&alb1)
	db.Create(&alb2)
	_ = db.Model(&alb1).Association("Artists").Replace([]*model.Artist{&artist})
	_ = db.Model(&alb2).Association("Artists").Replace([]*model.Artist{&artist})

	db.Create(&model.Track{AlbumID: alb1.ID, LibraryID: lib1.ID, Filename: "1.mp3", FilePath: "/l1/1.mp3"})
	db.Create(&model.Track{AlbumID: alb2.ID, LibraryID: lib2.ID, Filename: "2.mp3", FilePath: "/l2/2.mp3"})

	id1 := lib1.ID
	counts, err := s.GetArtistAlbumCounts(&store.ArtistsFilter{LibraryID: &id1})
	if err != nil {
		t.Fatal(err)
	}
	if counts[artist.ID] != 1 {
		t.Fatalf("expected 1 album in library 1, got %d", counts[artist.ID])
	}
}

func TestSearchArtistsByLibrary(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	lib1 := model.Library{Name: "L1", Path: "/l1"}
	lib2 := model.Library{Name: "L2", Path: "/l2"}
	db.Create(&lib1)
	db.Create(&lib2)

	a1 := model.Artist{Name: "Alpha", NameNorm: "alpha"}
	a2 := model.Artist{Name: "Alphonse", NameNorm: "alphonse"}
	db.Create(&a1)
	db.Create(&a2)

	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "x"}
	db.Create(&album)
	t1 := model.Track{AlbumID: album.ID, LibraryID: lib1.ID, Filename: "1.mp3", FilePath: "/l1/1.mp3"}
	t2 := model.Track{AlbumID: album.ID, LibraryID: lib2.ID, Filename: "2.mp3", FilePath: "/l2/2.mp3"}
	db.Create(&t1)
	db.Create(&t2)
	_ = db.Model(&t1).Association("Artists").Replace([]*model.Artist{&a1})
	_ = db.Model(&t2).Association("Artists").Replace([]*model.Artist{&a2})

	id1 := lib1.ID
	got, err := s.SearchArtists("alph", 10, 0, &store.SearchFilter{LibraryID: &id1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Alpha" {
		t.Fatalf("expected Alpha only, got %+v", got)
	}
}
