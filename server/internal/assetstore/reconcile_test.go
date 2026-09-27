package assetstore

import (
	"testing"
)

func TestReconcileRemovesAutoOnlyOrphan(t *testing.T) {
	s := New(t.TempDir())
	if err := s.PutAuto(KindAlbum, "dead", "jpg", []byte("a")); err != nil {
		t.Fatal(err)
	}
	removed, keptManual, err := s.Reconcile(KindAlbum, map[string]struct{}{})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if removed != 1 || keptManual != 0 {
		t.Fatalf("removed=%d keptManual=%d, want 1/0", removed, keptManual)
	}
	if _, ok := s.Get(KindAlbum, "dead"); ok {
		t.Fatal("auto-only orphan should be removed")
	}
}

// A hand-uploaded cover is not rebuildable, and — keyed by identity — it
// correctly re-attaches if the entity comes back after a DB drop + rescan. So an
// orphaned manual upload is preserved, never swept.
func TestReconcilePreservesManualUploadOrphan(t *testing.T) {
	s := New(t.TempDir())
	if err := s.PutManual(KindAlbum, "dead", "jpg", []byte("m")); err != nil {
		t.Fatal(err)
	}
	removed, keptManual, err := s.Reconcile(KindAlbum, map[string]struct{}{})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if removed != 0 || keptManual != 1 {
		t.Fatalf("removed=%d keptManual=%d, want 0/1", removed, keptManual)
	}
	if _, ok := s.Get(KindAlbum, "dead"); !ok {
		t.Fatal("manual upload of an orphan must be preserved")
	}
}

// A dir holding both an auto and a manual cover counts as manual: it is kept.
func TestReconcileMixedDirWithManualIsPreserved(t *testing.T) {
	s := New(t.TempDir())
	if err := s.PutAuto(KindAlbum, "dead", "jpg", []byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := s.PutManual(KindAlbum, "dead", "png", []byte("m")); err != nil {
		t.Fatal(err)
	}
	removed, keptManual, err := s.Reconcile(KindAlbum, map[string]struct{}{})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if removed != 0 || keptManual != 1 {
		t.Fatalf("removed=%d keptManual=%d, want 0/1", removed, keptManual)
	}
}

func TestReconcileKeepsLiveEntries(t *testing.T) {
	s := New(t.TempDir())
	if err := s.PutAuto(KindAlbum, "live", "jpg", []byte("a")); err != nil {
		t.Fatal(err)
	}
	removed, keptManual, err := s.Reconcile(KindAlbum, map[string]struct{}{"live": {}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if removed != 0 || keptManual != 0 {
		t.Fatalf("removed=%d keptManual=%d, want 0/0", removed, keptManual)
	}
	if _, ok := s.Get(KindAlbum, "live"); !ok {
		t.Fatal("live entry must remain")
	}
}

func TestReconcileMissingKindDirIsNoOp(t *testing.T) {
	s := New(t.TempDir())
	removed, keptManual, err := s.Reconcile(KindAlbum, map[string]struct{}{})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if removed != 0 || keptManual != 0 {
		t.Fatalf("removed=%d keptManual=%d, want 0/0", removed, keptManual)
	}
}
