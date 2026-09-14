package imagecache_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/imagecache"
)

// seedEntry creates a fake derivative directory <root>/<kind>/<key>/ holding one
// file, standing in for cached derivatives without paying an encode.
func seedEntry(t *testing.T, root, kind, key string) {
	t.Helper()
	dir := filepath.Join(root, kind, key)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cover.abc123.200.webp"), []byte("x"), 0o640); err != nil {
		t.Fatalf("write derivative: %v", err)
	}
}

func TestReconcileRemovesOrphansKeepsLive(t *testing.T) {
	root := t.TempDir()
	c := imagecache.New(root)
	for _, k := range []string{"live1", "live2", "dead1", "dead2"} {
		seedEntry(t, root, "album", k)
	}
	live := map[string]struct{}{"live1": {}, "live2": {}}

	removed, err := c.Reconcile("album", live)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}
	for _, k := range []string{"live1", "live2"} {
		if _, err := os.Stat(filepath.Join(root, "album", k)); err != nil {
			t.Errorf("live entry %q should remain: %v", k, err)
		}
	}
	for _, k := range []string{"dead1", "dead2"} {
		if _, err := os.Stat(filepath.Join(root, "album", k)); !os.IsNotExist(err) {
			t.Errorf("dead entry %q should be gone", k)
		}
	}
}

// A different kind's directory must be untouched: the whitelist lives in the
// caller, but Reconcile itself must only ever walk the one kind it was given —
// this is what keeps the metadata editor's "editor" thumbnails safe.
func TestReconcileTouchesOnlyItsKind(t *testing.T) {
	root := t.TempDir()
	c := imagecache.New(root)
	seedEntry(t, root, "album", "dead")
	seedEntry(t, root, "editor", "some-source-file")

	if _, err := c.Reconcile("album", map[string]struct{}{}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "editor", "some-source-file")); err != nil {
		t.Errorf("editor entry must survive an album reconcile: %v", err)
	}
}

func TestReconcileMissingKindDirIsNoOp(t *testing.T) {
	c := imagecache.New(t.TempDir())
	removed, err := c.Reconcile("album", map[string]struct{}{})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
}

func TestReconcileRejectsUnsafeKind(t *testing.T) {
	c := imagecache.New(t.TempDir())
	if _, err := c.Reconcile("../escape", map[string]struct{}{}); err == nil {
		t.Fatal("expected an error for an unsafe kind")
	}
}
