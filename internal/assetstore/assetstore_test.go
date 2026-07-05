package assetstore

import (
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
