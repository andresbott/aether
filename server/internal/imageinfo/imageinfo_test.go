package imageinfo

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestDescribePNG(t *testing.T) {
	data := pngOf(t, 3, 5)
	got := Describe(data)
	if got.Width != 3 || got.Height != 5 {
		t.Fatalf("dims = %dx%d, want 3x5", got.Width, got.Height)
	}
	if got.Format != "png" {
		t.Fatalf("format = %q, want png", got.Format)
	}
	if got.Bytes != int64(len(data)) {
		t.Fatalf("bytes = %d, want %d", got.Bytes, len(data))
	}
}

func TestDescribeJPEG(t *testing.T) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 4)), nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	got := Describe(buf.Bytes())
	if got.Width != 8 || got.Height != 4 || got.Format != "jpeg" {
		t.Fatalf("got %+v, want 8x4 jpeg", got)
	}
}

func TestDescribeGIF(t *testing.T) {
	var buf bytes.Buffer
	palette := color.Palette{color.Black, color.White}
	if err := gif.Encode(&buf, image.NewPaletted(image.Rect(0, 0, 2, 7), palette), nil); err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	got := Describe(buf.Bytes())
	if got.Width != 2 || got.Height != 7 || got.Format != "gif" {
		t.Fatalf("got %+v, want 2x7 gif", got)
	}
}

func TestDescribeNonImageReportsSizeOnly(t *testing.T) {
	data := []byte("this is not an image")
	got := Describe(data)
	if got.Format != "" || got.Width != 0 || got.Height != 0 {
		t.Fatalf("got %+v, want zero dims and empty format", got)
	}
	if got.Bytes != int64(len(data)) {
		t.Fatalf("bytes = %d, want %d", got.Bytes, len(data))
	}
}

func TestDescribeFile(t *testing.T) {
	data := pngOf(t, 6, 2)
	p := filepath.Join(t.TempDir(), "cover.png")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	got, err := DescribeFile(p)
	if err != nil {
		t.Fatalf("DescribeFile: %v", err)
	}
	if got.Width != 6 || got.Height != 2 || got.Format != "png" || got.Bytes != int64(len(data)) {
		t.Fatalf("got %+v, want 6x2 png %d bytes", got, len(data))
	}
}

func TestDescribeFileMissing(t *testing.T) {
	if _, err := DescribeFile(filepath.Join(t.TempDir(), "nope.png")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}
