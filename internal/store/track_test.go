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
	db.Model(&album).Association("Artists").Replace([]*model.Artist{&artist})
	db.Model(&album).Association("Genres").Replace([]*model.Genre{&genre})
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
	db.Model(&track).Association("Artists").Replace([]*model.Artist{&artist})
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
	db.Model(&track).Association("Genres").Replace([]*model.Genre{&genre})
	songs, err := s.GetSongsByGenre("Rock", 10, 0)
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
	results, err := s.SearchSongs("every", 10, 0)
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
