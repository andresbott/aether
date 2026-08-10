package model_test

import (
	"testing"

	"github.com/andresbott/aether/internal/model"
)

func TestLibraryIsConfigManaged(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   bool
	}{
		{"config source", model.SourceConfig, true},
		{"db source", model.SourceDB, false},
		// A row written before Source existed reads as empty; it must count as
		// UI-owned so it stays editable rather than becoming stuck read-only.
		{"empty source", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lib := model.Library{Name: "Main", Source: tc.source}
			if got := lib.IsConfigManaged(); got != tc.want {
				t.Fatalf("IsConfigManaged() = %v, want %v", got, tc.want)
			}
		})
	}
}

// The column default must stay "db" so a row created without an explicit source
// (e.g. by a test or a direct insert) is UI-owned rather than config-owned.
func TestLibraryDefaultSourceIsDB(t *testing.T) {
	db := testDB(t)
	lib := model.Library{Name: "Main", Path: "/srv/music"}
	if err := db.Create(&lib).Error; err != nil {
		t.Fatal(err)
	}
	var got model.Library
	if err := db.First(&got, lib.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Source != model.SourceDB {
		t.Fatalf("expected source %q, got %q", model.SourceDB, got.Source)
	}
	if got.IsConfigManaged() {
		t.Fatal("a library created without a source must not be config-managed")
	}
}
