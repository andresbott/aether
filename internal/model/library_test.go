package model_test

import (
	"testing"

	"github.com/andresbott/aether/internal/model"
)

// The column defaults must stay in place: a library created with just a name
// (e.g. through CreateLibrary) still gets a browsable default view and an icon.
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
	if got.DefaultView != "albums" {
		t.Fatalf("expected default_view %q, got %q", "albums", got.DefaultView)
	}
	if got.Icon != "folder" {
		t.Fatalf("expected icon %q, got %q", "folder", got.Icon)
	}
}
