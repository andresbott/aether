package store_test

import (
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

func TestStarAndUnstar(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	artist := model.Artist{Name: "A", NameNorm: "a"}
	db.Create(&artist)
	if err := s.Star("artist", artist.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Star("artist", artist.ID); err != nil {
		t.Fatal(err)
	} // idempotent
	starred, err := s.GetStarred(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(starred.Artists) != 1 {
		t.Fatalf("expected 1 starred artist, got %d", len(starred.Artists))
	}
	if err := s.Unstar("artist", artist.ID); err != nil {
		t.Fatal(err)
	}
	starred, _ = s.GetStarred(nil)
	if len(starred.Artists) != 0 {
		t.Fatal("expected 0 starred artists after unstar")
	}
}

func TestGetStarredAll(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	artist := model.Artist{Name: "A", NameNorm: "a"}
	db.Create(&artist)
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "a"}
	db.Create(&album)
	track := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3"}
	db.Create(&track)
	s.Star("artist", artist.ID)
	s.Star("album", album.ID)
	s.Star("track", track.ID)
	starred, err := s.GetStarred(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(starred.Artists) != 1 || len(starred.Albums) != 1 || len(starred.Tracks) != 1 {
		t.Fatalf("unexpected: artists=%d albums=%d tracks=%d", len(starred.Artists), len(starred.Albums), len(starred.Tracks))
	}
}

func TestGetStarredByLibrary(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	lib1 := model.Library{Name: "L1", Path: "/l1"}
	lib2 := model.Library{Name: "L2", Path: "/l2"}
	db.Create(&lib1)
	db.Create(&lib2)

	artist1 := model.Artist{Name: "A1", NameNorm: "a1"}
	artist2 := model.Artist{Name: "A2", NameNorm: "a2"}
	db.Create(&artist1)
	db.Create(&artist2)

	album1 := model.Album{Name: "Alb1", NameNorm: "alb1", AlbumArtistNorm: "a1"}
	album2 := model.Album{Name: "Alb2", NameNorm: "alb2", AlbumArtistNorm: "a2"}
	db.Create(&album1)
	db.Create(&album2)

	t1 := model.Track{AlbumID: album1.ID, LibraryID: lib1.ID, Filename: "1.mp3", FilePath: "/l1/1.mp3"}
	t2 := model.Track{AlbumID: album2.ID, LibraryID: lib2.ID, Filename: "2.mp3", FilePath: "/l2/2.mp3"}
	db.Create(&t1)
	db.Create(&t2)
	db.Model(&t1).Association("Artists").Replace([]*model.Artist{&artist1})
	db.Model(&t2).Association("Artists").Replace([]*model.Artist{&artist2})

	if err := s.Star("artist", artist1.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Star("artist", artist2.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Star("album", album1.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Star("album", album2.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Star("track", t1.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Star("track", t2.ID); err != nil {
		t.Fatal(err)
	}

	id1 := lib1.ID
	got, err := s.GetStarred(&store.StarredFilter{LibraryID: &id1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Artists) != 1 || got.Artists[0].Name != "A1" {
		t.Fatalf("expected [A1], got %+v", got.Artists)
	}
	if len(got.Albums) != 1 || got.Albums[0].Name != "Alb1" {
		t.Fatalf("expected [Alb1], got %+v", got.Albums)
	}
	if len(got.Tracks) != 1 || got.Tracks[0].LibraryID != lib1.ID {
		t.Fatalf("expected 1 track in library 1, got %+v", got.Tracks)
	}

	all, err := s.GetStarred(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Artists) != 2 || len(all.Albums) != 2 || len(all.Tracks) != 2 {
		t.Fatalf("expected all starred items with nil filter, got %+v", all)
	}
}
