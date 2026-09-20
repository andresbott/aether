package pathguard

import (
	"os"
	"path/filepath"
	"testing"
)

// spyOnRoots records which roots a check resolved.
func spyOnRoots(t *testing.T) *[]string {
	t.Helper()
	var touched []string
	orig := resolveRoot
	resolveRoot = func(root string) string {
		touched = append(touched, root)
		return orig(root)
	}
	t.Cleanup(func() { resolveRoot = orig })
	return &touched
}

// A hung network share must not stall media that lives in another folder: the
// guard probes the root the path is spelled under, and only that one when it
// matches.
func TestAllowsProbesOnlyTheRootThePathIsSpelledUnder(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	file := filepath.Join(b, "Album", "01.mp3")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	g := New(a, b) // a sorts first: the old loop would have probed it
	touched := spyOnRoots(t)

	if !g.Allows(file) {
		t.Fatal("a file under root b must be allowed")
	}
	if len(*touched) != 1 || (*touched)[0] != filepath.Clean(b) {
		t.Fatalf("probed roots = %v, want only %s", *touched, b)
	}
}

// A path spelled under no root is still checked against every root — it may be
// a row recorded under its resolved spelling — and still refused when none
// contains it.
func TestAllowsStillChecksEveryRootForAPathSpelledUnderNone(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	g := New(a, b)
	touched := spyOnRoots(t)

	if g.Allows(filepath.Join(t.TempDir(), "elsewhere.mp3")) {
		t.Fatal("a path outside every root must be refused")
	}
	if len(*touched) != 2 {
		t.Fatalf("probed roots = %v, want both", *touched)
	}
}

// Deny-all is what a server with no scan folder runs with: it must answer
// without touching the filesystem.
func TestAllowsWithNoRootsProbesNothing(t *testing.T) {
	g := New("", "")
	touched := spyOnRoots(t)
	if g.Allows("/anything/at/all.mp3") {
		t.Fatal("a guard with no usable roots must allow nothing")
	}
	if len(*touched) != 0 {
		t.Fatalf("probed roots = %v, want none", *touched)
	}
}
