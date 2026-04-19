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
	db.Model(&album).Association("Genres").Replace([]*model.Genre{&rock})
	track := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3"}
	db.Create(&track)
	db.Model(&track).Association("Genres").Replace([]*model.Genre{&rock, &electronic})
	genres, err := s.GetGenres()
	if err != nil {
		t.Fatal(err)
	}
	if len(genres) != 2 {
		t.Fatalf("expected 2 genres, got %d", len(genres))
	}
}
