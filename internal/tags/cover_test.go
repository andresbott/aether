// internal/tags/cover_test.go
package tags_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/tags"
	"go.senan.xyz/taglib"
)

// fixtureCopy copies the shared empty.flac fixture into a temp dir so tests
// can embed pictures into it.
func fixtureCopy(t *testing.T, name string) string {
	t.Helper()
	src := "testdata/empty.flac"
	data, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("no fixture at %s: %v", src, err)
	}
	dst := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatalf("write fixture copy: %v", err)
	}
	return dst
}

// A file whose only attached picture is a back cover has no cover: treating it
// as one makes the scanner flag the track as the album's cover source and the
// back scan ends up served as the album art.
func TestTaglibReaderHasCoverIgnoresNonFrontTypes(t *testing.T) {
	tests := []struct {
		typeID string
		want   bool
	}{
		{"Front Cover", true},
		{"Back Cover", false},
		{"Media", false},
		{"Leaflet Page", false},
		{"Artist", false},
		{"Band", false},
		{"Illustration", false},
		{"Other", false},
	}
	for _, tt := range tests {
		t.Run(tt.typeID, func(t *testing.T) {
			path := fixtureCopy(t, "one.flac")
			if err := taglib.WriteImageOptions(path, []byte{0xFF, 0xD8, 0xFF, 0xD9}, 0, tt.typeID, "", "image/jpeg"); err != nil {
				t.Fatalf("write %s: %v", tt.typeID, err)
			}
			m, err := tags.TaglibReader{}.Read(path)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if m.HasCover != tt.want {
				t.Errorf("HasCover for %s = %v, want %v", tt.typeID, m.HasCover, tt.want)
			}
		})
	}
}

// A front cover is still detected when it is not the first attached picture.
func TestTaglibReaderHasCoverFindsFrontBehindBack(t *testing.T) {
	path := fixtureCopy(t, "both.flac")
	if err := taglib.WriteImageOptions(path, []byte{0xFF, 0xD8, 0xFF, 0xD9}, 0, "Back Cover", "", "image/jpeg"); err != nil {
		t.Fatalf("write back: %v", err)
	}
	if err := taglib.WriteImageOptions(path, []byte{0xFF, 0xD8, 0xFF, 0xD9}, 1, "Front Cover", "", "image/jpeg"); err != nil {
		t.Fatalf("write front: %v", err)
	}
	m, err := tags.TaglibReader{}.Read(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !m.HasCover {
		t.Error("HasCover = false, want true (front cover present at index 1)")
	}
}
