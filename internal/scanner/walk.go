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
		walkFn := func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			relPath, _ := filepath.Rel(lib.Path, path)
			for _, ex := range excludes {
				if ex.MatchString(relPath) || ex.MatchString(d.Name()) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			if d.IsDir() {
				return nil
			}
			if followSymlinks && d.Type()&fs.ModeSymlink != 0 {
				resolved, err := filepath.EvalSymlinks(path)
				if err != nil {
					return nil
				}
				info, err := os.Stat(resolved)
				if err != nil {
					return nil
				}
				if info.IsDir() {
					return symWalk(resolved, func(innerPath string, innerD fs.DirEntry, innerErr error) error {
						if innerErr != nil || innerD.IsDir() {
							return nil
						}
						if !IsAudioFile(innerD.Name()) {
							return nil
						}
						innerInfo, err := innerD.Info()
						if err != nil {
							return nil
						}
						results = append(results, WalkResult{
							FilePath:  innerPath,
							LibraryID: lib.ID,
							ModTime:   innerInfo.ModTime(),
							Dir:       filepath.Dir(innerPath),
						})
						return nil
					})
				}
			}
			if !IsAudioFile(d.Name()) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			results = append(results, WalkResult{
				FilePath:  path,
				LibraryID: lib.ID,
				ModTime:   info.ModTime(),
				Dir:       filepath.Dir(path),
			})
			return nil
		}

		if followSymlinks {
			if err := symWalk(lib.Path, walkFn); err != nil {
				return nil, err
			}
		} else {
			if err := filepath.WalkDir(lib.Path, walkFn); err != nil {
				return nil, err
			}
		}
	}
	return results, nil
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
		if err != nil {
			return fn(path, d, err)
		}
		if d.Type()&fs.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return nil
			}
			info, err := os.Stat(resolved)
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if seen[resolved] {
					return nil
				}
				seen[resolved] = true
				return symWalkInner(resolved, fn, seen)
			}
			return fn(resolved, d, nil)
		}
		return fn(path, d, err)
	})
}
