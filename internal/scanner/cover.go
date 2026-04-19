// internal/scanner/cover.go
package scanner

import (
	"path/filepath"
	"strings"
)

var coverNames = []string{"cover", "folder", "front", "album", "albumart", "albumartsmall", "thumb"}
var coverExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true, ".webp": true}

var coverPriority = map[string]int{
	"cover":    0,
	"front":    1,
	"folder":   2,
	"album":    3,
	"albumart": 4,
}

func IsCoverFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	if !coverExts[ext] {
		return false
	}
	base := strings.ToLower(strings.TrimSuffix(filename, filepath.Ext(filename)))
	for _, name := range coverNames {
		if base == name {
			return true
		}
	}
	return false
}

func BestCover(filenames []string) string {
	best := ""
	bestPri := len(coverPriority) + 1
	for _, f := range filenames {
		if !IsCoverFile(f) {
			continue
		}
		base := strings.ToLower(strings.TrimSuffix(f, filepath.Ext(f)))
		pri, ok := coverPriority[base]
		if !ok {
			pri = len(coverPriority)
		}
		if pri < bestPri {
			bestPri = pri
			best = f
		}
	}
	return best
}
