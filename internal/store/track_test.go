package store_test

import (
	"fmt"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

func createTestAlbumAndArtist(t *testing.T, s *store.Store) (model.Album, model.Artist, model.Genre) {
	t.Helper()
	db := s.DB()
	artist := model.Artist{Name: "Radiohead", NameNorm: "radiohead"}
	db.Create(&artist)
	genre := model.Genre{Name: "Rock"}
	db.Create(&genre)
	album := model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead", Year: 2000}
	db.Create(&album)
	_ = db.Model(&album).Association("Artists").Replace([]*model.Artist{&artist})
	_ = db.Model(&album).Association("Genres").Replace([]*model.Genre{&genre})
	return album, artist, genre
}

func TestUpsertTrack(t *testing.T) {
	s := testStore(t)
	album, artist, genre := createTestAlbumAndArtist(t, s)
	track := model.Track{
		AlbumID: album.ID, Filename: "01.mp3", FilePath: "/music/01.mp3",
		Title: "Everything", Duration: 250, Bitrate: 320,
	}
	err := s.Transaction(func(tx *store.Store) error {
		return tx.UpsertTrack(&track, []*model.Artist{&artist}, []*model.Genre{&genre})
	})
	if err != nil {
		t.Fatal(err)
	}
	if track.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	track.Title = "Everything In Its Right Place"
	err = s.Transaction(func(tx *store.Store) error {
		return tx.UpsertTrack(&track, []*model.Artist{&artist}, []*model.Genre{&genre})
	})
	if err != nil {
		t.Fatal(err)
	}
	var loaded model.Track
	s.DB().First(&loaded, track.ID)
	if loaded.Title != "Everything In Its Right Place" {
		t.Fatalf("expected updated title, got: %s", loaded.Title)
	}
}

func TestGetSong(t *testing.T) {
	s := testStore(t)
	album, artist, _ := createTestAlbumAndArtist(t, s)
	db := s.DB()
	track := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/music/01.mp3", Title: "Everything"}
	db.Create(&track)
	_ = db.Model(&track).Association("Artists").Replace([]*model.Artist{&artist})
	found, err := s.GetSong(track.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found.Title != "Everything" {
		t.Fatalf("unexpected: %s", found.Title)
	}
	if found.Album == nil {
		t.Fatal("expected album preload")
	}
	if len(found.Artists) != 1 {
		t.Fatalf("expected 1 artist, got %d", len(found.Artists))
	}
}

func TestGetSongNotFound(t *testing.T) {
	s := testStore(t)
	_, err := s.GetSong(9999)
	if err == nil {
		t.Fatal("expected error for missing song")
	}
}

func TestGetRandomSongs(t *testing.T) {
	s := testStore(t)
	album, _, _ := createTestAlbumAndArtist(t, s)
	db := s.DB()
	for i := 0; i < 5; i++ {
		db.Create(&model.Track{
			AlbumID: album.ID, Filename: fmt.Sprintf("%02d.mp3", i),
			FilePath: fmt.Sprintf("/music/%02d.mp3", i),
			Title: fmt.Sprintf("Track %d", i), Year: 2000,
		})
	}
	songs, err := s.GetRandomSongs(3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(songs) != 3 {
		t.Fatalf("expected 3 songs, got %d", len(songs))
	}
}

func TestGetSongsByGenre(t *testing.T) {
	s := testStore(t)
	album, _, genre := createTestAlbumAndArtist(t, s)
	db := s.DB()
	track := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/music/01.mp3"}
	db.Create(&track)
	_ = db.Model(&track).Association("Genres").Replace([]*model.Genre{&genre})
	songs, err := s.GetSongsByGenre("Rock", 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(songs) != 1 {
		t.Fatalf("expected 1 song, got %d", len(songs))
	}
}

func TestSearchSongs(t *testing.T) {
	s := testStore(t)
	album, _, _ := createTestAlbumAndArtist(t, s)
	db := s.DB()
	db.Create(&model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3", Title: "Everything", TitleNorm: "everything"})
	db.Create(&model.Track{AlbumID: album.ID, Filename: "02.mp3", FilePath: "/02.mp3", Title: "Kid A", TitleNorm: "kid a"})
	results, err := s.SearchSongs("every", 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestGetTrackFilePath(t *testing.T) {
	s := testStore(t)
	album, _, _ := createTestAlbumAndArtist(t, s)
	db := s.DB()
	track := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/music/radiohead/01.mp3"}
	db.Create(&track)
	path, err := s.GetTrackFilePath(track.ID)
	if err != nil {
		t.Fatal(err)
	}
	if path != "/music/radiohead/01.mp3" {
		t.Fatalf("unexpected: %s", path)
	}
}

func TestGetRandomSongsByLibrary(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	lib1 := model.Library{Name: "L1", Path: "/l1"}
	lib2 := model.Library{Name: "L2", Path: "/l2"}
	db.Create(&lib1)
	db.Create(&lib2)

	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "x"}
	db.Create(&album)
	db.Create(&model.Track{AlbumID: album.ID, LibraryID: lib1.ID, Filename: "1.mp3", FilePath: "/l1/1.mp3"})
	db.Create(&model.Track{AlbumID: album.ID, LibraryID: lib1.ID, Filename: "2.mp3", FilePath: "/l1/2.mp3"})
	db.Create(&model.Track{AlbumID: album.ID, LibraryID: lib2.ID, Filename: "3.mp3", FilePath: "/l2/3.mp3"})

	id1 := lib1.ID
	got, err := s.GetRandomSongs(10, &store.RandomSongsFilter{LibraryID: &id1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 tracks in library 1, got %d", len(got))
	}
	for _, tr := range got {
		if tr.LibraryID != lib1.ID {
			t.Fatalf("track %d has LibraryID %d, expected %d", tr.ID, tr.LibraryID, lib1.ID)
		}
	}
}

func TestGetSongsByGenreByLibrary(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	lib1 := model.Library{Name: "L1", Path: "/l1"}
	lib2 := model.Library{Name: "L2", Path: "/l2"}
	db.Create(&lib1)
	db.Create(&lib2)

	rock := model.Genre{Name: "Rock"}
	db.Create(&rock)

	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "x"}
	db.Create(&album)
	t1 := model.Track{AlbumID: album.ID, LibraryID: lib1.ID, Filename: "1.mp3", FilePath: "/l1/1.mp3"}
	t2 := model.Track{AlbumID: album.ID, LibraryID: lib2.ID, Filename: "2.mp3", FilePath: "/l2/2.mp3"}
	db.Create(&t1)
	db.Create(&t2)
	_ = db.Model(&t1).Association("Genres").Replace([]*model.Genre{&rock})
	_ = db.Model(&t2).Association("Genres").Replace([]*model.Genre{&rock})

	id1 := lib1.ID
	got, err := s.GetSongsByGenre("Rock", 10, 0, &store.SearchFilter{LibraryID: &id1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].LibraryID != lib1.ID {
		t.Fatalf("expected 1 track in library 1, got %+v", got)
	}
}

func TestSearchSongsByLibrary(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	lib1 := model.Library{Name: "L1", Path: "/l1"}
	lib2 := model.Library{Name: "L2", Path: "/l2"}
	db.Create(&lib1)
	db.Create(&lib2)

	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "x"}
	db.Create(&album)
	db.Create(&model.Track{AlbumID: album.ID, LibraryID: lib1.ID, Title: "Hello World", TitleNorm: "hello world", Filename: "1.mp3", FilePath: "/l1/1.mp3"})
	db.Create(&model.Track{AlbumID: album.ID, LibraryID: lib2.ID, Title: "Hello There", TitleNorm: "hello there", Filename: "2.mp3", FilePath: "/l2/2.mp3"})

	id1 := lib1.ID
	got, err := s.SearchSongs("hello", 10, 0, &store.SearchFilter{LibraryID: &id1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].LibraryID != lib1.ID {
		t.Fatalf("expected 1 track in library 1, got %+v", got)
	}
}
