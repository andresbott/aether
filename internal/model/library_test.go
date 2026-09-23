package model_test

import (
	"slices"
	"testing"

	"github.com/andresbott/aether/internal/model"
)

// The column defaults must stay in place: a library created with just a name
// (e.g. through CreateLibrary) still gets every view, a default view among
// them, and an icon.
func TestLibraryColumnDefaults(t *testing.T) {
	db := testDB(t)
	lib := model.Library{Name: "Main"}
	if err := db.Create(&lib).Error; err != nil {
		t.Fatal(err)
	}
	var got model.Library
	if err := db.First(&got, lib.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Views, model.LibraryViews()) {
		t.Fatalf("expected views %v, got %v", model.LibraryViews(), got.Views)
	}
	if got.DefaultView != model.ViewDiscover {
		t.Fatalf("expected default_view %q, got %q", model.ViewDiscover, got.DefaultView)
	}
	if got.HideFromArtistIndex {
		t.Fatal("expected a new library's artists in the artist index")
	}
	if got.SplitViews {
		t.Fatal("expected a new library to take a single sidebar entry")
	}
	if got.Icon != "folder" {
		t.Fatalf("expected icon %q, got %q", "folder", got.Icon)
	}
}

// Explicit views are stored as given, not replaced by the column default.
func TestLibraryViewsRoundTrip(t *testing.T) {
	db := testDB(t)
	lib := model.Library{Name: "Books", Views: []model.LibraryView{model.ViewReleases}, DefaultView: model.ViewReleases}
	if err := db.Create(&lib).Error; err != nil {
		t.Fatal(err)
	}
	var got model.Library
	if err := db.First(&got, lib.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Views, []model.LibraryView{model.ViewReleases}) || got.DefaultView != model.ViewReleases {
		t.Fatalf("got views %v opening on %q, want [releases] opening on releases", got.Views, got.DefaultView)
	}
}

// A saved false survives the insert: CatalogSettings.SplitViews carries no
// column default that would turn it back into true.
func TestCatalogSettingsPersistsFalse(t *testing.T) {
	db := testDB(t)
	if !model.DefaultCatalogSettings().SplitViews {
		t.Fatal("expected the root to split its views by default")
	}
	if err := db.Create(&model.CatalogSettings{ID: 1, SplitViews: false}).Error; err != nil {
		t.Fatal(err)
	}
	var got model.CatalogSettings
	if err := db.First(&got, 1).Error; err != nil {
		t.Fatal(err)
	}
	if got.SplitViews {
		t.Fatal("expected split_views=false to persist")
	}
}
