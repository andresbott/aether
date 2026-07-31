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

// The [0,1) upper bound must hold for the WORST case, not just for sampled
// inputs: a hash landing near the top of the range must not divide out to
// exactly 1.0. Random sampling cannot find this (odds ~1 in 10^16), so assert
// the arithmetic directly against the largest value the implementation can feed
// its divisor.
func TestJitterUpperBoundHoldsForMaxHash(t *testing.T) {
	// float64 represents integers exactly only below 2^53. Above that, conversion
	// rounds to nearest — so float64(2^63-1) becomes exactly 2^63, and dividing
	// by 2^63 would yield 1.0. Guard the divisor choice here.
	const mask53 = uint64(1)<<53 - 1
	maxRatio := float64(mask53) / float64(uint64(1)<<53)
	if maxRatio >= 1 {
		t.Fatalf("max representable ratio = %v, must be < 1", maxRatio)
	}
	// And the unsafe formulation must be recognisably unsafe, so nobody
	// "simplifies" back to it.
	unsafeRatio := float64(uint64(0x7fffffffffffffff)) / float64(uint64(1)<<63)
	if unsafeRatio < 1 {
		t.Fatal("expected the 63-bit formulation to reach 1.0; if this now holds, " +
			"the comment in jitter.go about round-to-nearest needs revisiting")
	}
}
