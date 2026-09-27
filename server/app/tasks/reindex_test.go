// app/tasks/reindex_test.go
package tasks

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
)

func TestNewReindexTaskFnIndexesPaths(t *testing.T) {
	// Arrange: a store with one scan folder whose directory holds one audio
	// file that is NOT yet in the index. (Reuse the scanner package's test
	// fixtures / helpers for building store+folder+file; see
	// internal/scanner/*_test.go.)
	cfg, store, reader, folder, audioPath := setupReindexFixture(t)

	fn := NewReindexTaskFn(cfg, store, reader)
	if err := fn(context.Background(), slog.Default(), ReindexParams{ScanFolder: folder, Paths: []string{audioPath}}); err != nil {
		t.Fatalf("reindex fn: %v", err)
	}

	if n := countTracks(t, store, folder); n != 1 {
		t.Fatalf("track not indexed: got %d tracks, want 1", n)
	}
}

// setupReindexFixture builds a store with one scan folder whose directory
// holds one real (tag-less) audio file, plus a real tags.Reader — so the
// reindex task's RescanPaths call exercises actual tag reading end to end (a
// stub reader would not catch a wiring break between the task, the scanner,
// and a real Reader).
//
// It reuses newTestStore (defined in helpers_test.go) for the
// migrated in-memory store, and copies the same testdata/empty.flac fixture
// internal/tags and internal/metadataedit tests already share into the
// scan folder — this package never writes to the shared fixture itself.
func setupReindexFixture(t *testing.T) (scanner.Config, *store.Store, tags.Reader, string, string) {
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
	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Main", Path: root, FollowSymlinks: true}})
	if err != nil {
		t.Fatal(err)
	}

	reader := tags.NewFallbackReader(tags.TaglibReader{}, tags.FFProbeReader{})
	return scanner.Config{Folders: set}, st, reader, "Main", dst
}

func countTracks(t *testing.T, s *store.Store, scanFolder string) int {
	t.Helper()
	var n int64
	s.DB().Model(&model.Track{}).Where("scan_folder = ?", scanFolder).Count(&n)
	return int(n)
}
