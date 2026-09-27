package store_test

import (
	"slices"
	"testing"

	"github.com/andresbott/aether/internal/model"
)

// TestFilterOptions seeds a small catalog exercising every rule
// FilterOptions applies: a blank suffix is not a format, a genre tagging no
// track is not offered, and release types are de-duplicated
// case-insensitively (the first spelling in the SQL sort order wins) while a
// nil ReleaseTypes slice — stored as the JSON literal null — contributes
// nothing.
func TestFilterOptions(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	jazz := &model.Genre{Name: "Jazz"}
	ambient := &model.Genre{Name: "Ambient"}
	unused := &model.Genre{Name: "Unused"}
	for _, g := range []*model.Genre{jazz, ambient, unused} {
		if err := db.Create(g).Error; err != nil {
			t.Fatal(err)
		}
	}

	typed := model.Album{Name: "typed", NameNorm: "typed", AlbumArtistNorm: "x", ReleaseTypes: []string{"Album"}}
	mixedCase := model.Album{Name: "mixedCase", NameNorm: "mixedcase", AlbumArtistNorm: "x", ReleaseTypes: []string{"album", "Live"}}
	untyped := model.Album{Name: "untyped", NameNorm: "untyped", AlbumArtistNorm: "x"} // nil ReleaseTypes
	for _, al := range []*model.Album{&typed, &mixedCase, &untyped} {
		if err := db.Create(al).Error; err != nil {
			t.Fatal(err)
		}
	}

	track := func(suffix, filename string, genres ...*model.Genre) {
		tr := model.Track{
			AlbumID: typed.ID, Suffix: suffix, Title: filename, TitleNorm: filename,
			Filename: filename, FilePath: "/music/" + filename,
		}
		if err := db.Create(&tr).Error; err != nil {
			t.Fatal(err)
		}
		if len(genres) > 0 {
			if err := db.Model(&tr).Association("Genres").Replace(genres); err != nil {
				t.Fatal(err)
			}
		}
	}
	track("flac", "1.flac", jazz)
	track("mp3", "2.mp3", ambient)
	track("flac", "3.flac")
	track("", "4.unknown")

	got, err := s.FilterOptions()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Formats, []string{"flac", "mp3"}) {
		t.Errorf("Formats = %v, want [flac mp3] (blank suffix excluded)", got.Formats)
	}
	if !slices.Equal(got.Genres, []string{"Ambient", "Jazz"}) {
		t.Errorf("Genres = %v, want [Ambient Jazz] (Unused tags no track)", got.Genres)
	}
	if !slices.Equal(got.ReleaseTypes, []string{"Album", "Live"}) {
		t.Errorf("ReleaseTypes = %v, want [Album Live] (case-insensitive de-dup, nil contributes nothing)", got.ReleaseTypes)
	}
}
