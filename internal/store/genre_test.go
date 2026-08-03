package store_test

import (
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

func TestFindOrCreateGenres(t *testing.T) {
	s := testStore(t)
	var genres []*model.Genre
	err := s.Transaction(func(tx *store.Store) error {
		var txErr error
		genres, txErr = tx.FindOrCreateGenres([]string{"Rock", "Electronic"})
		return txErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(genres) != 2 {
		t.Fatalf("expected 2 genres, got %d", len(genres))
	}
	var again []*model.Genre
	err = s.Transaction(func(tx *store.Store) error {
		var txErr error
		again, txErr = tx.FindOrCreateGenres([]string{"Rock"})
		return txErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if again[0].ID != genres[0].ID {
		t.Fatal("expected same genre ID")
	}
}

func TestGetGenre(t *testing.T) {
	s := testStore(t)
	rock := model.Genre{Name: "Rock"}
	s.DB().Create(&rock)

	got, err := s.GetGenre(rock.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Rock" {
		t.Fatalf("expected Rock, got %q", got.Name)
	}

	if _, err := s.GetGenre(999); err == nil {
		t.Fatal("expected error for missing genre")
	}
}

func TestSearchGenres(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	for _, g := range []model.Genre{
		{Name: "Rock"},
		{Name: "Post-Rock"},
		{Name: "Éxtreme Métal"},
		{Name: "Jazz"},
	} {
		if err := db.Create(&g).Error; err != nil {
			t.Fatal(err)
		}
	}

	// Substring match, ordered by normalized name.
	got, err := s.SearchGenres("rock", 20, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 genres, got %d: %+v", len(got), got)
	}
	if got[0].Name != "Post-Rock" || got[1].Name != "Rock" {
		t.Fatalf("expected Post-Rock then Rock, got %q, %q", got[0].Name, got[1].Name)
	}

	// Accents and case fold the same way the other searches do.
	got, err = s.SearchGenres("extreme me", 20, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Éxtreme Métal" {
		t.Fatalf("expected the accented genre, got %+v", got)
	}

	// Count and offset page through the matches.
	got, err = s.SearchGenres("rock", 1, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Rock" {
		t.Fatalf("expected the second match, got %+v", got)
	}
}

func TestSearchGenresReportsCounts(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	rock := model.Genre{Name: "Rock"}
	db.Create(&rock)
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "a"}
	db.Create(&album)
	_ = db.Model(&album).Association("Genres").Replace([]*model.Genre{&rock})
	for _, name := range []string{"01.mp3", "02.mp3"} {
		track := model.Track{AlbumID: album.ID, Filename: name, FilePath: "/" + name}
		db.Create(&track)
		_ = db.Model(&track).Association("Genres").Replace([]*model.Genre{&rock})
	}

	got, err := s.SearchGenres("rock", 20, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 genre, got %d", len(got))
	}
	if got[0].SongCount != 2 || got[0].AlbumCount != 1 {
		t.Fatalf("expected 2 songs / 1 album, got %d / %d", got[0].SongCount, got[0].AlbumCount)
	}
}

func TestSearchGenresFiltersByLibrary(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	rock := model.Genre{Name: "Rock"}
	prog := model.Genre{Name: "Prog Rock"}
	db.Create(&rock)
	db.Create(&prog)
	libA := model.Library{Name: "A", Path: "/a"}
	libB := model.Library{Name: "B", Path: "/b"}
	db.Create(&libA)
	db.Create(&libB)
	trackA := model.Track{Filename: "a.mp3", FilePath: "/a/a.mp3", LibraryID: libA.ID}
	trackB := model.Track{Filename: "b.mp3", FilePath: "/b/b.mp3", LibraryID: libB.ID}
	db.Create(&trackA)
	db.Create(&trackB)
	_ = db.Model(&trackA).Association("Genres").Replace([]*model.Genre{&rock})
	_ = db.Model(&trackB).Association("Genres").Replace([]*model.Genre{&prog})

	got, err := s.SearchGenres("rock", 20, 0, &store.SearchFilter{LibraryID: &libA.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Rock" {
		t.Fatalf("expected only the genre present in library A, got %+v", got)
	}
}

func TestGetGenres(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	rock := model.Genre{Name: "Rock"}
	electronic := model.Genre{Name: "Electronic"}
	db.Create(&rock)
	db.Create(&electronic)
	artist := model.Artist{Name: "A", NameNorm: "a"}
	db.Create(&artist)
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "a"}
	db.Create(&album)
	_ = db.Model(&album).Association("Genres").Replace([]*model.Genre{&rock})
	track := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3"}
	db.Create(&track)
	_ = db.Model(&track).Association("Genres").Replace([]*model.Genre{&rock, &electronic})
	genres, err := s.GetGenres()
	if err != nil {
		t.Fatal(err)
	}
	if len(genres) != 2 {
		t.Fatalf("expected 2 genres, got %d", len(genres))
	}
}
