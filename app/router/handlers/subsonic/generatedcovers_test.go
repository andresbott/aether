package subsonic

import (
	"fmt"
	"testing"
)

func TestRenderGeneratedCoverDeterministicAndVaries(t *testing.T) {
	a, err := renderGeneratedCover("seed", "Title", "bauhaus", 0, 96)
	if err != nil || len(a) == 0 {
		t.Fatalf("render failed: %v", err)
	}
	a2, _ := renderGeneratedCover("seed", "Title", "bauhaus", 0, 96)
	if fmt.Sprintf("%x", a) != fmt.Sprintf("%x", a2) {
		t.Fatal("same inputs must be byte-identical")
	}
	b, _ := renderGeneratedCover("seed", "Title", "bauhaus", 1, 96)
	if fmt.Sprintf("%x", a) == fmt.Sprintf("%x", b) {
		t.Fatal("different variation must differ")
	}
}
