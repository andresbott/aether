// Package imagecache stores display-sized, re-encoded copies of entity images
// so the server never ships a multi-megabyte original to a UI that renders it
// in an 80-pixel row.
package imagecache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gen2brain/webp"
	"golang.org/x/image/draw"

	"image/jpeg"

	// Source images are whatever the library and the providers hold; GIF and
	// BMP show up as folder art (see scanner.coverExts).
	_ "image/gif"
	_ "image/png"

	_ "golang.org/x/image/bmp"
)

// Format is the encoding of a cached derivative.
type Format string

const (
	FormatWebP Format = "webp"
	FormatJPEG Format = "jpg"
)

const (
	// Quality settings are a visual-quality/size compromise tuned for cover
	// art: WebP at 82 is visually indistinguishable from the source at display
	// sizes while landing well under a tenth of the original's bytes. Method 4
	// is libwebp's default speed/size balance — higher values buy a few percent
	// for several times the encode time, which matters because derivatives are
	// built inline on the first request for a size.
	webpQuality = 82
	webpMethod  = 4
	// JPEG is the fallback for clients that cannot decode WebP; 85 keeps it
	// visually comparable, at roughly 5x the WebP size.
	jpegQuality = 85
)

// Request identifies one derivative: the (kind, key, name) entry the source
// image belongs to, rendered at Size in Format.
type Request struct {
	Kind   string
	Key    string
	Name   string
	Size   int
	Format Format
	// Fingerprint identifies the source image the derivative is built from —
	// typically path+size+mtime for a file, or a content hash for bytes. It is
	// part of the cache key, so a new source produces a new derivative instead
	// of serving the old one: cover URLs are stable while the image behind them
	// is not, and the replacement is not always newer.
	Fingerprint string
	// Load returns the source image bytes. Called only on a cache miss, so a
	// hit costs no file read of a possibly large original.
	Load func() ([]byte, error)
}

type Cache struct {
	root string
}

func New(root string) *Cache {
	return &Cache{root: root}
}

// Kind/key come from entity identities (assetstore keys, DB IDs, MBIDs); the
// same guard as assetstore keeps a crafted one from escaping the cache root.
var keyRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// Entry names are one dot-separated field of the derivative filename, so unlike
// keys they must not contain dots.
var nameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Path returns the filesystem path of the cached derivative, building it on
// demand. Building one supersedes the entry's derivatives of every other
// fingerprint, so a changed cover does not leak a set of stale size variants.
func (c *Cache) Path(req Request) (string, error) {
	dir, err := c.entryDir(req.Kind, req.Key)
	if err != nil {
		return "", err
	}
	if !nameRe.MatchString(req.Name) {
		return "", fmt.Errorf("imagecache: unsafe entry name %q", req.Name)
	}
	if req.Size <= 0 {
		return "", fmt.Errorf("imagecache: size must be > 0, got %d", req.Size)
	}
	stamp := fingerprintTag(req.Fingerprint)
	path := filepath.Join(dir, derivativeName(req.Name, stamp, req.Size, req.Format))
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	src, err := req.Load()
	if err != nil {
		return "", fmt.Errorf("imagecache: load source: %w", err)
	}
	data, err := encode(src, req.Size, req.Format)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", fmt.Errorf("imagecache: mkdir: %w", err)
	}
	if err := writeAtomic(dir, path, data); err != nil {
		return "", err
	}
	c.sweep(dir, req.Name, stamp)
	return path, nil
}

// Delete drops every cached derivative of an entity, in every entry, size and
// format. Used when the entity's source image changes in a way the fingerprint
// cannot see (a manual upload, a removal) and on entity deletion. Missing is
// not an error.
func (c *Cache) Delete(kind, key string) error {
	dir, err := c.entryDir(kind, key)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("imagecache: delete entity: %w", err)
	}
	return nil
}

// FormatForAccept picks the derivative format for a request's Accept header.
// WebP is a third to a fifth of JPEG's bytes at the same visual quality, so it
// wins whenever the client names it; anything else — including a bare `*/*`,
// which plenty of Subsonic clients send while having no WebP decoder — gets
// JPEG.
func FormatForAccept(accept string) Format {
	for _, part := range strings.Split(accept, ",") {
		mediaType, _, _ := strings.Cut(part, ";")
		if strings.EqualFold(strings.TrimSpace(mediaType), "image/webp") {
			return FormatWebP
		}
	}
	return FormatJPEG
}

