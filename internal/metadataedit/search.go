package metadataedit

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// SearchFolders walks absRoot recursively and returns every folder whose name
// (its last path segment) contains query, matched case-insensitively. Paths are
// library-relative and forward-slash, exactly as ListFolders reports them, so a
// result round-trips back through the folder API. Results are sorted by path.
// A blank query matches nothing.
//
// When more than limit folders match, the walk stops early and truncated is
// true; limit <= 0 means no cap. Hidden directories are skipped unless
// opts.IncludeHidden is set. Symlinked directories are never traversed
// (filepath.WalkDir does not follow them), which keeps the walk lexically
// confined to the library root — the same confinement ListFolders relies on.
func SearchFolders(absRoot, query string, opts ListFoldersOptions, limit int) ([]Folder, bool, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil, false, nil
	}
	var out []Folder
	truncated := false
	err := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip an unreadable subtree silently, like ListTracks
		}
		if path == absRoot || !d.IsDir() {
			return nil
		}
		if !opts.IncludeHidden && isHidden(d.Name()) {
			return fs.SkipDir
		}
		if !strings.Contains(strings.ToLower(d.Name()), q) {
			return nil
		}
		rel, rerr := filepath.Rel(absRoot, path)
		if rerr != nil {
			return nil
		}
		out = append(out, Folder{
			Name:          d.Name(),
			Path:          filepath.ToSlash(rel),
			HasSubfolders: hasSubdirs(path, opts),
		})
		if limit > 0 && len(out) >= limit {
			truncated = true
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, truncated, nil
}
