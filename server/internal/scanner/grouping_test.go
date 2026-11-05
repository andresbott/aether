package scanner_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/tags"
)

// groupingReader tags every file with the grouping set for its album folder.
type groupingReader struct {
	fakeTagReader
	byAlbum map[string]string
}

func (g groupingReader) Read(ctx context.Context, absPath string) (tags.Metadata, error) {
	m, err := g.fakeTagReader.Read(ctx, absPath)
	m.Grouping = g.byAlbum[filepath.Base(filepath.Dir(absPath))]
	return m, err
}

func TestScanAssignsAndUpdatesGroupings(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"A/Film 1/01.mp3",
		"A/Film 2/01.mp3",
		"A/Loose/01.mp3",
	})
	folder := seedFolder(dir, nil)

	byAlbum := map[string]string{"Film 1": "Lord of the Rings", "Film 2": "Lord of the Rings"}
	s := newScanner(t, st, groupingReader{byAlbum: byAlbum}, folder)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	albumGrouping := func() map[string]string {
		var albums []model.Album
		if err := st.DB().Preload("Grouping").Find(&albums).Error; err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, a := range albums {
			if a.Grouping != nil {
				out[a.Name] = a.Grouping.Name
			} else {
				out[a.Name] = ""
			}
		}
		return out
	}
	got := albumGrouping()
	if got["Film 1"] != "Lord of the Rings" || got["Film 2"] != "Lord of the Rings" || got["Loose"] != "" {
		t.Fatalf("groupings after first scan = %v", got)
	}

	// Re-tag one album and drop the tag from the other: the next scan follows,
	// and the emptied grouping row disappears.
	byAlbum["Film 1"] = "Middle-earth"
	delete(byAlbum, "Film 2")
	// A full scan re-reads every file, so the changed tags are picked up.
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	got = albumGrouping()
	if got["Film 1"] != "Middle-earth" || got["Film 2"] != "" {
		t.Fatalf("groupings after retag = %v", got)
	}
	var n int64
	st.DB().Model(&model.Grouping{}).Count(&n)
	if n != 1 {
		t.Fatalf("expected one grouping left, got %d", n)
	}
}
