package imagecache_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/imagecache"
)

// pngBytes builds a w×h test image with varied pixel content, so a resize
// produces something meaningfully different from the source.
func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x % 251), uint8(y % 241), uint8((x * y) % 239), 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode source png: %v", err)
	}
	return buf.Bytes()
}

func TestPathScalesSourceToRequestedSize(t *testing.T) {
	c := imagecache.New(t.TempDir())
	src := pngBytes(t, 800, 800)

	p, err := c.Path(imagecache.Request{
		Kind:   "album",
		Key:    "1",
		Name:   "cover",
		Size:   200,
		Format: imagecache.FormatWebP,
		Load:   func() ([]byte, error) { return src, nil },
	})
	if err != nil {
		t.Fatalf("Path: %v", err)
	}

	f, err := os.Open(p)
	if err != nil {
		t.Fatalf("open derivative: %v", err)
	}
	defer func() { _ = f.Close() }()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("decode derivative: %v", err)
	}
	if format != "webp" {
		t.Errorf("format = %q, want webp", format)
	}
	if cfg.Width != 200 || cfg.Height != 200 {
		t.Errorf("derivative = %dx%d, want 200x200", cfg.Width, cfg.Height)
	}
	// <name>.<source fingerprint>.<size>.<ext>
	if base := filepath.Base(p); !strings.HasPrefix(base, "cover.") || !strings.HasSuffix(base, ".200.webp") {
		t.Errorf("filename = %q, want cover.<fingerprint>.200.webp", base)
	}
}

// A 16:9 artist photo squashed into a square box looks wrong, so size is a
// bounding box: the long edge becomes size and the short edge scales with it.
func TestPathPreservesAspectRatio(t *testing.T) {
	c := imagecache.New(t.TempDir())
	src := pngBytes(t, 1600, 900)

	p, err := c.Path(imagecache.Request{
		Kind:   "artist",
		Key:    "7",
		Name:   "cover",
		Size:   200,
		Format: imagecache.FormatWebP,
		Load:   func() ([]byte, error) { return src, nil },
	})
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	cfg := decodeConfig(t, p)
	if cfg.Width != 200 || cfg.Height != 112 {
		t.Errorf("derivative = %dx%d, want 200x112 (16:9 fitted into a 200 box)", cfg.Width, cfg.Height)
	}
}

// Upscaling wastes bytes and invents detail: a 120px source asked for at 200
// stays 120.
func TestPathDoesNotUpscaleSmallSource(t *testing.T) {
	c := imagecache.New(t.TempDir())
	src := pngBytes(t, 120, 120)

	p, err := c.Path(imagecache.Request{
		Kind: "album", Key: "2", Name: "cover", Size: 200, Format: imagecache.FormatWebP,
		Load: func() ([]byte, error) { return src, nil },
	})
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	cfg := decodeConfig(t, p)
	if cfg.Width != 120 || cfg.Height != 120 {
		t.Errorf("derivative = %dx%d, want 120x120 (source not upscaled)", cfg.Width, cfg.Height)
	}
}

// Clients that cannot decode WebP get a JPEG derivative, cached side by side
// with the WebP one under the same entry.
func TestPathEncodesJPEGVariant(t *testing.T) {
	c := imagecache.New(t.TempDir())
	src := pngBytes(t, 800, 800)
	load := func() ([]byte, error) { return src, nil }

	webpPath, err := c.Path(imagecache.Request{
		Kind: "album", Key: "3", Name: "cover", Size: 200, Format: imagecache.FormatWebP, Load: load,
	})
	if err != nil {
		t.Fatalf("Path(webp): %v", err)
	}
	jpegPath, err := c.Path(imagecache.Request{
		Kind: "album", Key: "3", Name: "cover", Size: 200, Format: imagecache.FormatJPEG, Load: load,
	})
	if err != nil {
		t.Fatalf("Path(jpeg): %v", err)
	}

	if base := filepath.Base(jpegPath); !strings.HasSuffix(base, ".200.jpg") {
		t.Errorf("jpeg filename = %q, want cover.<fingerprint>.200.jpg", base)
	}
	if jpegPath == webpPath {
		t.Fatal("jpeg and webp derivatives share a path; they must not overwrite each other")
	}
	if _, err := os.Stat(webpPath); err != nil {
		t.Errorf("webp derivative gone after the jpeg one was built: %v", err)
	}
	if _, format, err := image.Decode(mustOpen(t, jpegPath)); err != nil || format != "jpeg" {
		t.Errorf("decode jpeg derivative: format=%q err=%v", format, err)
	}
}

