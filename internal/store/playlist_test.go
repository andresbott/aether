package store_test

import (
	"testing"

	"github.com/andresbott/aether/internal/model"
)

func TestCreatePlaylist(t *testing.T) {
	s := testStore(t)
	pl, err := s.CreatePlaylist("My Mix", "admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	if pl.ID == 0 || pl.Name != "My Mix" {
		t.Fatalf("unexpected: %+v", pl)
	}
}

func TestCreatePlaylistWithTracks(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	t1 := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3"}
	t2 := model.Track{AlbumID: album.ID, Filename: "02.mp3", FilePath: "/02.mp3"}
	db.Create(&t1)
	db.Create(&t2)
	pl, err := s.CreatePlaylist("Mix", "admin", []uint{t1.ID, t2.ID})
	if err != nil {
		t.Fatal(err)
	}
	tracks, err := s.GetPlaylistTracks(pl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(tracks))
	}
}

func TestGetPlaylists(t *testing.T) {
	s := testStore(t)
	s.CreatePlaylist("A", "admin", nil)
	s.CreatePlaylist("B", "admin", nil)
	playlists, err := s.GetPlaylists()
	if err != nil {
		t.Fatal(err)
	}
	if len(playlists) != 2 {
		t.Fatalf("expected 2, got %d", len(playlists))
	}
}

func TestUpdatePlaylist(t *testing.T) {
	s := testStore(t)
	pl, _ := s.CreatePlaylist("Old Name", "admin", nil)
	err := s.UpdatePlaylist(pl.ID, "New Name", "a comment")
	if err != nil {
		t.Fatal(err)
	}
	var loaded model.Playlist
	s.DB().First(&loaded, pl.ID)
	if loaded.Name != "New Name" || loaded.Comment != "a comment" {
		t.Fatalf("unexpected: %+v", loaded)
	}
}

func TestDeletePlaylist(t *testing.T) {
	s := testStore(t)
	pl, _ := s.CreatePlaylist("Temp", "admin", nil)
	if err := s.DeletePlaylist(pl.ID); err != nil {
		t.Fatal(err)
	}
	var count int64
	s.DB().Model(&model.Playlist{}).Count(&count)
	if count != 0 {
		t.Fatalf("expected 0 playlists, got %d", count)
	}
}

func TestAddTracksToPlaylist(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	t1 := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3"}
	db.Create(&t1)
	pl, _ := s.CreatePlaylist("Mix", "admin", nil)
	if err := s.AddTracksToPlaylist(pl.ID, []uint{t1.ID}); err != nil {
		t.Fatal(err)
	}
	tracks, _ := s.GetPlaylistTracks(pl.ID)
	if len(tracks) != 1 {
		t.Fatalf("expected 1, got %d", len(tracks))
	}
}

func TestRemoveTrackFromPlaylist(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	t1 := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3"}
	t2 := model.Track{AlbumID: album.ID, Filename: "02.mp3", FilePath: "/02.mp3"}
	db.Create(&t1)
	db.Create(&t2)
	pl, _ := s.CreatePlaylist("Mix", "admin", []uint{t1.ID, t2.ID})
	if err := s.RemoveTrackFromPlaylist(pl.ID, 0); err != nil {
		t.Fatal(err)
	}
	tracks, _ := s.GetPlaylistTracks(pl.ID)
	if len(tracks) != 1 {
		t.Fatalf("expected 1, got %d", len(tracks))
	}
}
