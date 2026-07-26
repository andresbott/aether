// internal/scanner/cover_test.go
package scanner_test

import (
	"testing"

	"github.com/andresbott/aether/internal/scanner"
)

func TestIsCoverFile(t *testing.T) {
	tests := []struct {
		name  string
		isCov bool
	}{
		{"cover.jpg", true},
		{"Cover.JPG", true},
		{"folder.jpg", true},
		{"front.png", true},
		{"album.png", true},
		{"albumart.jpg", true},
		{"Adele-19 [Front].jpg", true},
		{"BSO_Bar_Coyote--Frontal.jpg", true},
		{"my-cover-scan.png", true},
		{"track01.mp3", false},
		{"notes.txt", false},
		{"cover.txt", false},
		{"back.jpg", false},
		{"insert.jpg", false},
		// Non-front art must never be mistaken for the front cover, even
		// though the filename contains a cover/front token.
		{"Back Cover.jpg", false},
		{"back-cover.jpg", false},
		{"backcover.jpg", false},
		{"cover-back.jpg", false},
		{"cover_back.png", false},
		{"Adele-19 [Back].jpg", false},
		{"inside cover.jpg", false},
		{"inlay-cover.jpg", false},
		{"disc cover.jpg", false},
		{"booklet-cover.jpg", false},
		{"CD1 cover-disc.jpg", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanner.IsCoverFile(tt.name)
			if got != tt.isCov {
				t.Errorf("IsCoverFile(%q) = %v, want %v", tt.name, got, tt.isCov)
			}
		})
	}
}

func TestBestCover(t *testing.T) {
	files := []string{"back.jpg", "album.png", "cover.jpg", "front.png"}
	best := scanner.BestCover(files)
	if best != "cover.jpg" {
		t.Errorf("expected cover.jpg, got %s", best)
	}
}

func TestBestCoverPrefersExactOverSubstring(t *testing.T) {
	files := []string{"Adele-19 [Front].jpg", "cover.jpg"}
	best := scanner.BestCover(files)
	if best != "cover.jpg" {
		t.Errorf("expected cover.jpg, got %s", best)
	}
}

func TestBestCoverSubstringMatch(t *testing.T) {
	files := []string{"back.jpg", "Adele-19 [Front].jpg"}
	best := scanner.BestCover(files)
	if best != "Adele-19 [Front].jpg" {
		t.Errorf("expected Adele-19 [Front].jpg, got %s", best)
	}
}

// A folder holding both faces of the sleeve must serve the front one. Plain
// alphabetical order puts "Back Cover.jpg" first, so this only works if the
// back name is rejected outright.
func TestBestCoverRejectsBackCoverNames(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  string
	}{
		{"back and front", []string{"Back Cover.jpg", "Front Cover.jpg"}, "Front Cover.jpg"},
		{"backcover and frontcover", []string{"backcover.jpg", "frontcover.jpg"}, "frontcover.jpg"},
		{"back only", []string{"Back Cover.jpg"}, ""},
		{"back and generic cover", []string{"back-cover.jpg", "cover.jpg"}, "cover.jpg"},
		{"non-front art only", []string{"Back Cover.jpg", "booklet-cover.jpg", "disc cover.jpg"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scanner.BestCover(tt.files); got != tt.want {
				t.Errorf("BestCover(%v) = %q, want %q", tt.files, got, tt.want)
			}
		})
	}
}

func TestBestCoverEmpty(t *testing.T) {
	best := scanner.BestCover(nil)
	if best != "" {
		t.Errorf("expected empty, got %s", best)
	}
}
