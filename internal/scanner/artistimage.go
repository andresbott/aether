// internal/scanner/artistimage.go
package scanner

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/andresbott/aether/internal/unidecode"
)

// artistImagePriority names the files that represent an artist rather than a
// release. Matches are exact (extension aside) — a substring rule would turn
// "the artist live.jpg" into the band's portrait. Lower is better.
var artistImagePriority = map[string]int{
	"artist":      0,
	"artistthumb": 1,
	"folder":      2,
}

// BestArtistImage returns the highest-priority artist-image filename from
// filenames, or "" when none qualifies.
func BestArtistImage(filenames []string) string {
	best := ""
	bestPri := -1
	for _, f := range filenames {
		pri := artistImageRank(f)
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

// IsUsableArtistImagePath reports whether path still works as an artist image:
// the file must exist and its name must be one of the artist-image names. Used
// to re-check a path already on record instead of trusting it forever.
func IsUsableArtistImagePath(path string) bool {
	if path == "" || artistImageRank(filepath.Base(path)) < 0 {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// artistImageRank returns the match priority for filename (lower is better), or
// -1 when the filename does not name an artist image.
func artistImageRank(filename string) int {
	ext := strings.ToLower(filepath.Ext(filename))
	if !coverExts[ext] {
		return -1
	}
	base := strings.ToLower(strings.TrimSuffix(filename, filepath.Ext(filename)))
	if pri, ok := artistImagePriority[base]; ok {
		return pri
	}
	return -1
}

// DetectArtistImage looks for an image representing artistName in the directory
// tree between the track and the library root. A directory only qualifies as
// the artist folder when it is **both** positioned above the track's own
// directory — so a `<collection>/<artist>/<album>` layout is required and an
// album's own `folder.jpg` is never taken for an artist portrait — **and**
// named after the artist. Libraries laid out some other way simply yield "",
// rather than an image of whoever the parent directory happens to be.
//
// libRoot itself is never a candidate, and the search never walks above it.
func DetectArtistImage(libRoot, trackPath, artistName string) string {
	wantName := unidecode.Normalize(artistName)
	if wantName == "" {
		return ""
	}
	root := filepath.Clean(libRoot)
	// Start above the track's own directory: that one is the album directory.
	dir := filepath.Dir(filepath.Dir(filepath.Clean(trackPath)))
	for dir != root && strings.HasPrefix(dir, root+string(filepath.Separator)) {
		if unidecode.Normalize(filepath.Base(dir)) == wantName {
			if img := bestArtistImageInDir(dir); img != "" {
				return img
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// bestArtistImageInDir returns the full path of the best artist image directly
// inside dir, or "" when there is none.
func bestArtistImageInDir(dir string) string {
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
	best := BestArtistImage(names)
	if best == "" {
		return ""
	}
	return filepath.Join(dir, best)
}
