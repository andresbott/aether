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
		{"track01.mp3", false},
		{"notes.txt", false},
		{"cover.txt", false},
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

func TestBestCoverEmpty(t *testing.T) {
	best := scanner.BestCover(nil)
	if best != "" {
		t.Errorf("expected empty, got %s", best)
	}
}
