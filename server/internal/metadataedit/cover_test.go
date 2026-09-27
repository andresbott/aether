package metadataedit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/metadataedit"
	"go.senan.xyz/taglib"
)

// a minimal 1x1 PNG
var pngBytes = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
	0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}

func TestWriteEmbeddedCover_RoundTrip(t *testing.T) {
	src := "testdata/empty.flac"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no fixture at %s: %v", src, err)
	}
	dst := filepath.Join(t.TempDir(), "copy.flac")
	copyFileForWriter(t, src, dst)

	if err := metadataedit.WriteEmbeddedCover(dst, pngBytes); err != nil {
		t.Fatalf("WriteEmbeddedCover: %v", err)
	}
	got, err := taglib.ReadImage(dst)
	if err != nil {
		t.Fatalf("ReadImage: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("no embedded image read back")
	}
}

func TestWriteFolderCover(t *testing.T) {
	dir := t.TempDir()

	path, err := metadataedit.WriteFolderCover(dir, "png", pngBytes)
	if err != nil {
		t.Fatalf("WriteFolderCover: %v", err)
	}
	if filepath.Base(path) != "cover.png" {
		t.Fatalf("want cover.png, got %s", path)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cover: %v", err)
	}
	if string(got) != string(pngBytes) {
		t.Fatal("cover bytes differ")
	}

	// Writing a jpg replaces the png (no stale sibling left behind).
	jpgPath, err := metadataedit.WriteFolderCover(dir, "jpg", []byte("jpeg"))
	if err != nil {
		t.Fatalf("WriteFolderCover jpg: %v", err)
	}
	if filepath.Base(jpgPath) != "cover.jpg" {
		t.Fatalf("want cover.jpg, got %s", jpgPath)
	}
	if _, err := os.Stat(filepath.Join(dir, "cover.png")); !os.IsNotExist(err) {
		t.Fatal("stale cover.png was not removed")
	}
}

func TestWriteFolderCover_NormalizesExt(t *testing.T) {
	dir := t.TempDir()
	path, err := metadataedit.WriteFolderCover(dir, "jpeg", []byte("x"))
	if err != nil {
		t.Fatalf("WriteFolderCover: %v", err)
	}
	if filepath.Base(path) != "cover.jpg" {
		t.Fatalf("want cover.jpg for jpeg ext, got %s", path)
	}
}
