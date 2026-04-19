// internal/scanner/walk_test.go
package scanner_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

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

	results, err := scanner.Walk([]scanner.MusicPath{{Path: dir}}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 audio files, got %d", len(results))
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
	results, err := scanner.Walk([]scanner.MusicPath{{Path: dir}}, excludes, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 file after exclude, got %d", len(results))
	}
}

func TestWalkMultiplePaths(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	createTestFiles(t, dir1, []string{"01.mp3"})
	createTestFiles(t, dir2, []string{"02.flac"})

	results, err := scanner.Walk([]scanner.MusicPath{
		{Path: dir1, Alias: "lib1"},
		{Path: dir2, Alias: "lib2"},
	}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 files, got %d", len(results))
	}
}
