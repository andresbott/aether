package subsonic

import (
	"bytes"
	"image"
	"testing"
)

// TestGenerateCoverRendersAtRequestedSize pins the grain-consistency fix: the
// generated cover must render at the requested display size (not a fixed master
// the cache downscales), so film grain is applied at the size actually shown.
func TestGenerateCoverRendersAtRequestedSize(t *testing.T) {
	for _, size := range []int{96, 256, 512} {
		data, err := generateCover("shakira|banana ep", "bauhaus", "Banana EP", size)
		if err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("size %d: decode: %v", size, err)
		}
		if cfg.Width != size || cfg.Height != size {
			t.Errorf("size %d: rendered %dx%d, want %dx%d", size, cfg.Width, cfg.Height, size, size)
		}
	}
}
