package assetstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPutAutoAndGet(t *testing.T) {
	s := New(t.TempDir())
	if err := s.PutAuto(KindArtist, "mbid-1", "jpg", []byte("img")); err != nil {
		t.Fatalf("PutAuto: %v", err)
	}
	path, ok := s.Get(KindArtist, "mbid-1")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if filepath.Base(path) != "cover.auto.jpg" {
		t.Fatalf("got %q", filepath.Base(path))
	}
	if b, _ := os.ReadFile(path); string(b) != "img" {
		t.Fatalf("bad contents %q", b)
	}
}

func TestManualPreferredOverAuto(t *testing.T) {
	s := New(t.TempDir())
	_ = s.PutAuto(KindArtist, "m", "jpg", []byte("auto"))
	_ = s.PutManual(KindArtist, "m", "png", []byte("manual"))
	path, ok := s.Get(KindArtist, "m")
	if !ok || filepath.Base(path) != "cover.png" {
		t.Fatalf("expected manual cover.png, got ok=%v path=%q", ok, path)
	}
}

func TestPutAutoReplacesOldExtension(t *testing.T) {
	s := New(t.TempDir())
	_ = s.PutAuto(KindArtist, "m", "jpg", []byte("a"))
	_ = s.PutAuto(KindArtist, "m", "png", []byte("b"))
	path, ok := s.Get(KindArtist, "m")
	if !ok || filepath.Base(path) != "cover.auto.png" {
		t.Fatalf("expected cover.auto.png, got %q", path)
	}
	// the stale .jpg must be gone
	if _, ok2 := os.Stat(filepath.Join(filepath.Dir(path), "cover.auto.jpg")); ok2 == nil {
		t.Fatal("stale cover.auto.jpg should have been removed")
	}
}

func TestGetMissing(t *testing.T) {
	s := New(t.TempDir())
	if _, ok := s.Get(KindArtist, "nope"); ok {
		t.Fatal("expected ok=false")
	}
}

func TestInvalidKeyRejected(t *testing.T) {
	s := New(t.TempDir())
	if err := s.PutAuto(KindArtist, "../escape", "jpg", []byte("x")); err == nil {
		t.Fatal("expected error for unsafe key")
	}
	if _, ok := s.Get(KindArtist, "../escape"); ok {
		t.Fatal("expected ok=false for unsafe key")
	}
}

func TestSingleDotKeyRejected(t *testing.T) {
	s := New(t.TempDir())
	// A key of "." must be rejected — it cleans to the kind root, so a
	// Delete would destroy every entity's images.
	if err := s.PutManual(KindArtist, ".", "jpg", []byte("x")); err == nil {
		t.Fatal("expected error for key=\".\"")
	}
	if _, ok := s.Get(KindArtist, "."); ok {
		t.Fatal("expected ok=false for key=\".\"")
	}
	if err := s.Delete(KindArtist, "."); err == nil {
		t.Fatal("expected error for Delete with key=\".\"")
	}
}

