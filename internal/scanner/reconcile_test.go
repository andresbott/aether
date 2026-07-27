// internal/scanner/reconcile_test.go
package scanner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanner"
)

func TestReconcileRepointsCoverPathToANewFolderImage(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Artist/Album/01.mp3",
		"Artist/Album/cover.jpg",
	})
	seedLibrary(t, st, dir, nil)

	s := scanner.New(scanner.Config{}, st, fakeTagReader{})
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var album model.Album
	if err := st.DB().First(&album).Error; err != nil {
		t.Fatal(err)
	}
	if album.CoverPath != filepath.Join(dir, "Artist/Album/cover.jpg") {
		t.Fatalf("unexpected initial cover path: %q", album.CoverPath)
	}

	// The editor replaces cover.jpg with cover.png (WriteFolderPicture removes
	// the sibling variant), so the stored path now points at a missing file.
	if err := os.Remove(filepath.Join(dir, "Artist/Album/cover.jpg")); err != nil {
		t.Fatal(err)
	}
	createTestFiles(t, dir, []string{"Artist/Album/cover.png"})

	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.DB().First(&album, album.ID).Error; err != nil {
		t.Fatal(err)
	}
	if album.CoverPath != filepath.Join(dir, "Artist/Album/cover.png") {
		t.Fatalf("expected the cover path to be repointed, got %q", album.CoverPath)
	}
}

func TestReconcileClearsAVanishedCoverPath(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Artist/Album/01.mp3",
		"Artist/Album/cover.jpg",
	})
	seedLibrary(t, st, dir, nil)

	s := scanner.New(scanner.Config{}, st, fakeTagReader{})
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "Artist/Album/cover.jpg")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var album model.Album
	if err := st.DB().First(&album).Error; err != nil {
		t.Fatal(err)
	}
	if album.CoverPath != "" {
		t.Fatalf("expected the stale cover path to be cleared, got %q", album.CoverPath)
	}
}
