package metadataedit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/metadataedit"
)

// a minimal 1x1 JPEG (just distinct bytes; taglib stores them opaquely)
var jpgBytes = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0xFF, 0xD9}

func pictureFixture(t *testing.T) string {
	t.Helper()
	src := "testdata/empty.flac"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no fixture at %s: %v", src, err)
	}
	dst := filepath.Join(t.TempDir(), "copy.flac")
	copyFileForWriter(t, src, dst)
	return dst
}

func embeddedTypes(t *testing.T, path string) []string {
	t.Helper()
	images, err := metadataedit.ListEmbeddedPictures(path)
	if err != nil {
		t.Fatalf("ListEmbeddedPictures: %v", err)
	}
	out := make([]string, 0, len(images))
	for _, img := range images {
		out = append(out, img.Type)
	}
	return out
}

func TestEmbeddedPicture_WriteReadByType(t *testing.T) {
	path := pictureFixture(t)

	if err := metadataedit.WriteEmbeddedPicture(path, "Front Cover", pngBytes, ""); err != nil {
		t.Fatalf("write front: %v", err)
	}
	if err := metadataedit.WriteEmbeddedPicture(path, "Back Cover", jpgBytes, ""); err != nil {
		t.Fatalf("write back: %v", err)
	}

	front, ok, err := metadataedit.ReadEmbeddedPicture(path, "Front Cover")
	if err != nil || !ok {
		t.Fatalf("read front: ok=%v err=%v", ok, err)
	}
	if string(front) != string(pngBytes) {
		t.Fatal("front bytes differ")
	}
	back, ok, err := metadataedit.ReadEmbeddedPicture(path, "Back Cover")
	if err != nil || !ok {
		t.Fatalf("read back: ok=%v err=%v", ok, err)
	}
	if string(back) != string(jpgBytes) {
		t.Fatal("back bytes differ")
	}
	if _, ok, _ := metadataedit.ReadEmbeddedPicture(path, "Media"); ok {
		t.Fatal("Media should be absent")
	}
}

func TestEmbeddedPicture_ReplaceKeepsOthers(t *testing.T) {
	path := pictureFixture(t)
	_ = metadataedit.WriteEmbeddedPicture(path, "Front Cover", pngBytes, "")
	_ = metadataedit.WriteEmbeddedPicture(path, "Back Cover", pngBytes, "")

	// Replace the front cover in place.
	if err := metadataedit.WriteEmbeddedPicture(path, "Front Cover", jpgBytes, ""); err != nil {
		t.Fatalf("replace front: %v", err)
	}
	got := embeddedTypes(t, path)
	if len(got) != 2 {
		t.Fatalf("want 2 pictures after replace, got %v", got)
	}
	front, _, _ := metadataedit.ReadEmbeddedPicture(path, "Front Cover")
	if string(front) != string(jpgBytes) {
		t.Fatal("front cover was not replaced")
	}
	back, ok, _ := metadataedit.ReadEmbeddedPicture(path, "Back Cover")
	if !ok || string(back) != string(pngBytes) {
		t.Fatal("back cover must survive a front-cover replace")
	}
}

func TestEmbeddedPicture_DeleteByType(t *testing.T) {
	path := pictureFixture(t)
	_ = metadataedit.WriteEmbeddedPicture(path, "Front Cover", pngBytes, "")
	_ = metadataedit.WriteEmbeddedPicture(path, "Back Cover", jpgBytes, "")

	if err := metadataedit.DeleteEmbeddedPicture(path, "Front Cover"); err != nil {
		t.Fatalf("delete front: %v", err)
	}
	got := embeddedTypes(t, path)
	if len(got) != 1 || got[0] != "Back Cover" {
		t.Fatalf("want only Back Cover left, got %v", got)
	}
	// Deleting an absent type is a no-op.
	if err := metadataedit.DeleteEmbeddedPicture(path, "Media"); err != nil {
		t.Fatalf("delete absent type: %v", err)
	}
}

func TestFolderPicture_WriteNameDelete(t *testing.T) {
	dir := t.TempDir()

	path, err := metadataedit.WriteFolderPicture(dir, "back", "png", pngBytes)
	if err != nil {
		t.Fatalf("WriteFolderPicture: %v", err)
	}
	if filepath.Base(path) != "back.png" {
		t.Fatalf("want back.png, got %s", path)
	}
	if got := metadataedit.FolderPictureName(dir, "back"); got != "back.png" {
		t.Fatalf("FolderPictureName: got %q", got)
	}
	if got := metadataedit.FolderPictureName(dir, "disc"); got != "" {
		t.Fatalf("absent base must return empty, got %q", got)
	}

	// Writing the sibling extension replaces the old file.
	if _, err := metadataedit.WriteFolderPicture(dir, "back", "jpeg", jpgBytes); err != nil {
		t.Fatalf("WriteFolderPicture jpg: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "back.png")); !os.IsNotExist(err) {
		t.Fatal("stale back.png was not removed")
	}
	if got := metadataedit.FolderPictureName(dir, "back"); got != "back.jpg" {
		t.Fatalf("FolderPictureName after replace: got %q", got)
	}

	if err := metadataedit.DeleteFolderPicture(dir, "back"); err != nil {
		t.Fatalf("DeleteFolderPicture: %v", err)
	}
	if got := metadataedit.FolderPictureName(dir, "back"); got != "" {
		t.Fatalf("picture should be gone, got %q", got)
	}
	// Deleting again is a no-op.
	if err := metadataedit.DeleteFolderPicture(dir, "back"); err != nil {
		t.Fatalf("second DeleteFolderPicture: %v", err)
	}
}

func TestPictureTypeByID(t *testing.T) {
	pt, ok := metadataedit.PictureTypeByID("Back Cover")
	if !ok || pt.FileBase != "back" {
		t.Fatalf("Back Cover: ok=%v pt=%+v", ok, pt)
	}
	if _, ok := metadataedit.PictureTypeByID("Nope"); ok {
		t.Fatal("unknown type must not resolve")
	}
	front, _ := metadataedit.PictureTypeByID("Front Cover")
	if front.FileBase != "cover" {
		t.Fatalf("Front Cover FileBase must stay 'cover' (subsonic serving), got %q", front.FileBase)
	}
}
