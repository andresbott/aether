package discovery

import (
	"math"
	"testing"
)

// hashRatio's upper bound must hold for the WORST case, not just for sampled
// hashes: the inputs that break it are ~1 in 10^16, so no amount of calling
// Jitter would find them. Feed the arithmetic its extreme inputs directly.
func TestHashRatioStaysBelowOne(t *testing.T) {
	cases := []struct {
		name string
		sum  uint64
	}{
		{"zero", 0},
		{"all bits set", math.MaxUint64},
		{"max 53-bit value", uint64(1)<<53 - 1},
		{"max 63-bit value", uint64(1)<<63 - 1},
		{"one below 2^53", uint64(1) << 53},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hashRatio(tc.sum)
			if got < 0 || got >= 1 {
				t.Fatalf("hashRatio(%d) = %v, want [0,1)", tc.sum, got)
			}
		})
	}
}

// The 63-bit formulation this code deliberately avoids really does reach 1.0.
// If this ever stops holding, the rationale in hashRatio's comment is stale.
func TestUnsafeSixtyThreeBitFormulationReachesOne(t *testing.T) {
	unsafe := float64(uint64(1)<<63-1) / float64(uint64(1)<<63)
	if unsafe < 1 {
		t.Fatal("float64(2^63-1)/2^63 no longer reaches 1.0; hashRatio's comment " +
			"about round-to-nearest needs revisiting")
	}
}
