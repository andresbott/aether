// internal/scanner/cover.go
package scanner

import (
	"path/filepath"
	"strings"
)

var coverNames = []string{"cover", "folder", "front", "album", "albumart", "albumartsmall", "thumb"}
var coverExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true, ".webp": true}

// Priorities: exact-match tokens beat substring matches. Lower is better.
var coverExactPriority = map[string]int{
	"cover":         0,
	"front":         1,
	"folder":        2,
	"album":         3,
	"albumart":      4,
	"albumartsmall": 5,
	"thumb":         6,
}

var coverSubstringTokens = []string{"cover", "front", "folder", "album", "albumart"}

func IsCoverFile(filename string) bool {
	return coverRank(filename) >= 0
}

func BestCover(filenames []string) string {
	best := ""
	bestPri := -1
	for _, f := range filenames {
		pri := coverRank(f)
		if pri < 0 {
			continue
		}
		if best == "" || pri < bestPri {
			bestPri = pri
			best = f
		}
	}
	return best
}

// coverRank returns the match priority for filename (lower is better), or -1
// if the filename is not recognised as a cover. Exact matches of canonical
// names rank highest; substring matches rank below all exact matches.
func coverRank(filename string) int {
	ext := strings.ToLower(filepath.Ext(filename))
	if !coverExts[ext] {
		return -1
	}
	base := strings.ToLower(strings.TrimSuffix(filename, filepath.Ext(filename)))
	if pri, ok := coverExactPriority[base]; ok {
		return pri
	}
	exactMax := len(coverExactPriority)
	for i, tok := range coverSubstringTokens {
		if strings.Contains(base, tok) {
			return exactMax + i
		}
	}
	return -1
}
