package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPATCipherGeneratesAndReloads(t *testing.T) {
	dir := t.TempDir()
	c1, err := loadPATCipher(dir)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	ct, keyID, err := c1.Encrypt("secret", "ctx")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if keyID != patCipherKeyID {
		t.Errorf("keyID = %q, want %q", keyID, patCipherKeyID)
	}
	// A second load must decrypt what the first encrypted (key persisted).
	c2, err := loadPATCipher(dir)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	got, err := c2.Decrypt(ct, keyID, "ctx")
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != "secret" {
		t.Errorf("round-trip = %q", got)
	}
	// The key file exists with owner-only permissions.
	fi, err := os.Stat(filepath.Join(dir, patCipherKeysFile))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("key file mode = %v, want 0600", fi.Mode().Perm())
	}
}

func TestLoadPATCipherRejectsCorruptFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, patCipherKeysFile), []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPATCipher(dir); err == nil {
		t.Error("wrong-size key file must error, not be silently regenerated")
	}
}
