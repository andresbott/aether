package metadataedit_test

import (
	"slices"
	"testing"

	"github.com/andresbott/aether/internal/metadataedit"
)

func TestManagedTagKeys_SortedAndConsistentWithIsManagedTag(t *testing.T) {
	keys := metadataedit.ManagedTagKeys()
	if !slices.IsSorted(keys) {
		t.Fatalf("keys not sorted: %v", keys)
	}
	for _, want := range []string{"TITLE", "RELEASETYPE", "MUSICBRAINZ_ALBUMTYPE"} {
		if !slices.Contains(keys, want) {
			t.Fatalf("%s missing from %v", want, keys)
		}
	}
	for _, k := range keys {
		if !metadataedit.IsManagedTag(k) {
			t.Fatalf("listed key %q is not managed", k)
		}
	}
	// Callers get their own copy.
	keys[0] = "MUTATED"
	if slices.Contains(metadataedit.ManagedTagKeys(), "MUTATED") {
		t.Fatal("ManagedTagKeys returned shared storage")
	}
}
