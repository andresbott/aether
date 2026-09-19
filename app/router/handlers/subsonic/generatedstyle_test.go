package subsonic

import (
	"slices"
	"testing"
)

// TestDefaultCoverStyleIsDeterministicAndKnown pins that the auto-fill style is
// picked deterministically by seed over the full built-in style set — there is
// no configurable selection to confine it.
func TestDefaultCoverStyleIsDeterministicAndKnown(t *testing.T) {
	known := coverStyleNames(coverGen)
	for _, seed := range []string{"a", "b", "c", "zzz", "The Beatles|Abbey Road"} {
		got := defaultCoverStyle(seed)
		if !slices.Contains(known, got) {
			t.Fatalf("seed %q picked %q, not a known style %v", seed, got, known)
		}
		if again := defaultCoverStyle(seed); again != got {
			t.Fatalf("seed %q not deterministic: %q then %q", seed, got, again)
		}
	}
}

// TestStyleAvailableAcceptsEveryStyle pins that every built-in style is offered:
// there is no admin-configurable "available" subset.
func TestStyleAvailableAcceptsEveryStyle(t *testing.T) {
	for _, name := range availableStyles() {
		if !styleAvailable(name) {
			t.Fatalf("style %q reported unavailable", name)
		}
	}
	if styleAvailable("nonexistent-style") {
		t.Fatal("unknown style must not be available")
	}
}
