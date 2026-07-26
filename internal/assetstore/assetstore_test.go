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

func TestNamedEntriesCoexist(t *testing.T) {
	s := New(t.TempDir())
	_ = s.PutManual(KindAlbum, "1", "jpg", []byte("front"))
	if err := s.PutManualNamed(KindAlbum, "1", "back", "png", []byte("back")); err != nil {
		t.Fatalf("PutManualNamed: %v", err)
	}
	if p, ok := s.Get(KindAlbum, "1"); !ok || filepath.Base(p) != "cover.jpg" {
		t.Fatalf("front cover: ok=%v path=%q", ok, p)
	}
	p, ok := s.GetNamed(KindAlbum, "1", "back")
	if !ok || filepath.Base(p) != "back.png" {
		t.Fatalf("back: ok=%v path=%q", ok, p)
	}
	if b, _ := os.ReadFile(p); string(b) != "back" {
		t.Fatalf("bad contents %q", b)
	}
}

func TestNamedGetNoPrefixConfusion(t *testing.T) {
	s := New(t.TempDir())
	_ = s.PutManualNamed(KindAlbum, "1", "back", "jpg", []byte("x"))
	if _, ok := s.GetNamed(KindAlbum, "1", "b"); ok {
		t.Fatal("entry name must match exactly, not by prefix")
	}
	if _, ok := s.GetNamed(KindAlbum, "1", "backdrop"); ok {
		t.Fatal("entry name must match exactly")
	}
}

func TestDeleteNamedLeavesOtherEntries(t *testing.T) {
	s := New(t.TempDir())
	_ = s.PutManual(KindAlbum, "1", "jpg", []byte("front"))
	_ = s.PutAuto(KindAlbum, "1", "jpg", []byte("front-auto"))
	_ = s.PutManualNamed(KindAlbum, "1", "back", "jpg", []byte("back"))
	if err := s.DeleteNamed(KindAlbum, "1", "cover"); err != nil {
		t.Fatalf("DeleteNamed: %v", err)
	}
	if _, ok := s.Get(KindAlbum, "1"); ok {
		t.Fatal("cover (manual and auto) should be gone")
	}
	if _, ok := s.GetNamed(KindAlbum, "1", "back"); !ok {
		t.Fatal("back entry must survive deleting the cover entry")
	}
	// idempotent, also on a missing entity dir
	if err := s.DeleteNamed(KindAlbum, "1", "cover"); err != nil {
		t.Fatalf("second DeleteNamed: %v", err)
	}
	if err := s.DeleteNamed(KindAlbum, "nope", "cover"); err != nil {
		t.Fatalf("DeleteNamed on missing entity: %v", err)
	}
}

func TestNamedRejectsUnsafeNames(t *testing.T) {
	s := New(t.TempDir())
	for _, name := range []string{"a.b", "cover.auto", "", "../x"} {
		if err := s.PutManualNamed(KindAlbum, "1", name, "jpg", []byte("x")); err == nil {
			t.Fatalf("expected error for name %q", name)
		}
		if _, ok := s.GetNamed(KindAlbum, "1", name); ok {
			t.Fatalf("expected ok=false for name %q", name)
		}
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

// Clearing a user's upload must leave the auto-fetched image behind: the two are
// separate variants of the same entry, and only the upload is the user's to drop.
func TestDeleteManualKeepsAutoVariant(t *testing.T) {
	s := New(t.TempDir())
	if err := s.PutAuto(KindArtist, "a1", "jpg", []byte("auto")); err != nil {
		t.Fatal(err)
	}
	if err := s.PutManual(KindArtist, "a1", "png", []byte("manual")); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteManual(KindArtist, "a1"); err != nil {
		t.Fatalf("DeleteManual: %v", err)
	}

	path, manual, ok := s.GetEntry(KindArtist, "a1")
	if !ok {
		t.Fatal("auto variant was deleted too")
	}
	if manual {
		t.Errorf("manual variant survived: %s", path)
	}
	if filepath.Base(path) != "cover.auto.jpg" {
		t.Errorf("path = %q, want cover.auto.jpg", filepath.Base(path))
	}
}

// Nothing to delete is not an error — a clear on an entity with no upload (or no
// directory at all) is a no-op.
func TestDeleteManualIsIdempotent(t *testing.T) {
	s := New(t.TempDir())
	if err := s.DeleteManual(KindArtist, "missing"); err != nil {
		t.Fatalf("DeleteManual on empty store: %v", err)
	}
	if err := s.PutAuto(KindArtist, "a1", "jpg", []byte("auto")); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteManual(KindArtist, "a1"); err != nil {
		t.Fatalf("DeleteManual with no upload: %v", err)
	}
	if _, _, ok := s.GetEntry(KindArtist, "a1"); !ok {
		t.Fatal("auto variant removed by a no-op clear")
	}
}
