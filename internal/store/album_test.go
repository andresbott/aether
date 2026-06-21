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
	_ = db.Model(&album).Association("Artists").Replace([]*model.Artist{&artist})
	rock := model.Genre{Name: "Rock"}
	db.Create(&rock)
	_ = db.Model(&album).Association("Genres").Replace([]*model.Genre{&rock})
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
	results, err := s.SearchAlbums("kid", 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestGetAlbumListByLibrary(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	lib1 := model.Library{Name: "L1", Path: "/l1"}
	lib2 := model.Library{Name: "L2", Path: "/l2"}
	db.Create(&lib1)
	db.Create(&lib2)

	onlyL1 := model.Album{Name: "Only1", NameNorm: "only1", AlbumArtistNorm: "x"}
	onlyL2 := model.Album{Name: "Only2", NameNorm: "only2", AlbumArtistNorm: "x"}
	shared := model.Album{Name: "Shared", NameNorm: "shared", AlbumArtistNorm: "x"}
	db.Create(&onlyL1)
	db.Create(&onlyL2)
	db.Create(&shared)

	db.Create(&model.Track{AlbumID: onlyL1.ID, LibraryID: lib1.ID, Filename: "a.mp3", FilePath: "/l1/a.mp3"})
	db.Create(&model.Track{AlbumID: onlyL2.ID, LibraryID: lib2.ID, Filename: "b.mp3", FilePath: "/l2/b.mp3"})
	db.Create(&model.Track{AlbumID: shared.ID, LibraryID: lib1.ID, Filename: "c.mp3", FilePath: "/l1/c.mp3"})
	db.Create(&model.Track{AlbumID: shared.ID, LibraryID: lib2.ID, Filename: "d.mp3", FilePath: "/l2/d.mp3"})

	id1 := lib1.ID
	got, err := s.GetAlbumList("alphabeticalByName", 10, 0, &store.AlbumListFilter{LibraryID: &id1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 albums (Only1 + Shared), got %d", len(got))
	}
	names := map[string]bool{got[0].Name: true, got[1].Name: true}
	if !names["Only1"] || !names["Shared"] {
		t.Fatalf("expected Only1 and Shared, got %v", names)
	}

	all, err := s.GetAlbumList("alphabeticalByName", 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 albums with nil filter, got %d", len(all))
	}
}

func TestSearchAlbumsByLibrary(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	lib1 := model.Library{Name: "L1", Path: "/l1"}
	lib2 := model.Library{Name: "L2", Path: "/l2"}
	db.Create(&lib1)
	db.Create(&lib2)

	a1 := model.Album{Name: "Blue Album", NameNorm: "blue album", AlbumArtistNorm: "x"}
	a2 := model.Album{Name: "Blue Planet", NameNorm: "blue planet", AlbumArtistNorm: "x"}
	db.Create(&a1)
	db.Create(&a2)
	db.Create(&model.Track{AlbumID: a1.ID, LibraryID: lib1.ID, Filename: "1.mp3", FilePath: "/l1/1.mp3"})
	db.Create(&model.Track{AlbumID: a2.ID, LibraryID: lib2.ID, Filename: "2.mp3", FilePath: "/l2/2.mp3"})

	id1 := lib1.ID
	got, err := s.SearchAlbums("blue", 10, 0, &store.SearchFilter{LibraryID: &id1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Blue Album" {
		t.Fatalf("expected only Blue Album, got %+v", got)
	}

	all, err := s.SearchAlbums("blue", 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 albums with nil filter, got %d", len(all))
	}
}
