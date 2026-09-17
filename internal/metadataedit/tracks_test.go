package metadataedit_test

import (
	"context"
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

func (s stubReader) Read(_ context.Context, p string) (tags.Metadata, error) {
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
		filepath.Join(root, "album", "01.flac"): {Title: "One", Artist: []string{"A"}, Album: "X", Year: 2020, DiscNumber: 1, DiscSubtitle: "CD 1", MBArtistID: []string{"id-a"}, MBReleaseID: "rel-1", MBReleaseGroupID: "rg-1", ReleaseTypes: []string{"Album", "Compilation"}},
		filepath.Join(root, "album", "02.mp3"):  {Title: "Two", Artist: []string{"A"}, Album: "X", Year: 2020, DiscNumber: 2, DiscSubtitle: "CD 2"},
	}}
	got, err := metadataedit.ListTracks(context.Background(), root, root, reader)
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
	if got[0].MBReleaseID != "rel-1" || got[0].MBReleaseGroupID != "rg-1" {
		t.Fatalf("release IDs not surfaced on row 0: %+v", got[0])
	}
	if len(got[0].ReleaseTypes) != 2 || got[0].ReleaseTypes[0] != "Album" || got[0].ReleaseTypes[1] != "Compilation" {
		t.Fatalf("release types not surfaced on row 0: %+v", got[0].ReleaseTypes)
	}
	if got[0].DiscNumber != 1 || got[0].DiscSubtitle != "CD 1" {
		t.Fatalf("disc fields not surfaced on row 0: %+v", got[0])
	}
	if got[1].DiscNumber != 2 || got[1].DiscSubtitle != "CD 2" {
		t.Fatalf("disc fields not surfaced on row 1: %+v", got[1])
	}
	if got[0].Error != "" {
		t.Fatalf("unexpected error on row 0: %q", got[0].Error)
	}
}

func TestListTracks_ReadErrorCapturedPerFile(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "bad.mp3"))
	reader := stubReader{errPath: filepath.Join(root, "bad.mp3")}
	got, err := metadataedit.ListTracks(context.Background(), root, root, reader)
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

// wideStubReader can read a format Aether does not support (.mpc), mirroring
// the real ffprobe reader whose capability is wider than tags.Supported.
type wideStubReader struct{ byPath map[string]tags.Metadata }

func (wideStubReader) CanRead(p string) bool {
	ext := filepath.Ext(p)
	return ext == ".mp3" || ext == ".mpc"
}

func (r wideStubReader) Read(_ context.Context, p string) (tags.Metadata, error) {
	return r.byPath[p], nil
}

// TestListTracks_GatesOnSupportedNotReadable is the false-success fix: the
// editor must only offer files the scanner will index. A reader that can parse
// an unsupported format (.mpc) must not cause that file to be listed, or a user
// would edit it, get a green save, and the track would never appear.
func TestListTracks_GatesOnSupportedNotReadable(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "keep.mp3"))
	touch(t, filepath.Join(root, "drop.mpc"))

	reader := wideStubReader{byPath: map[string]tags.Metadata{
		filepath.Join(root, "keep.mp3"): {Title: "Keep"},
		filepath.Join(root, "drop.mpc"): {Title: "Drop"},
	}}
	got, err := metadataedit.ListTracks(context.Background(), root, root, reader)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected only the supported file listed, got %d: %+v", len(got), got)
	}
	if got[0].Path != "keep.mp3" {
		t.Fatalf("expected keep.mp3, got %q", got[0].Path)
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
