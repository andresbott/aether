package store_test

import (
	"testing"

	"github.com/andresbott/aether/internal/model"
)

func TestAssetIdentitiesReturnsEveryCoverOwningEntity(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	db.Create(&model.Album{Name: "Al", NameNorm: "al", AlbumArtistNorm: "ar", MBReleaseID: "rel-1"})
	db.Create(&model.Artist{Name: "Ar", NameNorm: "ar", MBArtistID: "mbid-1"})
	db.Create(&model.Genre{Name: "Rock"})
	db.Create(&model.Playlist{UUID: "uuid-1", Name: "P"})
	db.Create(&model.InternetRadioStation{Name: "R", StreamURL: "http://s"})

	ids, err := s.AssetIdentities()
	if err != nil {
		t.Fatalf("AssetIdentities: %v", err)
	}
	if len(ids.Albums) != 1 || len(ids.Artists) != 1 || len(ids.Genres) != 1 ||
		len(ids.Playlists) != 1 || len(ids.Radios) != 1 {
		t.Fatalf("counts: albums=%d artists=%d genres=%d playlists=%d radios=%d",
			len(ids.Albums), len(ids.Artists), len(ids.Genres), len(ids.Playlists), len(ids.Radios))
	}
	// The identity fields the asset keys are derived from must be populated.
	if ids.Albums[0].MBReleaseID != "rel-1" || ids.Albums[0].NameNorm != "al" ||
		ids.Artists[0].MBArtistID != "mbid-1" || ids.Artists[0].NameNorm != "ar" ||
		ids.Genres[0].Name != "Rock" || ids.Playlists[0].UUID != "uuid-1" ||
		ids.Radios[0].StreamURL != "http://s" {
		t.Fatalf("identity fields not populated: %+v", ids)
	}
}

func TestAssetIdentitiesEmptyStore(t *testing.T) {
	s := testStore(t)
	ids, err := s.AssetIdentities()
	if err != nil {
		t.Fatalf("AssetIdentities: %v", err)
	}
	if len(ids.Albums)+len(ids.Artists)+len(ids.Genres)+len(ids.Playlists)+len(ids.Radios) != 0 {
		t.Fatalf("expected no identities, got %+v", ids)
	}
}
