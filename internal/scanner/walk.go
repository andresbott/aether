// internal/scanner/walk.go
package scanner

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/internal/tags"
)

type WalkResult struct {
	FilePath string
	// ScanFolder is the name of the scan folder this file was walked under; reconcile
	// stamps it on the track (model.Track.ScanFolder).
	ScanFolder string
	FileSize   int64
	ModTime    time.Time
	Dir        string
}

// IsAudioFile reports whether name is a file Aether indexes. It delegates to
// tags.Supported so the scanner holds no extension list of its own: what gets
// walked into the library and what the metadata editor offers to edit are the
// same set by construction.
func IsAudioFile(name string) bool {
	return tags.Supported(name)
}

// Walk collects the audio files under one scan folder.
func Walk(folder scanfolder.Folder, excludes []*regexp.Regexp) ([]WalkResult, error) {
	var results []WalkResult
	walkFn := makeWalkFn(folder, excludes, &results)
	var err error
	if folder.FollowSymlinks {
		err = symWalk(folder.Path, walkFn)
	} else {
		err = filepath.WalkDir(folder.Path, walkFn)
	}
	if err != nil {
		return nil, err
	}
	return results, nil
}

// WalkWouldEmit reports whether a full Walk of folder would index abs and, if
// so, the WalkResult it would produce. It is the single admission predicate the
// editor's targeted rescan shares with the crawling Walk, so a rescan can never
// insert a track the next scan would immediately delete. Whether abs is even
// reachable — and the canonical path the walk would record it under — depends on
// folder.FollowSymlinks.
func WalkWouldEmit(folder scanfolder.Folder, excludes []*regexp.Regexp, abs string) (WalkResult, bool) {
	root := filepath.Clean(folder.Path)
	followSymlinks := folder.FollowSymlinks
	clean := filepath.Clean(abs)

	// The spelled path must live inside the scan folder: the crawl only reaches a
	// file by descending from the root, so a path lexically outside it is one no
	// walk could visit. (A followed symlink may point the resolved file outside
	// the root; that is judged below, on the resolved path.)
	relOrig, err := filepath.Rel(root, clean)
	if err != nil || relOrig == ".." || strings.HasPrefix(relOrig, ".."+string(filepath.Separator)) {
		return WalkResult{}, false
	}
	// Audio-ness keys off the entry name the walk sees — the spelled leaf, not a
	// symlink target — so IsAudioFile reads abs, not the resolved path.
	if !IsAudioFile(abs) {
		return WalkResult{}, false
	}

	// Resolve to the path the walk actually visits, records and tests against. A
	// no-follow walk (filepath.WalkDir) never descends a symlinked directory, so
	// a file behind one is unreachable and refused; the leaf itself may be a
	// symlink, which the walk emits under its link spelling. With follow, the
	// walk descends into the target and judges the resolved path, so admission
	// resolves and judges that same spelling.
	path := clean
	segments := strings.Split(relOrig, string(filepath.Separator))
	if followSymlinks {
		if anySegmentIsSymlink(root, segments) {
			resolved, rerr := filepath.EvalSymlinks(abs)
			if rerr != nil {
				return WalkResult{}, false // broken or unreadable link: the walk skips it
			}
			path = resolved
		}
	} else if anySegmentIsSymlink(root, segments[:len(segments)-1]) {
		return WalkResult{}, false
	}

	// Excludes are tested against the path the walk visits — the resolved one
	// under follow — so a pattern keys off the same spelling either code path
	// would see (e.g. a symlink into a directory excluded by its real name).
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return WalkResult{}, false
	}
	if excludedByAnySegment(rel, excludes) {
		return WalkResult{}, false
	}

	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return WalkResult{}, false
	}
	// No separate tagReader.CanRead gate: IsAudioFile is tags.Supported, and
	// every supported format is readable by some reader (enforced by
	// tags.TestSupportedIsReadable), so admission asks one question, not two.
	return WalkResult{
		FilePath:   path,
		ScanFolder: folder.Name,
		FileSize:   info.Size(),
		ModTime:    info.ModTime(),
		Dir:        filepath.Dir(path),
	}, true
}

// anySegmentIsSymlink reports whether any of the given path segments, joined
// onto libRoot in order, is a symlink. It underpins two rules: a no-follow walk
// skips a file whose ancestor directory is a symlink, and a follow walk records
// a symlinked file by its resolved path.
func anySegmentIsSymlink(libRoot string, segments []string) bool {
	prefix := filepath.Clean(libRoot)
	for _, seg := range segments {
		prefix = filepath.Join(prefix, seg)
		fi, err := os.Lstat(prefix)
		if err != nil {
			// Unreadable segment: the file won't stat either, so WalkWouldEmit's
			// own stat rejects it. Don't mask that decision here.
			return false
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return true
		}
	}
	return false
}

// makeWalkFn builds the WalkDirFunc for one scan folder: it applies excludes,
// optionally follows symlinks, and appends audio files to results.
func makeWalkFn(folder scanfolder.Folder, excludes []*regexp.Regexp, results *[]WalkResult) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if skip, skipDir := matchExcludes(folder.Path, path, d, excludes); skip {
			if skipDir {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if folder.FollowSymlinks && d.Type()&fs.ModeSymlink != 0 {
			return walkSymlinkEntry(path, d, folder.Name, results)
		}
		appendAudio(path, d, folder.Name, results)
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
// excluded when a pattern matches either its path relative to the scan folder's
// root or its bare name. Shared with the rescan's admission check so the two
// cannot drift apart.
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
func walkSymlinkEntry(path string, d fs.DirEntry, scanFolder string, results *[]WalkResult) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil
	}
	if info.IsDir() {
		return symWalk(resolved, collectAudioFn(scanFolder, results))
	}
	appendAudio(path, d, scanFolder, results)
	return nil
}

// collectAudioFn returns a WalkDirFunc that appends every audio file it visits.
func collectAudioFn(scanFolder string, results *[]WalkResult) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		appendAudio(path, d, scanFolder, results)
		return nil
	}
}

// appendAudio appends path to results when it is an audio file whose info can
// be read.
func appendAudio(path string, d fs.DirEntry, scanFolder string, results *[]WalkResult) {
	if !IsAudioFile(d.Name()) {
		return
	}
	info, err := audioFileInfo(path, d)
	if err != nil {
		return
	}
	*results = append(*results, WalkResult{
		FilePath:   path,
		ScanFolder: scanFolder,
		FileSize:   info.Size(),
		ModTime:    info.ModTime(),
		Dir:        filepath.Dir(path),
	})
}

// audioFileInfo describes the audio file rather than the directory entry that
// led to it: for a symlink, d.Info() is an lstat of the link, whose size is the
// length of its target path. planTrackContinuity matches moved files on
// file_size, so a link's own size would be a false fingerprint.
func audioFileInfo(path string, d fs.DirEntry) (os.FileInfo, error) {
	if d.Type()&fs.ModeSymlink != 0 {
		return os.Stat(path)
	}
	return d.Info()
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
