// folderdetect.go locates the folder that represents a music artist on disk and
// the best portrait image inside it. It recognises the
// <collection>/<artist>/<album>[/<disc>] layout at any depth: an artist folder is
// a strict ancestor of the track's own directory whose basename matches the
// artist name. Matching is by name and position only — never an album's own
// folder art — so a library laid out some other way yields nothing rather than a
// wrong image. It has no scanner or store dependencies, so callers outside the
// scanner (e.g. the metadata editor, to create an artist image file) can reuse it.
package artistimage

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/andresbott/aether/internal/unidecode"
)

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
