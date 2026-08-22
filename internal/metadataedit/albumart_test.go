package metadataedit_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/metadataedit"
	"github.com/andresbott/aether/internal/tags"
)

// nopReader is a tags.Reader that reads nothing. Album.Matrix accepts a
// reader for interface symmetry with the rest of the package, but its
// resolution (embedded pictures via taglib, folder files via os.ReadDir)
// never calls it — these tests confirm that by never returning anything
// useful from it.
type nopReader struct{}

func (nopReader) CanRead(string) bool { return false }
func (nopReader) Read(context.Context, string) (tags.Metadata, error) {
	return tags.Metadata{}, nil
}

// mustMkdir is defined in tracks_test.go.

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// findSlot returns the named type+slot cell from a matrix, or nil.
func findSlot(matrix []metadataedit.TypeSlots, typeID, slot string) *metadataedit.SlotState {
	for _, ts := range matrix {
		if ts.Type.ID != typeID {
			continue
		}
		for i := range ts.Slots {
			if ts.Slots[i].Slot == slot {
				return &ts.Slots[i]
			}
		}
	}
	return nil
}

func TestAlbumMatrix_FolderArtAcrossDiscDirs(t *testing.T) {
	root := t.TempDir()
	// CD1 has cover.png; CD2 has none -> folder slot present, mixed=true.
	mustMkdir(t, filepath.Join(root, "CD1"))
	mustMkdir(t, filepath.Join(root, "CD2"))
	mustWrite(t, filepath.Join(root, "CD1", "01.flac"), "a")
	mustWrite(t, filepath.Join(root, "CD2", "01.flac"), "b")
	mustWrite(t, filepath.Join(root, "CD1", "cover.png"), "img")

	al, err := metadataedit.ResolveAlbum(root, []string{"CD1/01.flac", "CD2/01.flac"})
	if err != nil {
		t.Fatal(err)
	}
	got := al.Matrix(context.Background(), nopReader{})
	front := findSlot(got, "Front Cover", "folder")
	if front == nil {
		t.Fatalf("front/folder not present; matrix=%v", got)
	}
	if front.Detail != "cover.png" || !front.Mixed {
		t.Errorf("got detail=%q mixed=%v; want cover.png / mixed", front.Detail, front.Mixed)
	}
	if front.Source.RelPath != "CD1/cover.png" {
		t.Errorf("source relpath=%q; want CD1/cover.png", front.Source.RelPath)
	}
}

// TestAlbumMatrix_EmbeddedPresenceAndRepresentative confirms embedded
// presence is counted over every selected track and the cell's Source points
// at the first track (in selection order) that carries the picture.
func TestAlbumMatrix_EmbeddedPresenceAndRepresentative(t *testing.T) {
	src := "testdata/empty.flac"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no fixture at %s: %v", src, err)
	}
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "album"))
	one := filepath.Join(root, "album", "01.flac")
	two := filepath.Join(root, "album", "02.flac")
	copyFileForWriter(t, src, one)
	copyFileForWriter(t, src, two)
	if err := metadataedit.WriteEmbeddedPicture(two, "Media", []byte("img"), ""); err != nil {
		t.Fatal(err)
	}

	al, err := metadataedit.ResolveAlbum(root, []string{"album/01.flac", "album/02.flac"})
	if err != nil {
		t.Fatal(err)
	}
	got := al.Matrix(context.Background(), nopReader{})
	media := findSlot(got, "Media", "embedded")
	if media == nil {
		t.Fatalf("media/embedded not present; matrix=%v", got)
	}
	if media.PresentCount != 1 || media.TotalCount != 2 {
		t.Errorf("present=%d total=%d; want 1/2", media.PresentCount, media.TotalCount)
	}
	if media.Source.RelPath != "album/02.flac" || media.Source.Slot != "embedded" || media.Source.TypeID != "Media" {
		t.Errorf("source=%+v; want {album/02.flac embedded Media}", media.Source)
	}
}

func TestResolveAlbum_EmptyInputErrors(t *testing.T) {
	root := t.TempDir()
	if _, err := metadataedit.ResolveAlbum(root, nil); err == nil {
		t.Fatal("expected an error for a nil selection")
	}
	if _, err := metadataedit.ResolveAlbum(root, []string{}); err == nil {
		t.Fatal("expected an error for an empty selection")
	}
}