// ContentType is the MIME type a derivative in this format is served as.
func (f Format) ContentType() string {
	if f == FormatWebP {
		return "image/webp"
	}
	return "image/jpeg"
}

// derivativeName builds a derivative's filename: <name>.<stamp>.<size>.<ext>,
// e.g. "back.3f9a1c2b.200.webp".
func derivativeName(name, stamp string, size int, format Format) string {
	return name + "." + stamp + "." + strconv.Itoa(size) + "." + string(format)
}

// fingerprintTag hashes a fingerprint into a short filename-safe tag. The
// fingerprint itself is a path or an mtime string, neither of which belongs in
// a filename.
func fingerprintTag(fingerprint string) string {
	sum := sha256.Sum256([]byte(fingerprint))
	return hex.EncodeToString(sum[:6])
}

// sweep removes the entry's derivatives built from superseded sources, in every
// size and format. Other named entries of the same entity are left alone.
func (c *Cache) sweep(dir, name, keepStamp string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		n, stamp, ok := splitDerivative(e.Name())
		if !ok || n != name || stamp == keepStamp {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
}

// splitDerivative parses a derivative filename back into its entry name and
// fingerprint tag. Anything else (a temp file, a stray file) returns ok=false
// so the sweep never removes it.
func splitDerivative(filename string) (name, stamp string, ok bool) {
	parts := strings.Split(filename, ".")
	if len(parts) != 4 {
		return "", "", false
	}
	return parts[0], parts[1], parts[0] != "" && parts[1] != ""
}

func (c *Cache) entryDir(kind, key string) (string, error) {
	if !keyRe.MatchString(kind) || !keyRe.MatchString(key) || strings.Contains(key, "..") {
		return "", fmt.Errorf("imagecache: unsafe kind/key %q/%q", kind, key)
	}
	return filepath.Join(c.root, kind, key), nil
}

// encode decodes src, scales it to fit a size×size box, and re-encodes it in
// format.
func encode(src []byte, size int, format Format) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("imagecache: decode source: %w", err)
	}
	dst := scale(img, size)
	var buf bytes.Buffer
	switch format {
	case FormatJPEG:
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jpegQuality})
	case FormatWebP:
		err = webp.Encode(&buf, dst, webp.Options{Quality: webpQuality, Method: webpMethod})
	default:
		return nil, fmt.Errorf("imagecache: unknown format %q", format)
	}
	if err != nil {
		return nil, fmt.Errorf("imagecache: encode %s: %w", format, err)
	}
	return buf.Bytes(), nil
}

// scale resizes img to fit inside a size×size box, keeping its aspect ratio:
// the long edge becomes size and the short edge follows. Squashing a 16:9
// artist photo into a square would distort it.
func scale(img image.Image, size int) image.Image {
	b := img.Bounds()
	w, h := fit(b.Dx(), b.Dy(), size)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}

// fit returns the dimensions of a srcW×srcH image scaled to fit a size box,
// never smaller than a single pixel on either edge. A source already smaller
// than the box is left at its own size: upscaling only invents detail and costs
// bytes.
func fit(srcW, srcH, size int) (int, int) {
	if srcW <= 0 || srcH <= 0 {
		return size, size
	}
	if srcW <= size && srcH <= size {
		return srcW, srcH
	}
	w, h := size, size
	if srcW > srcH {
		h = srcH * size / srcW
	} else if srcH > srcW {
		w = srcW * size / srcH
	}
	return max(w, 1), max(h, 1)
}

// writeAtomic writes data to a temp file in dir and renames it over path, so a
// concurrent reader never sees a partially written image.
func writeAtomic(dir, path string, data []byte) error {
	tmp, err := os.CreateTemp(dir, "img-*.tmp")
	if err != nil {
		return fmt.Errorf("imagecache: temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("imagecache: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("imagecache: close: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("imagecache: rename: %w", err)
	}
	return nil
}
