package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCoverSettingsRoundTrip(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	in := CoverSettings{ID: 1, DefaultStyles: []string{"bauhaus", "rings"}, AvailableStyles: []string{"classic"}}
	if err := db.Create(&in).Error; err != nil {
		t.Fatal(err)
	}
	var got CoverSettings
	if err := db.First(&got, 1).Error; err != nil {
		t.Fatal(err)
	}
	if len(got.DefaultStyles) != 2 || got.DefaultStyles[0] != "bauhaus" || len(got.AvailableStyles) != 1 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}
