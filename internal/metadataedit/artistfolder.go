package metadataedit

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/andresbott/aether/internal/tags"
	"github.com/andresbott/aether/internal/unidecode"
)

// ArtistFolderFor resolves the artist folder for a selected folder: the folder
// itself when it is an artist folder, otherwise the nearest ancestor that is one.
// So selecting the artist folder, one of its albums, or a disc sub-folder like
// "CD 1" all resolve to the same artist folder. ok is false when no folder from
// absDir up to (but excluding) the library root qualifies.
func ArtistFolderFor(ctx context.Context, libRoot, absDir string, reader tags.Reader) (string, bool) {
	root := filepath.Clean(libRoot)
	dir := filepath.Clean(absDir)
	for dir != root && strings.HasPrefix(dir, root+string(filepath.Separator)) {
		if IsArtistFolder(ctx, dir, reader) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

// IsArtistFolder reports whether absDir is an artist folder: a folder whose
// immediate sub-folders hold an artist's albums. It is eligible when some track
// under an immediate sub-folder is tagged with an album artist OR a track artist
// that matches absDir's own name (normalized) — the same rule the scanner uses to
// attach a folder artist image, which probes both sets (recordArtistProbes). So a
// folder judged eligible is one where an artist.jpg written here would actually be
// picked up, while a genre/collection folder (whose name matches no credit) stays
// out. Checking the track artist too matters because many libraries name the
// folder after the performer without setting a separate album-artist tag.
//
// It reads at most one tag per immediate sub-folder and stops at the first match,
// so an artist folder usually costs a single tag read.
func IsArtistFolder(ctx context.Context, absDir string, reader tags.Reader) bool {
	want := unidecode.Normalize(filepath.Base(absDir))
	if want == "" {
		return false
	}
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if creditMatches(ctx, filepath.Join(absDir, e.Name()), want, reader) {
			return true
		}
	}
	return false
}

// creditMatches reads the first readable track under dir and reports whether any
// of its album-artist or track-artist values normalizes to want. Tracks of one
// album share these credits, so the first readable track decides.
func creditMatches(ctx context.Context, dir, want string, reader tags.Reader) bool {
	matched := false
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !reader.CanRead(path) {
			return nil
		}
		meta, rerr := reader.Read(ctx, path)
		if rerr != nil {
			return nil // unreadable; keep looking for a readable track
		}
		for _, name := range meta.AlbumArtist {
			if unidecode.Normalize(name) == want {
				matched = true
				return fs.SkipAll
			}
		}
		for _, name := range meta.Artist {
			if unidecode.Normalize(name) == want {
				matched = true
				return fs.SkipAll
			}
		}
		return fs.SkipAll // the first readable track settles this album
	})
	return matched
}

// FirstAudioPath returns the absolute path of the first readable audio file under
// absDir. The artist-image write uses it to rescan one representative track —
// enough for the scanner to re-probe the artist and detect a newly written folder
// image, without re-indexing the artist's whole discography.
func FirstAudioPath(absDir string, reader tags.Reader) (string, bool) {
	found := ""
	_ = filepath.WalkDir(absDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if reader.CanRead(path) {
			found = path
			return fs.SkipAll
		}
		return nil
	})
	return found, found != ""
}
