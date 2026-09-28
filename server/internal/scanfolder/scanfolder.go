// Package scanfolder holds the directories Aether scans for music. They are
// declared in the config file and nowhere else: this package turns that list
// into a validated, immutable Set that the scanner, the media path guard and
// the admin API read, so none of them needs a database row to know where music
// lives.
package scanfolder

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

// maxNameLength bounds a scan folder's name.
const maxNameLength = 200

// Folder is one directory tree to scan.
type Folder struct {
	// Name identifies the folder: every track is stamped with the name of the
	// folder it was indexed under (tracks.scan_folder), and library filters
	// reference it. Unique within a Set.
	Name string
	// Path is the root directory; absolute and cleaned once the folder has been
	// through NewSet.
	Path string
	// ExcludePatterns are Go regular expressions: an entry whose path relative
	// to Path, or whose bare name, matches one is skipped.
	ExcludePatterns []string
	// FollowSymlinks makes the scan descend into symlinked directories.
	FollowSymlinks bool
}

// Excludes compiles ExcludePatterns. NewSet has already proven they compile, so
// a folder taken from a Set never yields the error.
func (f Folder) Excludes() ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, 0, len(f.ExcludePatterns))
	for _, p := range f.ExcludePatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("scan folder %q: invalid exclude pattern %q: %w", f.Name, p, err)
		}
		out = append(out, re)
	}
	return out, nil
}

