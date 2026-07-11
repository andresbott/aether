package metadataedit_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/metadataedit"
	"github.com/andresbott/aether/internal/tags"
)

type stubReader struct {
	byPath  map[string]tags.Metadata
	errPath string
}

func (s stubReader) CanRead(p string) bool {
	ext := filepath.Ext(p)
	return ext == ".mp3" || ext == ".flac" || ext == ".ogg"
}

func (s stubReader) Read(p string) (tags.Metadata, error) {
	if p == s.errPath {
		return tags.Metadata{}, errors.New("boom")
	}
	m, ok := s.byPath[p]
	if !ok {
		return tags.Metadata{}, errors.New("no fixture for " + p)
	}
	return m, nil
}

func TestListTracks_RecursiveAndFiltered(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "album"))
	touch(t, filepath.Join(root, "album", "01.flac"))
	touch(t, filepath.Join(root, "album", "02.mp3"))
	touch(t, filepath.Join(root, "album", "cover.jpg"))
	touch(t, filepath.Join(root, "notes.txt"))

	reader := stubReader{byPath: map[string]tags.Metadata{
		filepath.Join(root, "album", "01.flac"): {Title: "One", Artist: []string{"A"}, Album: "X", Year: 2020, MBArtistID: []string{"id-a"}},
		filepath.Join(root, "album", "02.mp3"):  {Title: "Two", Artist: []string{"A"}, Album: "X", Year: 2020},
	}}
	got, err := metadataedit.ListTracks(root, root, reader)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 audio files, got %d: %+v", len(got), got)
	}
	if got[0].Path != "album/01.flac" || got[1].Path != "album/02.mp3" {
		t.Fatalf("paths should be library-relative and sorted: %+v", got)
	}
	if got[0].Title != "One" || got[1].Title != "Two" {
		t.Fatalf("titles mismatch: %+v", got)
	}
	if len(got[0].MBArtistIDs) != 1 || got[0].MBArtistIDs[0] != "id-a" {
		t.Fatalf("MBArtistIDs not surfaced: %+v", got[0].MBArtistIDs)
	}
	if got[0].Error != "" {
		t.Fatalf("unexpected error on row 0: %q", got[0].Error)
	}
}

func TestListTracks_ReadErrorCapturedPerFile(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "bad.mp3"))
	reader := stubReader{errPath: filepath.Join(root, "bad.mp3")}
	got, err := metadataedit.ListTracks(root, root, reader)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 row, got %d", len(got))
	}
	if got[0].Error == "" {
		t.Fatal("expected Error populated")
	}
	if got[0].Title != "" {
		t.Fatal("expected empty Title when read errors")
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func touch(t *testing.T, p string) {
	t.Helper()
	if err := os.WriteFile(p, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
}
