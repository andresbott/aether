// Package artistimage fetches artist images from external providers, keyed by
// the MusicBrainz artist MBID, behind a small Provider interface, and also
// locates the folder that represents a music artist on disk and the best portrait
// image inside it. It recognises the <collection>/<artist>/<album>[/<disc>] layout
// at any depth: an artist folder is a strict ancestor of the track's own directory
// whose basename matches the artist name. Matching is by name and position only —
// never an album's own folder art — so a library laid out some other way yields
// nothing rather than a wrong image.
//
// It has no scanner or store dependencies, so callers outside the scanner (e.g.
// the metadata editor, to create an artist image file) can reuse it.
package artistimage

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/time/rate"

	"github.com/andresbott/aether/internal/unidecode"
)

// requestsPerSecond is the fair-use rate limit applied per provider to its
// outbound API and image-download requests (burst 1).
const requestsPerSecond rate.Limit = 1

// ImageCandidate is one portrait a provider offers for an artist. FullURL is the
// image stored on commit; ThumbURL is a lighter preview variant the grid loads.
type ImageCandidate struct {
	FullURL  string
	ThumbURL string
	Provider string // producing Provider.Name(); routes Chain.Download
}

type Provider interface {
	// List returns the provider's portrait candidates for the MBID, in the
	// order the provider returns them, or nil when it has none.
	List(ctx context.Context, mbid string) ([]ImageCandidate, error)
	// Download fetches the bytes of a URL this provider listed.
	Download(ctx context.Context, url string) ([]byte, string, error)
	Name() string
}

type Chain struct {
	providers []Provider
}

func NewChain(ps ...Provider) *Chain { return &Chain{providers: ps} }

func (c *Chain) List(ctx context.Context, mbid string) ([]ImageCandidate, error) {
	var all []ImageCandidate
	var lastErr error
	for _, p := range c.providers {
		cs, err := p.List(ctx, mbid)
		if err != nil {
			lastErr = err // a provider that errors is skipped, not fatal…
			continue
		}
		all = append(all, cs...)
	}
	if len(all) == 0 && lastErr != nil {
		return nil, lastErr // …unless nobody produced anything, then surface it
	}
	return all, nil
}

func (c *Chain) Download(ctx context.Context, providerName, url string) ([]byte, string, error) {
	for _, p := range c.providers {
		if p.Name() == providerName {
			return p.Download(ctx, url)
		}
	}
	return nil, "", fmt.Errorf("artistimage: no provider named %q", providerName)
}

// Fetch keeps the one-shot contract the auto-fetch job and setMBID rely on:
// list candidates and download the first that succeeds, falling through to
// the next candidate — possibly from a different provider — when a download
// fails or comes back empty. fanart.tv lists first, so its top thumb is still
// tried first.
func (c *Chain) Fetch(ctx context.Context, mbid string) ([]byte, string, error) {
	cands, err := c.List(ctx, mbid)
	if err != nil {
		return nil, "", err
	}
	var lastErr error
	for _, cand := range cands {
		data, ext, derr := c.Download(ctx, cand.Provider, cand.FullURL)
		if derr != nil {
			lastErr = derr // this candidate's image download failed; try the next
			continue
		}
		if len(data) > 0 {
			return data, ext, nil
		}
	}
	return nil, "", lastErr // nil when there were no candidates at all
}

// extFromURL derives a normalized image extension from a URL, defaulting to jpg.
func extFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "jpg"
	}
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(u.Path), "."))
	if ext == "jpeg" {
		ext = "jpg"
	}
	if ext != "jpg" && ext != "png" {
		ext = "jpg"
	}
	return ext
}

// ==============================================================================
// Local file-based artist image detection (new in this refactor)
// ==============================================================================

// imageExts are the file extensions treated as images.
var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true, ".webp": true,
}

// namePriority names the files that represent an artist rather than a release.
// Matches are exact (extension aside) — a substring rule would turn "the artist
// live.jpg" into the band's portrait. Lower is better.
var namePriority = map[string]int{
	"artist":      0,
	"artistthumb": 1,
	"folder":      2,
}

// BestFilename returns the highest-priority artist-image filename from filenames,
// or "" when none qualifies.
func BestFilename(filenames []string) string {
	best := ""
	bestPri := -1
	for _, f := range filenames {
		pri := rank(f)
		if pri < 0 {
			continue
		}
		if best == "" || pri < bestPri {
			bestPri = pri
			best = f
		}
	}
	return best
}

// IsUsablePath reports whether path still works as an artist image: the file must
// exist and its name must be one of the artist-image names. Used to re-check a
// path already on record instead of trusting it forever.
func IsUsablePath(path string) bool {
	if path == "" || rank(filepath.Base(path)) < 0 {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// rank returns the match priority for filename (lower is better), or -1 when the
// filename does not name an artist image.
func rank(filename string) int {
	ext := strings.ToLower(filepath.Ext(filename))
	if !imageExts[ext] {
		return -1
	}
	base := strings.ToLower(strings.TrimSuffix(filename, filepath.Ext(filename)))
	if pri, ok := namePriority[base]; ok {
		return pri
	}
	return -1
}

// BestInDir returns the full path of the best artist image directly inside dir,
// or "" when there is none.
func BestInDir(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	best := BestFilename(names)
	if best == "" {
		return ""
	}
	return filepath.Join(dir, best)
}

// artistDirs returns every strict ancestor of startDir (excluding startDir itself
// and the library root, never crossing above it) whose basename matches
// artistName, deepest first. startDir is a directory at or below the album level —
// typically the directory a track file sits in — so an album's own folder art is
// never mistaken for a portrait and a flat <artist>/<track> layout yields nothing.
func artistDirs(libRoot, startDir, artistName string) []string {
	want := unidecode.Normalize(artistName)
	if want == "" {
		return nil
	}
	root := filepath.Clean(libRoot)
	var out []string
	dir := filepath.Dir(filepath.Clean(startDir))
	for dir != root && strings.HasPrefix(dir, root+string(filepath.Separator)) {
		if unidecode.Normalize(filepath.Base(dir)) == want {
			out = append(out, dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return out
}

// FindDir returns the artist's folder for a track living under startDir — the
// nearest ancestor named after artistName — regardless of whether it holds an
// image yet. ok is false when the layout has no such folder. Callers that need to
// create an artist image (e.g. the metadata editor) use this to decide where to
// write it.
func FindDir(libRoot, startDir, artistName string) (string, bool) {
	dirs := artistDirs(libRoot, startDir, artistName)
	if len(dirs) == 0 {
		return "", false
	}
	return dirs[0], true
}

// Detect returns the path of the artist image found in the artist's folder for a
// track living under startDir, or "" when the layout has no artist folder or the
// folder holds no usable image. When several ancestors share the artist's name it
// returns the image from the nearest one that has any.
func Detect(libRoot, startDir, artistName string) string {
	for _, dir := range artistDirs(libRoot, startDir, artistName) {
		if img := BestInDir(dir); img != "" {
			return img
		}
	}
	return ""
}