// Available reports whether the root can be scanned right now: it must not be
// a symlink itself, and it must stat as a directory. This is runtime state,
// never a configuration error — a share that is not mounted yet must not keep
// the server from starting — so NewSet does not check it; the scan preflight
// and the startup warning do.
//
// A root that is itself a symlink is refused rather than silently indexing
// nothing: filepath.WalkDir does not descend a symlink root, and the
// follow-symlinks walk marks the resolved root seen and then skips it when it
// meets the root symlink — so either way the scan would "succeed" with zero
// files. Symlinks INSIDE a root are unaffected; they are followed when
// FollowSymlinks is true.
func (f Folder) Available() error {
	if info, err := os.Lstat(f.Path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		target, _ := os.Readlink(f.Path)
		return fmt.Errorf("root %q is a symbolic link (to %q): a symlinked root is not walked — set Path to the real directory; "+
			"symlinks INSIDE a root are followed when FollowSymlinks is true", f.Path, target)
	}
	info, err := os.Stat(f.Path)
	if err != nil {
		return fmt.Errorf("root %q is unavailable: %w", f.Path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("root %q is not a directory", f.Path)
	}
	return nil
}

// probes shares one in-flight availability probe per root (see bounded).
var probes singleflight.Group

// AvailableWithin is Available with a deadline. A stat on a dead network mount
// can block for minutes; anything that probes a root on a request path or at
// startup must use this, so one hung share cannot hang the server. The scan
// itself keeps using Available: it runs in a background task, and a scan that
// cannot read a root has nothing better to do than wait for the answer.
func (f Folder) AvailableWithin(d time.Duration) error {
	return bounded(d, f.Path, fmt.Sprintf("root %q", f.Path), f.Available)
}

// bounded runs probe aside and stops waiting after d. A probe blocked on a dead
// mount cannot be cancelled, and it pins an OS thread for as long as it blocks,
// so probes are shared per key: while one is in flight, later callers wait on IT
// instead of starting another. A hung root therefore costs one thread however
// often it is asked about. Nothing is cached past completion: the first call
// after a probe returns starts a fresh one.
func bounded(d time.Duration, key, what string, probe func() error) error {
	done := probes.DoChan(key, func() (any, error) { return nil, probe() })
	select {
	case res := <-done:
		return res.Err
	case <-time.After(d):
		return fmt.Errorf("%s did not answer within %s (a hung mount?)", what, d)
	}
}

// Set is the immutable collection of configured scan folders, in name order. A
// nil *Set is a valid empty set.
type Set struct {
	folders []Folder
}

// NewSet validates folders and returns them as a Set. It normalizes each entry
// (trimmed name, absolute cleaned path) and rejects what would make scanning
// ambiguous or fail late: a missing name or path, an over-long name, an exclude
// pattern that does not compile, a repeated name, and roots that are equal or
// nested. Nested roots are rejected because a file under both would be walked
// twice; roots that merely reach one directory through symlinks cannot be seen
// lexically and are settled by the scan's ownership rule instead (the last
// folder to walk a file, in name order, owns it).
//
// Whether a root currently exists is deliberately NOT checked (see
// Folder.Available).
func NewSet(folders []Folder) (*Set, error) {
	out := make([]Folder, 0, len(folders))
	for i, f := range folders {
		f.Name = strings.TrimSpace(f.Name)
		f.Path = strings.TrimSpace(f.Path)
		f.ExcludePatterns = slices.Clone(f.ExcludePatterns)
		if f.Name == "" {
			return nil, fmt.Errorf("scan folder #%d: name is required", i+1)
		}
		if len(f.Name) > maxNameLength {
			return nil, fmt.Errorf("scan folder %q: name too long (max %d chars)", f.Name, maxNameLength)
		}
		if f.Path == "" {
			return nil, fmt.Errorf("scan folder %q: path is required", f.Name)
		}
		abs, err := filepath.Abs(f.Path)
		if err != nil {
			return nil, fmt.Errorf("scan folder %q: invalid path %q: %w", f.Name, f.Path, err)
		}
		f.Path = abs
		if _, err := f.Excludes(); err != nil {
			return nil, err
		}
		for _, prev := range out {
			if prev.Name == f.Name {
				return nil, fmt.Errorf("scan folder %q is declared twice", f.Name)
			}
			if prev.Path == f.Path || within(prev.Path, f.Path) || within(f.Path, prev.Path) {
				return nil, fmt.Errorf("scan folders %q (%s) and %q (%s) overlap: a root must not equal or contain another",
					prev.Name, prev.Path, f.Name, f.Path)
			}
		}
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return &Set{folders: out}, nil
}

// within reports whether p lies strictly inside root, lexically.
func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// detached returns f with its own ExcludePatterns backing array, so a caller
// that edits the folder it was handed cannot reach the set's copy.
func detached(f Folder) Folder {
	f.ExcludePatterns = slices.Clone(f.ExcludePatterns)
	return f
}

// Len is the number of folders.
func (s *Set) Len() int {
	if s == nil {
		return 0
	}
	return len(s.folders)
}

// All returns a copy of the folders in name order — the order they are scanned
// in.
func (s *Set) All() []Folder {
	if s == nil {
		return nil
	}
	out := make([]Folder, 0, len(s.folders))
	for _, f := range s.folders {
		out = append(out, detached(f))
	}
	return out
}

// ByName returns the folder with exactly this name.
func (s *Set) ByName(name string) (Folder, bool) {
	if s == nil {
		return Folder{}, false
	}
	for _, f := range s.folders {
		if f.Name == name {
			return detached(f), true
		}
	}
	return Folder{}, false
}

// Containing returns the folder whose root is absPath or lexically contains it.
// Roots never nest, so at most one matches. Symlinks are not resolved: this
// answers "which folder was this path spelled under", not "where does it live".
func (s *Set) Containing(absPath string) (Folder, bool) {
	if s == nil {
		return Folder{}, false
	}
	clean := filepath.Clean(absPath)
	for _, f := range s.folders {
		if clean == f.Path || within(f.Path, clean) {
			return detached(f), true
		}
	}
	return Folder{}, false
}

// Roots returns every folder's root path, in name order.
func (s *Set) Roots() []string {
	if s == nil {
		return nil
	}
	roots := make([]string, 0, len(s.folders))
	for _, f := range s.folders {
		roots = append(roots, f.Path)
	}
	return roots
}
