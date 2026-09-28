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

func TestSampleCandidatesFromAvailable(t *testing.T) {
	avail := []string{"bauhaus", "rings"}
	got := sampleCandidates(avail, "seed", 9)
	if len(got) != 9 {
		t.Fatalf("want 9 candidates, got %d", len(got))
	}
	for _, c := range got {
		if c.Style != "bauhaus" && c.Style != "rings" {
			t.Fatalf("candidate style %q not in available set", c.Style)
		}
	}
	if len(sampleCandidates(nil, "seed", 9)) != 0 {
		t.Fatal("empty available set yields no candidates")
	}
}
