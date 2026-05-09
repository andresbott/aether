// Package metadataedit provides library-relative filesystem operations
// and tag read/write for the admin metadata editor. It deliberately
// avoids any coupling to the scanner, store (tracks/albums/artists),
// or player code so it can be extracted into a standalone tool.
package metadataedit

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ErrOutsideLibrary is returned when a library-relative path resolves
// to a location outside the library root.
var ErrOutsideLibrary = errors.New("path resolves outside library root")

// ResolveInLibrary takes a library root (absolute) and a relative path
// (possibly empty) and returns the absolute resolved path, rejecting
// anything that escapes the root. An absolute `rel` is always rejected.
func ResolveInLibrary(libRoot, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("%w: %q is absolute", ErrOutsideLibrary, rel)
	}
	cleanRoot := filepath.Clean(libRoot)
	if rel == "" {
		// Empty rel cannot traverse; cleanRoot is the library root itself.
		return cleanRoot, nil
	}
	joined := filepath.Clean(filepath.Join(cleanRoot, rel))
	// Using filepath.Rel guards against symlink-free traversal.
	relResolved, err := filepath.Rel(cleanRoot, joined)
	if err != nil {
		return "", fmt.Errorf("resolve: %w", err)
	}
	if relResolved == ".." || strings.HasPrefix(relResolved, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", ErrOutsideLibrary, rel)
	}
	return joined, nil
}
