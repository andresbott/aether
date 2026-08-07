package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSessionKeys(t *testing.T) {
	t.Run("generates and persists keys on first start", func(t *testing.T) {
		dir := t.TempDir()
		hash1, block1, err := loadSessionKeys(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(hash1) != sessionHashKeyLen || len(block1) != sessionBlockKeyLen {
			t.Fatalf("key lengths = %d/%d, want %d/%d", len(hash1), len(block1), sessionHashKeyLen, sessionBlockKeyLen)
		}

		// A second load (server restart) must return the same keys, or every
		// session cookie dies on restart.
		hash2, block2, err := loadSessionKeys(dir)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(hash1, hash2) || !bytes.Equal(block1, block2) {
			t.Error("keys changed across loads; sessions would not survive a restart")
		}
	})

	t.Run("rejects a truncated keys file instead of regenerating", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, sessionKeysFile), []byte("short"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := loadSessionKeys(dir); err == nil {
			t.Fatal("expected an error for a corrupt keys file")
		}
	})
}