// A cached derivative is reused: the second call must not re-read the source.
func TestPathReusesCachedDerivative(t *testing.T) {
	c := imagecache.New(t.TempDir())
	src := pngBytes(t, 800, 800)
	loads := 0
	req := imagecache.Request{
		Kind: "album", Key: "4", Name: "cover", Size: 200, Format: imagecache.FormatWebP,
		Load: func() ([]byte, error) { loads++; return src, nil },
	}

	first, err := c.Path(req)
	if err != nil {
		t.Fatalf("Path (first): %v", err)
	}
	second, err := c.Path(req)
	if err != nil {
		t.Fatalf("Path (second): %v", err)
	}
	if first != second {
		t.Errorf("paths differ across calls: %q vs %q", first, second)
	}
	if loads != 1 {
		t.Errorf("source loaded %d times, want 1 (second call must hit the cache)", loads)
	}
}

// A source that isn't a decodable image must surface an error rather than
// leaving a truncated or empty file behind for every later request to serve.
func TestPathRejectsUndecodableSource(t *testing.T) {
	root := t.TempDir()
	c := imagecache.New(root)

	_, err := c.Path(imagecache.Request{
		Kind: "album", Key: "5", Name: "cover", Size: 200, Format: imagecache.FormatWebP,
		Load: func() ([]byte, error) { return []byte("not an image"), nil },
	})
	if err == nil {
		t.Fatal("Path returned nil error for an undecodable source")
	}
	entries, _ := os.ReadDir(filepath.Join(root, "album", "5"))
	if len(entries) != 0 {
		t.Errorf("left %d file(s) behind after a failed encode, want none", len(entries))
	}
}

// The cover behind a stable URL changes (an upload lands, an image is removed
// and an older folder file resurfaces). A derivative keyed only by size would
// pin the old bitmap forever, so the source fingerprint is part of the key.
func TestPathRebuildsWhenSourceFingerprintChanges(t *testing.T) {
	c := imagecache.New(t.TempDir())
	tall := pngBytes(t, 400, 800)
	wide := pngBytes(t, 800, 400)

	first, err := c.Path(imagecache.Request{
		Kind: "album", Key: "6", Name: "cover", Size: 200, Format: imagecache.FormatWebP,
		Fingerprint: "v1",
		Load:        func() ([]byte, error) { return tall, nil },
	})
	if err != nil {
		t.Fatalf("Path (first): %v", err)
	}
	second, err := c.Path(imagecache.Request{
		Kind: "album", Key: "6", Name: "cover", Size: 200, Format: imagecache.FormatWebP,
		Fingerprint: "v2",
		Load:        func() ([]byte, error) { return wide, nil },
	})
	if err != nil {
		t.Fatalf("Path (second): %v", err)
	}

	if first == second {
		t.Fatal("a changed fingerprint reused the same derivative path; the stale image would be served")
	}
	if cfg := decodeConfig(t, second); cfg.Width != 200 || cfg.Height != 100 {
		t.Errorf("second derivative = %dx%d, want 200x100 (built from the new source)", cfg.Width, cfg.Height)
	}
}

