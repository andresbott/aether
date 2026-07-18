package store_test

import (
	"testing"

	"github.com/andresbott/aether/internal/model"
)

func TestCreatePlaylist(t *testing.T) {
	s := testStore(t)
	pl, err := s.CreatePlaylist("My Mix", "admin", false, nil)
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
	pl, err := s.CreatePlaylist("Mix", "admin", false, []uint{t1.ID, t2.ID})
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
	_, _ = s.CreatePlaylist("A", "admin", false, nil)
	_, _ = s.CreatePlaylist("B", "admin", false, nil)
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
	pl, _ := s.CreatePlaylist("Old Name", "admin", false, nil)
	name := "New Name"
	comment := "a comment"
	public := true
	err := s.UpdatePlaylist(pl.ID, &name, &comment, &public)
	if err != nil {
		t.Fatal(err)
	}
	var loaded model.Playlist
	s.DB().First(&loaded, pl.ID)
	if loaded.Name != "New Name" || loaded.Comment != "a comment" || !loaded.Public {
		t.Fatalf("unexpected: %+v", loaded)
	}
}

func TestUpdatePlaylistPartial(t *testing.T) {
	s := testStore(t)
	pl, _ := s.CreatePlaylist("Keep Name", "admin", false, nil)
	comment := "only comment"
	if err := s.UpdatePlaylist(pl.ID, nil, &comment, nil); err != nil {
		t.Fatal(err)
	}
	var loaded model.Playlist
	s.DB().First(&loaded, pl.ID)
	if loaded.Name != "Keep Name" || loaded.Comment != "only comment" {
		t.Fatalf("partial update should not blank name: %+v", loaded)
	}
}

func TestSetPlaylistTracks(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	t1 := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3"}
	t2 := model.Track{AlbumID: album.ID, Filename: "02.mp3", FilePath: "/02.mp3"}
	t3 := model.Track{AlbumID: album.ID, Filename: "03.mp3", FilePath: "/03.mp3"}
	db.Create(&t1)
	db.Create(&t2)
	db.Create(&t3)
	pl, _ := s.CreatePlaylist("Mix", "admin", false, []uint{t1.ID, t2.ID})
	if err := s.SetPlaylistTracks(pl.ID, []uint{t3.ID}); err != nil {
		t.Fatal(err)
	}
	tracks, _ := s.GetPlaylistTracks(pl.ID)
	if len(tracks) != 1 || tracks[0].ID != t3.ID {
		t.Fatalf("expected only t3, got %+v", tracks)
	}
}

func TestDeletePlaylist(t *testing.T) {
	s := testStore(t)
	pl, _ := s.CreatePlaylist("Temp", "admin", false, nil)
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
	pl, _ := s.CreatePlaylist("Mix", "admin", false, nil)
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
	pl, _ := s.CreatePlaylist("Mix", "admin", false, []uint{t1.ID, t2.ID})
	if err := s.RemoveTrackFromPlaylist(pl.ID, 0); err != nil {
		t.Fatal(err)
	}
	tracks, _ := s.GetPlaylistTracks(pl.ID)
	if len(tracks) != 1 {
		t.Fatalf("expected 1, got %d", len(tracks))
	}
}
