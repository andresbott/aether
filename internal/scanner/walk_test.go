// internal/scanner/walk_test.go
package scanner_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanner"
)

func createTestFiles(t *testing.T, dir string, files []string) {
	t.Helper()
	for _, f := range files {
		p := filepath.Join(dir, f)
		_ = os.MkdirAll(filepath.Dir(p), 0750)
		_ = os.WriteFile(p, []byte("fake"), 0644)
	}
}

func TestWalk(t *testing.T) {
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"artist1/album1/01.mp3",
		"artist1/album1/02.flac",
		"artist1/album1/cover.jpg",
		"artist1/album1/notes.txt",
		"artist2/album2/01.ogg",
	})

	results, err := scanner.Walk([]model.Library{{ID: 1, Path: dir}}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 audio files, got %d", len(results))
	}
	for _, r := range results {
		if r.LibraryID != 1 {
			t.Fatalf("expected LibraryID=1, got %d", r.LibraryID)
		}
	}
}

func TestWalkExcludePattern(t *testing.T) {
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"music/01.mp3",
		".hidden/02.mp3",
		"music/Thumbs.db",
	})

	excludes := []*regexp.Regexp{regexp.MustCompile(`^\..`)}
	results, err := scanner.Walk([]model.Library{{ID: 1, Path: dir}}, excludes, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 file after exclude, got %d", len(results))
	}
}

func TestWalkMultipleLibraries(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	createTestFiles(t, dir1, []string{"01.mp3"})
	createTestFiles(t, dir2, []string{"02.flac"})

	results, err := scanner.Walk([]model.Library{
		{ID: 1, Path: dir1},
		{ID: 2, Path: dir2},
	}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 files, got %d", len(results))
	}
	byLib := map[uint]string{}
	for _, r := range results {
		byLib[r.LibraryID] = filepath.Base(r.FilePath)
	}
	if byLib[1] != "01.mp3" {
		t.Fatalf("LibraryID=1 should map to 01.mp3, got %q", byLib[1])
	}
	if byLib[2] != "02.flac" {
		t.Fatalf("LibraryID=2 should map to 02.flac, got %q", byLib[2])
	}
}
