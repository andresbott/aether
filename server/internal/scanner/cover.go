// internal/scanner/cover.go
package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

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

// coverDisqualifyTokens name art that is explicitly NOT the front cover. A
// filename carrying one of these is rejected even when it also contains a
// cover/front token ("Back Cover.jpg", "booklet-cover.jpg", "cover-disc.png"),
// so the back scan of a sleeve never gets served as the album cover.
var coverDisqualifyTokens = []string{
	"back", "rear", "inside", "inlay", "inner", "insert", "booklet", "leaflet",
	"disc", "disk", "cd1", "cd2", "media", "label", "obi", "spine", "tray", "sleeve",
	"artist", "band", "logo",
}

func IsCoverFile(filename string) bool {
	return coverRank(filename) >= 0
}

// IsUsableCoverPath reports whether path still works as an album's front cover:
// the file must exist and its name must qualify as front art. Used to re-check
// a cover path already on record instead of trusting it forever.
func IsUsableCoverPath(path string) bool {
	if path == "" || !IsCoverFile(filepath.Base(path)) {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
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
// if the filename is not recognised as a front cover. Names describing other
// artwork are rejected outright; exact matches of canonical names rank highest;
// substring matches rank below all exact matches.
func coverRank(filename string) int {
	ext := strings.ToLower(filepath.Ext(filename))
	if !coverExts[ext] {
		return -1
	}
	base := strings.ToLower(strings.TrimSuffix(filename, filepath.Ext(filename)))
	if pri, ok := coverExactPriority[base]; ok {
		return pri
	}
	if namesOtherArtwork(base) {
		return -1
	}
	exactMax := len(coverExactPriority)
	for i, tok := range coverSubstringTokens {
		if strings.Contains(base, tok) {
			return exactMax + i
		}
	}
	return -1
}

// namesOtherArtwork reports whether base names artwork other than the front
// cover. Tokens match on word boundaries — anything that is not a letter or a
// digit separates words — so "back-cover" and "Adele-19 [Back]" are rejected
// while "Backstreet Boys - cover" is not. Separatorless compounds of a
// disqualifying token and a cover token ("backcover", "coverback") are rejected
// too, but only when they account for the whole word.
func namesOtherArtwork(base string) bool {
	words := strings.FieldsFunc(base, func(r rune) bool {
		isLetter := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		return !isLetter && !isDigit
	})
	for _, w := range words {
		for _, tok := range coverDisqualifyTokens {
			if w == tok || isTokenCompound(w, tok) {
				return true
			}
		}
	}
	return false
}

// isTokenCompound reports whether word is exactly the disqualifying token glued
// to a cover token, in either order.
func isTokenCompound(word, disqualify string) bool {
	for _, tok := range coverSubstringTokens {
		if word == disqualify+tok || word == tok+disqualify {
			return true
		}
	}
	return false
}