// Stale derivatives of the same entry are swept when a new fingerprint lands,
// otherwise every cover change leaks a full set of size variants forever.
func TestPathRemovesDerivativesOfSupersededFingerprint(t *testing.T) {
	root := t.TempDir()
	c := imagecache.New(root)
	src := pngBytes(t, 800, 800)

	for _, size := range []int{80, 200} {
		if _, err := c.Path(imagecache.Request{
			Kind: "album", Key: "7", Name: "cover", Size: size, Format: imagecache.FormatWebP,
			Fingerprint: "v1", Load: func() ([]byte, error) { return src, nil },
		}); err != nil {
			t.Fatalf("Path(size=%d): %v", size, err)
		}
	}
	if _, err := c.Path(imagecache.Request{
		Kind: "album", Key: "7", Name: "cover", Size: 200, Format: imagecache.FormatWebP,
		Fingerprint: "v2", Load: func() ([]byte, error) { return src, nil },
	}); err != nil {
		t.Fatalf("Path(v2): %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(root, "album", "7"))
	if err != nil {
		t.Fatalf("read entry dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 1 {
		t.Errorf("entry holds %v, want only the v2 derivative", names)
	}
}

// Two entries of the same entity (front cover and back cover) are independent:
// rebuilding one must not sweep the other.
func TestPathKeepsOtherNamedEntries(t *testing.T) {
	root := t.TempDir()
	c := imagecache.New(root)
	src := pngBytes(t, 800, 800)
	load := func() ([]byte, error) { return src, nil }

	back, err := c.Path(imagecache.Request{
		Kind: "album", Key: "8", Name: "back", Size: 200, Format: imagecache.FormatWebP,
		Fingerprint: "b1", Load: load,
	})
	if err != nil {
		t.Fatalf("Path(back): %v", err)
	}
	if _, err := c.Path(imagecache.Request{
		Kind: "album", Key: "8", Name: "cover", Size: 200, Format: imagecache.FormatWebP,
		Fingerprint: "c1", Load: load,
	}); err != nil {
		t.Fatalf("Path(cover): %v", err)
	}
	if _, err := c.Path(imagecache.Request{
		Kind: "album", Key: "8", Name: "cover", Size: 200, Format: imagecache.FormatWebP,
		Fingerprint: "c2", Load: load,
	}); err != nil {
		t.Fatalf("Path(cover v2): %v", err)
	}

	if _, err := os.Stat(back); err != nil {
		t.Errorf("back-cover derivative swept by a front-cover rebuild: %v", err)
	}
}

// Deleting an entity's cached images is how an upload or a removal drops every
// derivative at once, without knowing which sizes were ever built.
func TestDeleteRemovesEveryDerivativeOfTheEntity(t *testing.T) {
	root := t.TempDir()
	c := imagecache.New(root)
	src := pngBytes(t, 800, 800)
	load := func() ([]byte, error) { return src, nil }

	for _, size := range []int{80, 200} {
		if _, err := c.Path(imagecache.Request{
			Kind: "artist", Key: "9", Name: "cover", Size: size, Format: imagecache.FormatWebP,
			Fingerprint: "v1", Load: load,
		}); err != nil {
			t.Fatalf("Path(size=%d): %v", size, err)
		}
	}
	kept, err := c.Path(imagecache.Request{
		Kind: "artist", Key: "10", Name: "cover", Size: 80, Format: imagecache.FormatWebP,
		Fingerprint: "v1", Load: load,
	})
	if err != nil {
		t.Fatalf("Path(other entity): %v", err)
	}

	if err := c.Delete("artist", "9"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "artist", "9")); !os.IsNotExist(err) {
		t.Errorf("entity dir still present after Delete: err=%v", err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("Delete removed another entity's derivative: %v", err)
	}
}

// Deleting an entity that has nothing cached is a no-op, not an error: callers
// invalidate on every cover change without tracking what was ever built.
func TestDeleteOnMissingEntitySucceeds(t *testing.T) {
	c := imagecache.New(t.TempDir())
	if err := c.Delete("artist", "404"); err != nil {
		t.Errorf("Delete on an uncached entity: %v", err)
	}
}

func TestFormatForAcceptPrefersWebPWhenSupported(t *testing.T) {
	tests := []struct {
		name   string
		accept string
		want   imagecache.Format
	}{
		{"explicit webp", "image/webp,image/apng,*/*", imagecache.FormatWebP},
		{"wildcard only", "*/*", imagecache.FormatJPEG},
		{"jpeg only", "image/jpeg", imagecache.FormatJPEG},
		{"absent header", "", imagecache.FormatJPEG},
		{"webp with quality value", "image/jpeg;q=0.9, image/webp;q=0.8", imagecache.FormatWebP},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := imagecache.FormatForAccept(tc.accept); got != tc.want {
				t.Errorf("FormatForAccept(%q) = %q, want %q", tc.accept, got, tc.want)
			}
		})
	}
}

func mustOpen(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func decodeConfig(t *testing.T, path string) image.Config {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return cfg
}
