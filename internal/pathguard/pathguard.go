// Package pathguard answers one question: is this absolute path inside one of a
// set of allowed root directories?
//
// It exists because the media handlers serve files whose paths come from the
// database rather than from the request — a track's file_path, an album's
// cover_path. Nothing enforces that those rows point inside a configured
// library, so a stale row, a hand-edited one, or a metadata-editor bug can name
// any file the server process can read. This turns that assumption into an
// enforced check.
//
// metadataedit.ResolveInLibrary solves the adjacent problem — joining a
// request-supplied *relative* path onto a root without escaping it. This one
// takes an already-absolute path of unknown provenance and asks where it lives.
package pathguard

import (
	"path/filepath"
	"strings"
)

// Guard holds a set of allowed root directories.
type Guard struct {
	roots []string
}

// New returns a Guard allowing paths inside any of roots. Empty roots are
// ignored; a Guard with no usable roots allows nothing, because failing open
// would disable the check exactly when the configuration is broken.
func New(roots ...string) *Guard {
	g := &Guard{roots: make([]string, 0, len(roots))}
	for _, r := range roots {
		if r == "" {
			continue
		}
		g.roots = append(g.roots, resolve(r))
	}
	return g
}

// Allows reports whether path lies inside one of the Guard's roots.
func (g *Guard) Allows(path string) bool {
	if !filepath.IsAbs(path) {
		return false
	}
	resolved := resolve(path)
	for _, root := range g.roots {
		if contains(root, resolved) {
			return true
		}
	}
	return false
}

// Within reports whether path lies inside root. Both are resolved through
// symlinks first, so a link pointing out of the root does not smuggle a file
// past the check while a link that stays inside keeps working.
func Within(root, path string) bool {
	if root == "" || !filepath.IsAbs(path) || !filepath.IsAbs(root) {
		return false
	}
	return contains(resolve(root), resolve(path))
}

// resolve cleans path and follows symlinks as far as they exist. A path that
// does not exist yet cannot be resolved, so the deepest existing ancestor is
// resolved and the remaining (purely lexical, already cleaned) suffix is
// rejoined — enough to keep a `..` from escaping while still working for files
// that were deleted between the scan and the request.
func resolve(path string) string {
	clean := filepath.Clean(path)
	if r, err := filepath.EvalSymlinks(clean); err == nil {
		return r
	}
	dir, base := filepath.Split(clean)
	parent := filepath.Clean(dir)
	if parent == clean {
		// Reached the filesystem root without resolving anything.
		return clean
	}
	return filepath.Join(resolve(parent), base)
}

// contains reports whether path is root or lives beneath it. Both arguments must
// already be cleaned and absolute. The separator check is what makes this a path
// comparison rather than a string-prefix one: /music must not contain
// /music-private.
func contains(root, path string) bool {
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
}
