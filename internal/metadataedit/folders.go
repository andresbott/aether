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
	IsSymlink     bool // the entry is a symlink that resolves to a directory
}

// ListFoldersOptions tunes which entries ListFolders returns. The zero value is
// the conservative choice: real directories only, hidden ones omitted.
type ListFoldersOptions struct {
	// IncludeHidden keeps dot-directories in the listing.
	IncludeHidden bool
	// IncludeSymlinks reports symlinks that resolve to a directory as folders.
	// Only enable it where traversing outside the starting directory is
	// acceptable — a symlink can point anywhere on the filesystem, so callers
	// that confine paths lexically (ResolveInLibrary) must leave it off.
	IncludeSymlinks bool
}

// ListFolders returns the immediate subdirectories of absPath, sorted by name.
// Non-directory entries are filtered out, subject to opts. The Path field is
// relative to the directory passed in and uses '/' so it round-trips cleanly
// through the HTTP API.
func ListFolders(absPath string, opts ListFoldersOptions) ([]Folder, error) {
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}
	out := make([]Folder, 0, len(entries))
	for _, e := range entries {
		if !opts.IncludeHidden && isHidden(e.Name()) {
			continue
		}
		full := filepath.Join(absPath, e.Name())
		// ReadDir reports the link itself, not its target, so a symlinked
		// directory has IsDir() == false and needs an explicit stat.
		symlink := e.Type()&os.ModeSymlink != 0
		switch {
		case e.IsDir():
		case symlink && opts.IncludeSymlinks && resolvesToDir(full):
		default:
			continue
		}
		out = append(out, Folder{
			Name:          e.Name(),
			Path:          e.Name(),
			HasSubfolders: hasSubdirs(full, opts),
			IsSymlink:     symlink,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// hasSubdirs reports whether dir contains at least one subdirectory that
// ListFolders would return for the same options — otherwise the picker would
// offer an expand arrow that opens an empty node.
// Errors are swallowed and reported as "no" to keep the UX simple.
func hasSubdirs(dir string, opts ListFoldersOptions) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !opts.IncludeHidden && isHidden(e.Name()) {
			continue
		}
		if e.IsDir() {
			return true
		}
		if e.Type()&os.ModeSymlink != 0 && opts.IncludeSymlinks &&
			resolvesToDir(filepath.Join(dir, e.Name())) {
			return true
		}
	}
	return false
}

// resolvesToDir follows a symlink and reports whether it lands on a directory.
// A broken or cyclic link fails the stat and is reported as "not a directory",
// which keeps it out of the listing.
func resolvesToDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isHidden(name string) bool {
	return strings.HasPrefix(name, ".")
}
