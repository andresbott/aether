// internal/scanner/walk.go
package scanner

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/andresbott/aether/internal/model"
)

var audioExtensions = map[string]bool{
	".mp3": true, ".flac": true, ".ogg": true, ".opus": true,
	".m4a": true, ".wma": true, ".wav": true, ".aiff": true,
	".ape": true, ".wv": true, ".aac": true, ".m4b": true,
	".mka": true, ".tta": true, ".dsf": true, ".webm": true,
}

type WalkResult struct {
	FilePath  string
	LibraryID uint
	ModTime   time.Time
	Dir       string
}

func IsAudioFile(name string) bool {
	return audioExtensions[strings.ToLower(filepath.Ext(name))]
}

func Walk(libs []model.Library, excludes []*regexp.Regexp, followSymlinks bool) ([]WalkResult, error) {
	var results []WalkResult
	for _, lib := range libs {
		walkFn := makeWalkFn(lib, excludes, followSymlinks, &results)
		var err error
		if followSymlinks {
			err = symWalk(lib.Path, walkFn)
		} else {
			err = filepath.WalkDir(lib.Path, walkFn)
		}
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

// makeWalkFn builds the WalkDirFunc for one library: it applies excludes,
// optionally follows symlinks, and appends audio files to results.
func makeWalkFn(lib model.Library, excludes []*regexp.Regexp, followSymlinks bool, results *[]WalkResult) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if skip, skipDir := matchExcludes(lib.Path, path, d, excludes); skip {
			if skipDir {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if followSymlinks && d.Type()&fs.ModeSymlink != 0 {
			return walkSymlinkEntry(path, d, lib.ID, results)
		}
		appendAudio(path, d, lib.ID, results)
		return nil
	}
}

// matchExcludes reports whether path (relative to root) or its base name
// matches any exclude pattern, and whether a matching directory should be
// skipped wholesale.
func matchExcludes(root, path string, d fs.DirEntry, excludes []*regexp.Regexp) (skip, skipDir bool) {
	relPath, _ := filepath.Rel(root, path)
	if matchesExclude(excludes, relPath, d.Name()) {
		return true, d.IsDir()
	}
	return false, false
}

// matchesExclude is the per-entry exclude test the walk applies: an entry is
// excluded when a pattern matches either its path relative to the library root
// or its bare name. Shared with the rescan's admission check so the two cannot
// drift apart.
func matchesExclude(excludes []*regexp.Regexp, relPath, name string) bool {
	for _, ex := range excludes {
		if ex.MatchString(relPath) || ex.MatchString(name) {
			return true
		}
	}
	return false
}

// walkSymlinkEntry handles a symlink encountered during a top-level walk:
// target directories are recursed into, regular-file targets are appended as
// the symlink path itself, and broken or unreadable links are skipped.
func walkSymlinkEntry(path string, d fs.DirEntry, libID uint, results *[]WalkResult) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil
	}
	if info.IsDir() {
		return symWalk(resolved, collectAudioFn(libID, results))
	}
	appendAudio(path, d, libID, results)
	return nil
}

// collectAudioFn returns a WalkDirFunc that appends every audio file it visits.
func collectAudioFn(libID uint, results *[]WalkResult) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		appendAudio(path, d, libID, results)
		return nil
	}
}

// appendAudio appends path to results when it is an audio file whose info can
// be read.
func appendAudio(path string, d fs.DirEntry, libID uint, results *[]WalkResult) {
	if !IsAudioFile(d.Name()) {
		return
	}
	info, err := d.Info()
	if err != nil {
		return
	}
	*results = append(*results, WalkResult{
		FilePath:  path,
		LibraryID: libID,
		ModTime:   info.ModTime(),
		Dir:       filepath.Dir(path),
	})
}

func symWalk(root string, fn fs.WalkDirFunc) error {
	seen := make(map[string]bool)
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolved = root
	}
	seen[resolved] = true

	return symWalkInner(root, fn, seen)
}

func symWalkInner(root string, fn fs.WalkDirFunc, seen map[string]bool) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.Type()&fs.ModeSymlink == 0 {
			return fn(path, d, err)
		}
		return followSymlinkEntry(path, d, fn, seen)
	})
}

// followSymlinkEntry resolves a symlink encountered during a recursive
// symlink-following walk: unseen target directories are descended into (the
// seen set guards against cycles), file targets are reported to fn, and broken
// links are skipped.
func followSymlinkEntry(path string, d fs.DirEntry, fn fs.WalkDirFunc, seen map[string]bool) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil
	}
	if !info.IsDir() {
		return fn(resolved, d, nil)
	}
	if seen[resolved] {
		return nil
	}
	seen[resolved] = true
	return symWalkInner(resolved, fn, seen)
}
