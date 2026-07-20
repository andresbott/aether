package store_test

import (
	"errors"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"gorm.io/gorm"
)

func TestCreateAndGetLibrary(t *testing.T) {
	s := testStore(t)
	lib := &model.Library{Name: "Main", Path: "/srv/music"}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	if lib.ID == 0 {
		t.Fatal("expected ID to be set after Create")
	}

	got, err := s.GetLibrary(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Main" || got.Path != "/srv/music" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetLibraryNotFound(t *testing.T) {
	s := testStore(t)
	_, err := s.GetLibrary(999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestListLibraries(t *testing.T) {
	s := testStore(t)
	if err := s.CreateLibrary(&model.Library{Name: "A", Path: "/a"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateLibrary(&model.Library{Name: "B", Path: "/b"}); err != nil {
		t.Fatal(err)
	}
	libs, err := s.ListLibraries()
	if err != nil {
		t.Fatal(err)
	}
	if len(libs) != 2 {
		t.Fatalf("expected 2 libraries, got %d", len(libs))
	}
}

func TestCreateLibraryDuplicateName(t *testing.T) {
	s := testStore(t)
	if err := s.CreateLibrary(&model.Library{Name: "Main", Path: "/a"}); err != nil {
		t.Fatal(err)
	}
	err := s.CreateLibrary(&model.Library{Name: "Main", Path: "/b"})
	if err == nil {
		t.Fatal("expected duplicate-name error")
	}
}

func TestCreateLibraryDuplicatePath(t *testing.T) {
	s := testStore(t)
	if err := s.CreateLibrary(&model.Library{Name: "A", Path: "/a"}); err != nil {
		t.Fatal(err)
	}
	err := s.CreateLibrary(&model.Library{Name: "B", Path: "/a"})
	if err == nil {
		t.Fatal("expected duplicate-path error")
	}
}

func TestUpdateLibrary(t *testing.T) {
	s := testStore(t)
	lib := &model.Library{Name: "Main", Path: "/a", FollowSymlinks: false}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	lib.Name = "Renamed"
	lib.FollowSymlinks = true
	if err := s.UpdateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetLibrary(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Renamed" || !got.FollowSymlinks {
		t.Fatalf("update did not persist: %+v", got)
	}
}

func TestDeleteTracksForLibrary(t *testing.T) {
	s := testStore(t)
	lib := &model.Library{Name: "L", Path: "/l"}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	otherLib := &model.Library{Name: "O", Path: "/o"}
	if err := s.CreateLibrary(otherLib); err != nil {
		t.Fatal(err)
	}
	db := s.DB()
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	db.Create(&model.Track{AlbumID: album.ID, LibraryID: lib.ID, Filename: "1.mp3", FilePath: "/l/1.mp3"})
	db.Create(&model.Track{AlbumID: album.ID, LibraryID: lib.ID, Filename: "2.mp3", FilePath: "/l/2.mp3"})
	db.Create(&model.Track{AlbumID: album.ID, LibraryID: otherLib.ID, Filename: "3.mp3", FilePath: "/o/3.mp3"})

	if err := s.DeleteTracksForLibrary(lib.ID); err != nil {
		t.Fatal(err)
	}

	var count int64
	db.Model(&model.Track{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 surviving track, got %d", count)
	}
}

func TestDeleteLibraryCascade(t *testing.T) {
	s := testStore(t)
	lib := &model.Library{Name: "L", Path: "/l"}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	db := s.DB()
	artist := model.Artist{Name: "X", NameNorm: "x"}
	db.Create(&artist)
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	_ = db.Model(&album).Association("Artists").Replace([]*model.Artist{&artist})
	track := model.Track{AlbumID: album.ID, LibraryID: lib.ID, Filename: "1.mp3", FilePath: "/l/1.mp3"}
	db.Create(&track)
	_ = db.Model(&track).Association("Artists").Replace([]*model.Artist{&artist})
	db.Create(&model.StarredItem{ItemType: "track", ItemID: track.ID})

	if err := s.DeleteLibrary(lib.ID); err != nil {
		t.Fatal(err)
	}

	var libCount, trackCount, albumCount, artistCount, starCount int64
	db.Model(&model.Library{}).Count(&libCount)
	db.Model(&model.Track{}).Count(&trackCount)
	db.Model(&model.Album{}).Count(&albumCount)
	db.Model(&model.Artist{}).Count(&artistCount)
	db.Model(&model.StarredItem{}).Count(&starCount)
	if libCount != 0 || trackCount != 0 || albumCount != 0 || artistCount != 0 || starCount != 0 {
		t.Fatalf("expected full cascade, got lib=%d t=%d alb=%d ar=%d star=%d",
			libCount, trackCount, albumCount, artistCount, starCount)
	}
}

func TestCreateLibraryFalseBoolsRoundTrip(t *testing.T) {
	s := testStore(t)
	lib := &model.Library{Name: "Main", Path: "/a", FollowSymlinks: false, ShowArtists: false}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetLibrary(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FollowSymlinks || got.ShowArtists {
		t.Fatalf("expected both bools false after create, got follow=%v show=%v", got.FollowSymlinks, got.ShowArtists)
	}
}
