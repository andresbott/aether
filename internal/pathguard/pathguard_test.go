package pathguard_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/pathguard"
)

func TestWithinAcceptsPathsInsideRoot(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{
		filepath.Join(root, "a.mp3"),
		filepath.Join(root, "artist", "album", "01.flac"),
		root, // the root itself is inside itself
	} {
		if !pathguard.Within(root, p) {
			t.Errorf("Within(%q, %q) = false, want true", root, p)
		}
	}
}

func TestWithinRejectsPathsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{
		"/etc/passwd",
		filepath.Join(root, "..", "escaped.mp3"),
		filepath.Join(root, "sub", "..", "..", "escaped.mp3"),
	} {
		if pathguard.Within(root, p) {
			t.Errorf("Within(%q, %q) = true, want false", root, p)
		}
	}
}

// A sibling directory whose name starts with the root's name is not inside it:
// comparing raw string prefixes would wrongly accept /music-private for /music.
func TestWithinRejectsSiblingWithRootAsNamePrefix(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "music")
	sibling := filepath.Join(base, "music-private", "secret.mp3")
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	if pathguard.Within(root, sibling) {
		t.Errorf("Within(%q, %q) = true; a name prefix is not a path boundary", root, sibling)
	}
}

// Relative paths cannot be reasoned about against an absolute root, so they are
// refused rather than resolved against the process's working directory.
func TestWithinRejectsRelativePath(t *testing.T) {
	root := t.TempDir()
	if pathguard.Within(root, "a/b.mp3") {
		t.Error("expected a relative path to be refused")
	}
}

// A symlink pointing out of the root must not smuggle a file past the guard:
// containment is about where the bytes actually live.
func TestWithinRejectsSymlinkEscapingRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "lib")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o750); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(outside, "secret.mp3")
	if err := os.WriteFile(secret, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.mp3")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if pathguard.Within(root, link) {
		t.Errorf("Within(%q, %q) = true; a symlink out of the root escapes it", root, link)
	}
}

// A symlink that stays inside the root is legitimate — some libraries are built
// out of them — and must keep working.
func TestWithinAcceptsSymlinkInsideRoot(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real.mp3")
	if err := os.WriteFile(real, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.mp3")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if !pathguard.Within(root, link) {
		t.Errorf("Within(%q, %q) = false; an in-root symlink is allowed", root, link)
	}
}

func TestGuardAllowsAnyConfiguredRoot(t *testing.T) {
	r1 := t.TempDir()
	r2 := t.TempDir()
	g := pathguard.New(r1, r2)

	if !g.Allows(filepath.Join(r1, "a.mp3")) {
		t.Error("expected a path in the first root to be allowed")
	}
	if !g.Allows(filepath.Join(r2, "deep", "b.mp3")) {
		t.Error("expected a path in the second root to be allowed")
	}
	if g.Allows("/etc/passwd") {
		t.Error("expected a path outside every root to be refused")
	}
}

// A Guard with no roots is the "no libraries configured" state. It must deny,
// not allow: failing open would make the guard useless exactly when the config
// is broken.
func TestGuardWithNoRootsDeniesEverything(t *testing.T) {
	g := pathguard.New()
	if g.Allows("/etc/passwd") {
		t.Error("a guard with no roots must deny")
	}
	if g.Allows(filepath.Join(t.TempDir(), "a.mp3")) {
		t.Error("a guard with no roots must deny")
	}
}

func TestGuardIgnoresEmptyRoots(t *testing.T) {
	root := t.TempDir()
	g := pathguard.New("", root, "")
	if !g.Allows(filepath.Join(root, "a.mp3")) {
		t.Error("expected the usable root to still allow its files")
	}
	// An empty root must not degenerate into "allow everything".
	if g.Allows("/etc/passwd") {
		t.Error("an empty root must not widen the guard")
	}
}
