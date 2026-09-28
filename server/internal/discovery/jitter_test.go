package discovery_test

import (
	"testing"

	"github.com/andresbott/aether/internal/discovery"
)

func TestJitterIsInUnitInterval(t *testing.T) {
	for id := uint(1); id <= 100; id++ {
		got := discovery.Jitter(42, "al", id)
		if got < 0 || got >= 1 {
			t.Fatalf("Jitter(42,\"al\",%d) = %v, want [0,1)", id, got)
		}
	}
}

func TestJitterIsDeterministic(t *testing.T) {
	a := discovery.Jitter(42, "al", 7)
	b := discovery.Jitter(42, "al", 7)
	if a != b {
		t.Fatalf("same inputs gave %v then %v", a, b)
	}
}

func TestJitterVariesBySeed(t *testing.T) {
	if discovery.Jitter(1, "al", 7) == discovery.Jitter(2, "al", 7) {
		t.Fatal("different seeds gave the same jitter")
	}
}

// An album and a playlist can share a numeric id; the kind prefix must keep
// them from drawing the same jitter.
func TestJitterVariesByKind(t *testing.T) {
	if discovery.Jitter(42, "al", 7) == discovery.Jitter(42, "pl", 7) {
		t.Fatal("album and playlist with the same id drew the same jitter")
	}
}

func TestJitterVariesByID(t *testing.T) {
	if discovery.Jitter(42, "al", 7) == discovery.Jitter(42, "al", 8) {
		t.Fatal("different ids gave the same jitter")
	}
}