// TestResolveAlbum_TracksAndDirs confirms Tracks() is the per-track embedded
// fan-out (in selection order) and Dirs() the distinct parent directories
// (the folder fan-out), matching today's selectionPaths/distinctDirs.
func TestResolveAlbum_TracksAndDirs(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "CD1"))
	mustMkdir(t, filepath.Join(root, "CD2"))
	mustWrite(t, filepath.Join(root, "CD1", "01.flac"), "a")
	mustWrite(t, filepath.Join(root, "CD2", "01.flac"), "b")

	al, err := metadataedit.ResolveAlbum(root, []string{"CD1/01.flac", "CD2/01.flac"})
	if err != nil {
		t.Fatal(err)
	}
	wantTracks := []string{filepath.Join(root, "CD1", "01.flac"), filepath.Join(root, "CD2", "01.flac")}
	if tracks := al.Tracks(); len(tracks) != 2 || tracks[0] != wantTracks[0] || tracks[1] != wantTracks[1] {
		t.Errorf("tracks = %v, want %v", tracks, wantTracks)
	}
	wantDirs := []string{filepath.Join(root, "CD1"), filepath.Join(root, "CD2")}
	if dirs := al.Dirs(); len(dirs) != 2 || dirs[0] != wantDirs[0] || dirs[1] != wantDirs[1] {
		t.Errorf("dirs = %v, want %v", dirs, wantDirs)
	}
}

// TestResolveAlbum_DirectoryEntrySeedsFolderOnlyAlbum confirms a selection
// entry that names a directory (rather than a track file) contributes only
// to Dirs(), never to Tracks() — the mechanism the picture handlers use to
// seed "browse this empty/untracked folder" without a selected track.
func TestResolveAlbum_DirectoryEntrySeedsFolderOnlyAlbum(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "album"))
	mustWrite(t, filepath.Join(root, "album", "cover.png"), "img")

	al, err := metadataedit.ResolveAlbum(root, []string{"album"})
	if err != nil {
		t.Fatal(err)
	}
	if tracks := al.Tracks(); len(tracks) != 0 {
		t.Errorf("tracks = %v, want none: a bare directory selection has no tracks", tracks)
	}
	wantDir := filepath.Join(root, "album")
	if dirs := al.Dirs(); len(dirs) != 1 || dirs[0] != wantDir {
		t.Errorf("dirs = %v, want [%s]", dirs, wantDir)
	}
	got := al.Matrix(context.Background(), nopReader{})
	front := findSlot(got, "Front Cover", "folder")
	if front == nil || front.Detail != "cover.png" {
		t.Fatalf("front/folder = %v, want cover.png", front)
	}
}

// TestAlbumOpen_FolderAndEmbedded confirms Open resolves a folder Source to
// a file path (no bytes) and an embedded Source to bytes (no path), and that
// the package-level OpenSource gives the identical result without an Album.
func TestAlbumOpen_FolderAndEmbedded(t *testing.T) {
	src := "testdata/empty.flac"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no fixture at %s: %v", src, err)
	}
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "album"))
	mustWrite(t, filepath.Join(root, "album", "cover.png"), "cover-bytes")
	trackAbs := filepath.Join(root, "album", "01.flac")
	copyFileForWriter(t, src, trackAbs)
	if err := metadataedit.WriteEmbeddedPicture(trackAbs, "Front Cover", []byte("embedded-bytes"), ""); err != nil {
		t.Fatal(err)
	}

	al, err := metadataedit.ResolveAlbum(root, []string{"album/01.flac"})
	if err != nil {
		t.Fatal(err)
	}
	matrix := al.Matrix(context.Background(), nopReader{})

	folder := findSlot(matrix, "Front Cover", "folder")
	if folder == nil {
		t.Fatal("front cover folder slot not found")
	}
	data, filePath, fp, oerr := al.Open(folder.Source)
	if oerr != nil {
		t.Fatalf("Open(folder): %v", oerr)
	}
	if filePath == "" || len(data) != 0 {
		t.Errorf("folder Open: filePath=%q data=%d bytes, want a filePath and no data", filePath, len(data))
	}
	if fp == "" {
		t.Error("folder Open: empty fingerprint")
	}

	embedded := findSlot(matrix, "Front Cover", "embedded")
	if embedded == nil {
		t.Fatal("front cover embedded slot not found")
	}
	edata, efilePath, efp, eerr := al.Open(embedded.Source)
	if eerr != nil {
		t.Fatalf("Open(embedded): %v", eerr)
	}
	if efilePath != "" || len(edata) == 0 {
		t.Errorf("embedded Open: filePath=%q data=%d bytes, want no filePath and some data", efilePath, len(edata))
	}
	if efp == "" {
		t.Error("embedded Open: empty fingerprint")
	}

	// OpenSource without an Album gives the identical result.
	data2, filePath2, fp2, err2 := metadataedit.OpenSource(root, embedded.Source)
	if err2 != nil || filePath2 != "" || string(data2) != string(edata) || fp2 != efp {
		t.Errorf("OpenSource diverges from Album.Open: data2=%q filePath2=%q fp2=%q err=%v", data2, filePath2, fp2, err2)
	}
}

