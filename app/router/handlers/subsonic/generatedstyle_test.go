package subsonic

import "testing"

func TestDefaultGeneratorConfinesToSet(t *testing.T) {
	gen := defaultGenerator([]string{"bauhaus"})
	for _, seed := range []string{"a", "b", "c", "zzz", "The Beatles|Abbey Road"} {
		if got := gen.StyleFor(seed).Name(); got != "bauhaus" {
			t.Fatalf("seed %q picked %q, want bauhaus", seed, got)
		}
	}
}

func TestDefaultGeneratorEmptyFallsBackToFull(t *testing.T) {
	gen := defaultGenerator(nil)
	if gen != coverGen {
		t.Fatal("empty set must fall back to the full generator")
	}
	gen = defaultGenerator([]string{"nonexistent-style"})
	if gen != coverGen {
		t.Fatal("unknown-only set must fall back to the full generator")
	}
}
