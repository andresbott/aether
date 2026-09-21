package store_test

import (
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return store.New(db)
}

// scanFolderFilter builds the filters of a library that selects tracks from
// the named scan folders — the shape most test libraries use to stand in for
// "the tracks stamped with this scan folder".
func scanFolderFilter(names ...string) []model.LibraryFilter {
	return []model.LibraryFilter{{Field: model.FilterScanFolder, Values: names}}
}
