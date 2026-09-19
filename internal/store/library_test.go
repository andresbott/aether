package store_test

import (
	"errors"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
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

func TestLibraryRoots(t *testing.T) {
	s := testStore(t)
	if err := s.CreateLibrary(&model.Library{Name: "A", Path: "/a"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateLibrary(&model.Library{Name: "B", Path: "/b"}); err != nil {
		t.Fatal(err)
	}

	roots, err := s.LibraryRoots()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, r := range roots {
		got[r] = true
	}
	if len(roots) != 2 || !got["/a"] || !got["/b"] {
		t.Fatalf("expected both library paths, got %v", roots)
	}
}

func TestLibraryRootsEmpty(t *testing.T) {
	s := testStore(t)
	roots, err := s.LibraryRoots()
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 0 {
		t.Fatalf("expected no roots, got %v", roots)
	}
}

func TestGetLibraryNotFound(t *testing.T) {
	s := testStore(t)
	_, err := s.GetLibrary(999)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected store.ErrNotFound, got %v", err)
	}
}

func TestFindLibraryByNameNotFound(t *testing.T) {
	s := testStore(t)
	_, err := s.FindLibraryByName("nope")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected store.ErrNotFound, got %v", err)
	}
}

func TestFindLibraryByPathNotFound(t *testing.T) {
	s := testStore(t)
	_, err := s.FindLibraryByPath("/nope")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected store.ErrNotFound, got %v", err)
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

// A library is a view: deleting it must leave every track in place.
func TestDeleteLibraryKeepsTracks(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	lib := &model.Library{Name: "L", Path: "/l"}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	db.Create(&model.Track{AlbumID: album.ID, ScanFolder: "L", Filename: "1.mp3", FilePath: "/l/1.mp3"})

	if err := s.DeleteLibrary(t.Context(), lib.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetLibrary(lib.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected the library gone, got %v", err)
	}
	var n int64
	db.Model(&model.Track{}).Count(&n)
	if n != 1 {
		t.Fatalf("expected the track to survive, got %d", n)
	}
}

func TestCreateLibraryHideArtistsRoundTrip(t *testing.T) {
	s := testStore(t)
	lib := &model.Library{Name: "Main", Path: "/a", HideArtists: true}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetLibrary(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.HideArtists {
		t.Fatal("expected HideArtists to persist on create")
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps to be set on create, got created=%v updated=%v", got.CreatedAt, got.UpdatedAt)
	}
}
