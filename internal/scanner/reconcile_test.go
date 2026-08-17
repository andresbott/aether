// internal/scanner/reconcile_test.go
package scanner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/tags"
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

// A folder that already carries a usable folder.jpg must still switch to a
// higher-ranked cover.jpg when one appears — the editor writes cover.<ext>, and
// nothing else repoints CoverPath now that the editor no longer touches the DB.
func TestReconcileRepointsToAHigherRankedCover(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Artist/Album/01.mp3",
		"Artist/Album/folder.jpg",
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
	if album.CoverPath != filepath.Join(dir, "Artist/Album/folder.jpg") {
		t.Fatalf("unexpected initial cover path: %q", album.CoverPath)
	}

	// folder.jpg stays in place and is still a perfectly usable cover, so the
	// old "only re-detect an unusable path" rule would keep serving it.
	createTestFiles(t, dir, []string{"Artist/Album/cover.jpg"})

	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.DB().First(&album, album.ID).Error; err != nil {
		t.Fatal(err)
	}
	if album.CoverPath != filepath.Join(dir, "Artist/Album/cover.jpg") {
		t.Fatalf("expected the cover path to repoint to cover.jpg, got %q", album.CoverPath)
	}
}

// Regression test: a multi-disc album spanning several directories must not
// lose its cover when a disc folder with no art reconciles. Albums are keyed
// on (name, albumArtist, mbReleaseID), not on directory, so "Album/CD 1/"
// holding cover.jpg and "Album/CD 2/" holding no art collapse to one album row.
// Whichever disc reconciles last must not blank the other's find.
func TestReconcilePreservesCoverAcrossMultiDiscAlbum(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Artist/Album/CD 1/01.mp3",
		"Artist/Album/CD 1/cover.jpg",
		"Artist/Album/CD 2/01.mp3",
	})
	seedLibrary(t, st, dir, nil)

	// Need a custom tag reader that returns the SAME album name for both discs,
	// so they collapse into one album row.
	albumName := "Multi Disc Album"
	reader := customAlbumTagReader{albumName: albumName}

	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	// Should be exactly one album row despite two directories.
	var albumCount int64
	if err := st.DB().Model(&model.Album{}).Count(&albumCount).Error; err != nil {
		t.Fatal(err)
	}
	if albumCount != 1 {
		t.Fatalf("expected 1 album (multi-disc), got %d", albumCount)
	}

	var album model.Album
	if err := st.DB().First(&album).Error; err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(dir, "Artist/Album/CD 1/cover.jpg")
	if album.CoverPath != want {
		t.Fatalf("expected CoverPath = %q, got %q (blanked by art-less sibling folder)", want, album.CoverPath)
	}
}

// customAlbumTagReader always returns the given album name regardless of directory.
type customAlbumTagReader struct {
	albumName string
}

func (r customAlbumTagReader) CanRead(absPath string) bool {
	return scanner.IsAudioFile(absPath)
}

func (r customAlbumTagReader) Read(_ context.Context, absPath string) (tags.Metadata, error) {
	return tags.Metadata{
		Title:       filepath.Base(absPath),
		Artist:      []string{"Test Artist"},
		AlbumArtist: []string{"Test Artist"},
		Album:       r.albumName,
		Genre:       []string{"Rock"},
		Year:        2020,
		TrackNumber: 1,
		Duration:    180,
		Bitrate:     320,
	}, nil
}
