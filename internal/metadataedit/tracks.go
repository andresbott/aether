package metadataedit

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/andresbott/aether/internal/tags"
)

// Track is the DTO returned for each audio file discovered under a folder.
// Path is library-relative with '/' separators so it is stable across the API.
// Error is non-empty when the tag read failed; the remaining fields are zero
// values in that case.
type Track struct {
	Path             string
	Name             string
	Title            string
	Artists          []string
	AlbumArtists     []string
	Album            string
	Year             int
	DiscNumber       int
	DiscSubtitle     string
	Compilation      bool
	MBArtistIDs      []string
	MBAlbumArtistIDs []string
	MBReleaseID      string
	MBReleaseGroupID string
	Error            string
}

// ListTracks walks absDir recursively, calling reader.Read on every file for
// which reader.CanRead returns true. Paths in the result are relative to
// libRoot. Read failures are captured per-row (non-fatal) so the client can
// still see the file.
func ListTracks(libRoot, absDir string, reader tags.Reader) ([]Track, error) {
	cleanRoot := filepath.Clean(libRoot)
	var out []Track
	err := filepath.WalkDir(absDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable subtree silently
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != absDir {
				return fs.SkipDir
			}
			return nil
		}
		if !reader.CanRead(path) {
			return nil
		}
		row := Track{
			Name:             d.Name(),
			Path:             toForwardRel(cleanRoot, path),
			Artists:          []string{},
			AlbumArtists:     []string{},
			MBArtistIDs:      []string{},
			MBAlbumArtistIDs: []string{},
		}
		meta, rerr := reader.Read(path)
		if rerr != nil {
			row.Error = rerr.Error()
			out = append(out, row)
			return nil
		}
		row.Title = meta.Title
		if meta.Artist != nil {
			row.Artists = meta.Artist
		}
		if meta.AlbumArtist != nil {
			row.AlbumArtists = meta.AlbumArtist
		}
		if meta.MBArtistID != nil {
			row.MBArtistIDs = meta.MBArtistID
		}
		if meta.MBAlbumArtistID != nil {
			row.MBAlbumArtistIDs = meta.MBAlbumArtistID
		}
		row.Album = meta.Album
		row.MBReleaseID = meta.MBReleaseID
		row.MBReleaseGroupID = meta.MBReleaseGroupID
		row.Year = meta.Year
		row.DiscNumber = meta.DiscNumber
		row.DiscSubtitle = meta.DiscSubtitle
		row.Compilation = meta.Compilation
		out = append(out, row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func toForwardRel(root, abs string) string {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return abs
	}
	return filepath.ToSlash(rel)
}