func TestDelete(t *testing.T) {
	s := New(t.TempDir())
	_ = s.PutManual(KindRadio, "h", "png", []byte("x"))
	if err := s.Delete(KindRadio, "h"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := s.Get(KindRadio, "h"); ok {
		t.Fatal("expected gone after Delete")
	}
	// Delete is idempotent
	if err := s.Delete(KindRadio, "h"); err != nil {
		t.Fatalf("second Delete: %v", err)
	}
}

// Callers that present the stored image to a user need to know whether it was
// uploaded or auto-fetched — the two are indistinguishable from the path alone
// unless you re-parse the filename.
func TestGetEntryReportsManualVsAuto(t *testing.T) {
	s := New(t.TempDir())

	if _, _, ok := s.GetEntry(KindArtist, "a1"); ok {
		t.Fatal("GetEntry should report not-found for an empty store")
	}

	if err := s.PutAuto(KindArtist, "a1", "jpg", []byte("x")); err != nil {
		t.Fatal(err)
	}
	path, manual, ok := s.GetEntry(KindArtist, "a1")
	if !ok || manual {
		t.Fatalf("auto image: got manual=%v ok=%v", manual, ok)
	}
	if filepath.Base(path) != "cover.auto.jpg" {
		t.Errorf("path = %q, want cover.auto.jpg", filepath.Base(path))
	}

	// A manual upload wins over the auto image and is reported as manual.
	if err := s.PutManual(KindArtist, "a1", "png", []byte("x")); err != nil {
		t.Fatal(err)
	}
	path, manual, ok = s.GetEntry(KindArtist, "a1")
	if !ok || !manual {
		t.Fatalf("manual image: got manual=%v ok=%v", manual, ok)
	}
	if filepath.Base(path) != "cover.png" {
		t.Errorf("path = %q, want cover.png", filepath.Base(path))
	}
}

func TestRekeyMovesEverything(t *testing.T) {
	s := New(t.TempDir())
	// Put a manual cover and an auto cover. Rekey must carry both variants, not
	// just the manual primary a naive read-and-re-Put would copy.
	if err := s.PutManual(KindAlbum, "oldkey", "jpg", []byte("manual-cover")); err != nil {
		t.Fatalf("PutManual cover: %v", err)
	}
	if err := s.PutAuto(KindAlbum, "oldkey", "png", []byte("auto-cover")); err != nil {
		t.Fatalf("PutAuto cover: %v", err)
	}

	if err := s.Rekey(KindAlbum, "oldkey", "newkey"); err != nil {
		t.Fatalf("Rekey: %v", err)
	}

	// Both variants must resolve under newkey with manual-vs-auto intact.
	path, manual, ok := s.GetEntry(KindAlbum, "newkey")
	if !ok || !manual {
		t.Fatalf("newkey cover: manual=%v ok=%v", manual, ok)
	}
	if b, _ := os.ReadFile(path); string(b) != "manual-cover" {
		t.Fatalf("newkey cover contents: got %q, want %q", b, "manual-cover")
	}

	// Manual wins in GetEntry, so we check the auto variant by reading the directory.
	dir, _ := s.entityDir(KindAlbum, "newkey")
	autoFile := filepath.Join(dir, "cover.auto.png")
	if autoB, err := os.ReadFile(autoFile); err != nil || string(autoB) != "auto-cover" {
		t.Fatalf("newkey auto cover: err=%v contents=%q", err, autoB)
	}

	// Nothing should remain under oldkey.
	if _, ok := s.Get(KindAlbum, "oldkey"); ok {
		t.Fatal("oldkey cover should be gone")
	}
}

func TestRekeyRefusesOccupiedDestination(t *testing.T) {
	s := New(t.TempDir())
	_ = s.PutManual(KindAlbum, "src", "jpg", []byte("src-image"))
	_ = s.PutManual(KindAlbum, "dst", "jpg", []byte("dst-image"))

	err := s.Rekey(KindAlbum, "src", "dst")
	if err == nil {
		t.Fatal("expected error for occupied destination")
	}
	if !errors.Is(err, ErrKeyOccupied) {
		t.Fatalf("expected ErrKeyOccupied, got: %v", err)
	}

	// Both sides must be intact.
	srcPath, ok := s.Get(KindAlbum, "src")
	if !ok {
		t.Fatal("src must still exist")
	}
	if b, _ := os.ReadFile(srcPath); string(b) != "src-image" {
		t.Fatalf("src contents changed: %q", b)
	}

	dstPath, ok := s.Get(KindAlbum, "dst")
	if !ok {
		t.Fatal("dst must still exist")
	}
	if b, _ := os.ReadFile(dstPath); string(b) != "dst-image" {
		t.Fatalf("dst contents changed: %q", b)
	}
}

func TestRekeyNoOp(t *testing.T) {
	s := New(t.TempDir())
	_ = s.PutManual(KindAlbum, "k", "jpg", []byte("x"))

	// oldKey == newKey should be a no-op returning nil.
	if err := s.Rekey(KindAlbum, "k", "k"); err != nil {
		t.Fatalf("Rekey same key: %v", err)
	}
	if _, ok := s.Get(KindAlbum, "k"); !ok {
		t.Fatal("asset should still be readable after same-key Rekey")
	}

	// Missing source should return nil (nothing to move).
	if err := s.Rekey(KindAlbum, "nonexistent", "newkey"); err != nil {
		t.Fatalf("Rekey missing source: %v", err)
	}
}

func TestRekeyEmptyDestinationAllowed(t *testing.T) {
	s := New(t.TempDir())
	_ = s.PutManual(KindAlbum, "src", "jpg", []byte("data"))

	// Create an empty directory at the destination.
	dstDir, _ := s.entityDir(KindAlbum, "dst")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Rekey should succeed since the empty directory holds no upload.
	if err := s.Rekey(KindAlbum, "src", "dst"); err != nil {
		t.Fatalf("Rekey with empty dest dir: %v", err)
	}

	if _, ok := s.Get(KindAlbum, "dst"); !ok {
		t.Fatal("asset should be under dst after move")
	}
	if _, ok := s.Get(KindAlbum, "src"); ok {
		t.Fatal("src should be gone")
	}
}

func TestRekeyUnsafeKeys(t *testing.T) {
	s := New(t.TempDir())
	_ = s.PutManual(KindAlbum, "safe", "jpg", []byte("data"))

	// Rekey with an unsafe key (containing ..) should be rejected.
	if err := s.Rekey(KindAlbum, "safe", "../escape"); err == nil {
		t.Fatal("expected error for unsafe newKey")
	}
	if err := s.Rekey(KindAlbum, "../escape", "safe"); err == nil {
		t.Fatal("expected error for unsafe oldKey")
	}

	// The source should be untouched.
	if _, ok := s.Get(KindAlbum, "safe"); !ok {
		t.Fatal("safe key should still exist after rejected Rekey")
	}
}
