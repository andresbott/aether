package metadataedit

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Folder describes one subdirectory of a resolved directory.
type Folder struct {
	Name          string
	Path          string // library-relative path, forward-slash form
	HasSubfolders bool
}

// ListFolders returns immediate subdirectories of absPath, sorted by name.
// Hidden entries (leading '.') and non-directory entries are filtered out.
// The Path field is relative to the directory passed in and uses '/' so it
// round-trips cleanly through the HTTP API.
func ListFolders(absPath string) ([]Folder, error) {
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}
	out := make([]Folder, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		out = append(out, Folder{
			Name:          e.Name(),
			Path:          e.Name(),
			HasSubfolders: hasSubdirs(filepath.Join(absPath, e.Name())),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// hasSubdirs reports whether dir contains at least one visible subdirectory.
// Errors are swallowed and reported as "no" to keep the UX simple.
func hasSubdirs(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			return true
		}
	}
	return false
}