func TestSourceValuesDecodeRoundTrip(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "album"))
	mustWrite(t, filepath.Join(root, "album", "cover.png"), "img")

	want := metadataedit.Source{RelPath: "album/cover.png", Slot: "folder", TypeID: "Front Cover"}
	q := want.Values()
	if q.Get("file") != "album/cover.png" || q.Get("slot") != "folder" || q.Get("type") != "Front Cover" || q.Get("sv") == "" {
		t.Fatalf("Values() = %v", q)
	}
	abs, decoded, err := metadataedit.DecodeSource(root, q)
	if err != nil {
		t.Fatalf("DecodeSource: %v", err)
	}
	if decoded != want {
		t.Errorf("decoded = %+v, want %+v", decoded, want)
	}
	wantAbs := filepath.Join(root, "album", "cover.png")
	if abs != wantAbs {
		t.Errorf("abs = %q, want %q", abs, wantAbs)
	}
}

func TestDecodeSource_RejectsTraversal(t *testing.T) {
	root := t.TempDir()
	q := metadataedit.Source{RelPath: "../outside", Slot: "folder", TypeID: "Front Cover"}.Values()
	if _, _, err := metadataedit.DecodeSource(root, q); err == nil {
		t.Fatal("expected an error for a path escaping the library root")
	}
}

// TestAlbumDeleteFolderPicture_RemovesAcrossDirs confirms folder-art removal
// fans out across every directory the selection spans, independently per
// directory (mirroring the fan-out on save), and leaves other types alone.
func TestAlbumDeleteFolderPicture_RemovesAcrossDirs(t *testing.T) {
	root := t.TempDir()
	one, two := filepath.Join(root, "album", "CD1"), filepath.Join(root, "album", "CD2")
	mustMkdir(t, one)
	mustMkdir(t, two)
	mustWrite(t, filepath.Join(one, "01.flac"), "a")
	mustWrite(t, filepath.Join(two, "01.flac"), "b")
	mustWrite(t, filepath.Join(one, "back.jpg"), "back-one")
	mustWrite(t, filepath.Join(two, "back.jpg"), "back-two")
	mustWrite(t, filepath.Join(one, "cover.png"), "cover")

	al, err := metadataedit.ResolveAlbum(root, []string{"album/CD1/01.flac", "album/CD2/01.flac"})
	if err != nil {
		t.Fatal(err)
	}
	pt, ok := metadataedit.PictureTypeByID("Back Cover")
	if !ok {
		t.Fatal("Back Cover type missing from registry")
	}
	if err := al.DeleteFolderPicture(pt); err != nil {
		t.Fatalf("DeleteFolderPicture: %v", err)
	}
	if _, err := os.Stat(filepath.Join(one, "back.jpg")); !os.IsNotExist(err) {
		t.Error("back.jpg must be removed from CD1")
	}
	if _, err := os.Stat(filepath.Join(two, "back.jpg")); !os.IsNotExist(err) {
		t.Error("back.jpg must be removed from CD2")
	}
	if _, err := os.Stat(filepath.Join(one, "cover.png")); err != nil {
		t.Error("cover.png must survive a back-cover delete")
	}
}
