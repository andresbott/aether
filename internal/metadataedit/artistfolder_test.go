package metadataedit_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/metadataedit"
	"github.com/andresbott/aether/internal/tags"
)

// fakeReader reads audio by extension and returns a fixed album artist for every
// file, so the artist-folder tests can drive the tag match without real files.
type fakeReader struct{ albumArtist string }

func (fakeReader) CanRead(p string) bool {
	e := strings.ToLower(filepath.Ext(p))
	return e == ".flac" || e == ".mp3"
}
func (r fakeReader) Read(context.Context, string) (tags.Metadata, error) {
	return tags.Metadata{AlbumArtist: []string{r.albumArtist}}, nil
}

func mkfile(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIsArtistFolder_TrueWhenAlbumArtistMatchesName(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "Radiohead", "OK Computer", "a.flac"))
	if !metadataedit.IsArtistFolder(
		context.Background(), filepath.Join(root, "Radiohead"), fakeReader{"Radiohead"}) {
		t.Fatal("expected Radiohead to be an artist folder")
	}
}

func TestIsArtistFolder_FalseWhenNameMismatch(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "Radiohead", "OK Computer", "a.flac"))
	// A genre/collection folder: its content is tagged with a different album
	// artist than the folder's own name.
	if metadataedit.IsArtistFolder(
		context.Background(), filepath.Join(root, "Radiohead"), fakeReader{"Someone Else"}) {
		t.Fatal("expected no match when the album artist differs from the folder name")
	}
}

func TestIsArtistFolder_FalseForAlbumFolder(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "Radiohead", "OK Computer", "a.flac"))
	// The album folder itself has no album sub-folders, so it is not an artist
	// folder even though its tracks match.
	if metadataedit.IsArtistFolder(
		context.Background(), filepath.Join(root, "Radiohead", "OK Computer"), fakeReader{"Radiohead"}) {
		t.Fatal("expected an album folder not to be an artist folder")
	}
}

// artistOnlyReader tags files with a track artist but no album artist — the
// common case in libraries that never set an album-artist tag.
type artistOnlyReader struct{ artist string }

func (artistOnlyReader) CanRead(p string) bool {
	e := strings.ToLower(filepath.Ext(p))
	return e == ".flac" || e == ".mp3"
}
func (r artistOnlyReader) Read(context.Context, string) (tags.Metadata, error) {
	return tags.Metadata{Artist: []string{r.artist}}, nil
}

func TestIsArtistFolder_MatchesTrackArtistWithoutAlbumArtist(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "Radiohead", "OK Computer", "a.flac"))
	if !metadataedit.IsArtistFolder(
		context.Background(), filepath.Join(root, "Radiohead"), artistOnlyReader{"Radiohead"}) {
		t.Fatal("expected match via track artist when the album-artist tag is absent")
	}
}

func TestIsArtistFolder_MatchesNormalized(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "Bjork", "Post", "a.flac"))
	if !metadataedit.IsArtistFolder(
		context.Background(), filepath.Join(root, "Bjork"), fakeReader{"Björk"}) {
		t.Fatal("expected normalization to match Björk against the Bjork folder")
	}
}

func TestArtistFolderFor_SelectedIsArtistFolder(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "Radiohead", "OK Computer", "a.flac"))
	dir, ok := metadataedit.ArtistFolderFor(
		context.Background(), root, filepath.Join(root, "Radiohead"), fakeReader{"Radiohead"})
	if !ok || dir != filepath.Join(root, "Radiohead") {
		t.Fatalf("got %q ok=%v; want the Radiohead folder", dir, ok)
	}
}

func TestArtistFolderFor_SelectedIsAlbum(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "Radiohead", "OK Computer", "a.flac"))
	dir, ok := metadataedit.ArtistFolderFor(
		context.Background(), root, filepath.Join(root, "Radiohead", "OK Computer"), fakeReader{"Radiohead"})
	if !ok || dir != filepath.Join(root, "Radiohead") {
		t.Fatalf("got %q ok=%v; want the Radiohead folder from an album", dir, ok)
	}
}

func TestArtistFolderFor_SelectedIsDiscSubfolder(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "Radiohead", "OK Computer", "CD 1", "a.flac"))
	dir, ok := metadataedit.ArtistFolderFor(
		context.Background(), root, filepath.Join(root, "Radiohead", "OK Computer", "CD 1"), fakeReader{"Radiohead"})
	if !ok || dir != filepath.Join(root, "Radiohead") {
		t.Fatalf("got %q ok=%v; want the Radiohead folder from a disc subfolder", dir, ok)
	}
}

func TestArtistFolderFor_NotFound(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "Rock", "OK Computer", "a.flac"))
	// The album artist ("Radiohead") matches no ancestor folder name up to root.
	if _, ok := metadataedit.ArtistFolderFor(
		context.Background(), root, filepath.Join(root, "Rock", "OK Computer"), fakeReader{"Radiohead"}); ok {
		t.Fatal("expected no artist folder when no ancestor name matches")
	}
}

func TestArtistFolderFor_RootNotEligible(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "Radiohead", "OK Computer", "a.flac"))
	if _, ok := metadataedit.ArtistFolderFor(
		context.Background(), root, root, fakeReader{"Radiohead"}); ok {
		t.Fatal("expected the library root itself to be ineligible")
	}
}

func TestFirstAudioPath(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "art", "alb", "a.flac"))
	p, ok := metadataedit.FirstAudioPath(filepath.Join(root, "art"), fakeReader{"x"})
	if !ok || filepath.Base(p) != "a.flac" {
		t.Fatalf("FirstAudioPath = %q, ok=%v; want .../a.flac", p, ok)
	}
}

func TestFirstAudioPath_NoneWhenEmpty(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok := metadataedit.FirstAudioPath(filepath.Join(root, "empty"), fakeReader{"x"}); ok {
		t.Fatal("expected no audio in an empty folder")
	}
}
