package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// TestArtistImagesKeyLoadedFromFileAndTrimmed verifies that an API key given as
// "@<path>" in the config is loaded from that file's content and trimmed of the
// trailing newline the file-read returns verbatim.
func TestArtistImagesKeyLoadedFromFileAndTrimmed(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "fanart.api.key")
	if err := os.WriteFile(keyFile, []byte("secret-123\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgFile := filepath.Join(dir, "config.yaml")
	content := "ArtistImages:\n  FanartApiKey: \"@" + keyFile + "\"\n"
	if err := os.WriteFile(cfgFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := getAppCfg(cfgFile, true)
	if err != nil {
		t.Fatalf("getAppCfg: %v", err)
	}
	if cfg.ArtistImages.FanartApiKey != "secret-123" {
		t.Fatalf("expected key loaded from file and trimmed to %q, got %q", "secret-123", cfg.ArtistImages.FanartApiKey)
	}
}
