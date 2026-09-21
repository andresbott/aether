// Package metadataedit provides folder-relative filesystem operations
// and tag read/write for the admin metadata editor. It deliberately
// avoids any coupling to the store (tracks/albums/artists) or player
// code, and to the scanner beyond its front-cover filename heuristic
// (scanner.BestCover, a one-way dependency: the scanner does not import
// this package), so it can be extracted into a standalone tool.
package metadataedit

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ErrOutsideRoot is returned when a folder-relative path resolves
// to a location outside the scan folder root.
var ErrOutsideRoot = errors.New("path resolves outside the scan folder")

// ResolveInRoot takes a root (absolute) and a relative path
// (possibly empty) and returns the absolute resolved path, rejecting
// anything that escapes the root. An absolute `rel` is always rejected.
func ResolveInRoot(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("%w: %q is absolute", ErrOutsideRoot, rel)
	}
	cleanRoot := filepath.Clean(root)
	if rel == "" {
		// Empty rel cannot traverse; cleanRoot is the root itself.
		return cleanRoot, nil
	}
	joined := filepath.Clean(filepath.Join(cleanRoot, rel))
	// Using filepath.Rel guards against symlink-free traversal.
	relResolved, err := filepath.Rel(cleanRoot, joined)
	if err != nil {
		return "", fmt.Errorf("resolve: %w", err)
	}
	if relResolved == ".." || strings.HasPrefix(relResolved, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", ErrOutsideRoot, rel)
	}
	return joined, nil
}
