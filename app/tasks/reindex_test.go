// app/tasks/reindex_test.go
package tasks

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
)

func TestNewReindexTaskFnIndexesPaths(t *testing.T) {
	// Arrange: a store with one library whose folder holds one audio file that
	// is NOT yet in the index. (Reuse the scanner package's test fixtures /
	// helpers for building store+library+file; see internal/scanner/*_test.go.)
	cfg, store, reader, libID, audioPath := setupReindexFixture(t)

	fn := NewReindexTaskFn(cfg, store, reader)
	if err := fn(context.Background(), slog.Default(), ReindexParams{LibraryID: libID, Paths: []string{audioPath}}); err != nil {
		t.Fatalf("reindex fn: %v", err)
	}

	if n := countTracks(t, store, libID); n != 1 {
		t.Fatalf("track not indexed: got %d tracks, want 1", n)
	}
}

// setupReindexFixture builds a store with one library whose folder holds one
// real (tag-less) audio file, plus a real tags.Reader — so the reindex task's
// RescanPaths call exercises actual tag reading end to end (a stub reader
// would not catch a wiring break between the task, the scanner, and a real
// Reader).
//
// It reuses newTestStore (defined in artistimage_test.go) for the
// migrated in-memory store, and copies the same testdata/empty.flac fixture
// internal/tags and internal/metadataedit tests already share into the
// library dir — this package never writes to the shared fixture itself.
func setupReindexFixture(t *testing.T) (scanner.Config, *store.Store, tags.Reader, uint, string) {
	t.Helper()
	fx := "../../internal/tags/testdata/empty.flac"
	if _, err := os.Stat(fx); err != nil {
		t.Skipf("no fixture at %s: %v", fx, err)
	}
	data, err := os.ReadFile(fx)
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	dst := filepath.Join(root, "song.flac")
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatal(err)
	}

	st := newTestStore(t)
	lib := &model.Library{Name: "Main", Path: root}
	if err := st.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}

	reader := tags.NewFallbackReader(tags.TaglibReader{}, tags.FFProbeReader{})
	return scanner.Config{}, st, reader, lib.ID, dst
}

func countTracks(t *testing.T, s *store.Store, libID uint) int {
	t.Helper()
	var n int64
	s.DB().Model(&model.Track{}).Where("library_id = ?", libID).Count(&n)
	return int(n)
}
