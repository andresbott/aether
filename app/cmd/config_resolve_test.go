package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveConfigFile covers the packaged-vs-development lookup: an explicit
// -c is mandatory so typos fail loudly, while the implicit search falls through
// the working directory to /etc/aether and finally to defaults-only.
func TestResolveConfigFile(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, "config.yaml")
	etc := filepath.Join(dir, "etc-config.yaml")
	if err := os.WriteFile(etc, []byte("DataDir: \"/var/lib/aether\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		flag          string
		paths         []string
		wantPath      string
		wantMandatory bool
	}{
		{
			name:          "explicit flag wins and is mandatory",
			flag:          "/nonexistent/custom.yaml",
			paths:         []string{etc},
			wantPath:      "/nonexistent/custom.yaml",
			wantMandatory: true,
		},
		{
			name:          "first existing search path is used",
			paths:         []string{local, etc},
			wantPath:      etc,
			wantMandatory: false,
		},
		{
			name:          "no config found means defaults only",
			paths:         []string{local, filepath.Join(dir, "missing.yaml")},
			wantPath:      "",
			wantMandatory: false,
		},
		{
			name:          "a directory in the search path is skipped",
			paths:         []string{dir, etc},
			wantPath:      etc,
			wantMandatory: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path, mandatory := resolveConfigFile(tc.flag, tc.paths)
			if path != tc.wantPath {
				t.Errorf("path: got %q, want %q", path, tc.wantPath)
			}
			if mandatory != tc.wantMandatory {
				t.Errorf("mandatory: got %v, want %v", mandatory, tc.wantMandatory)
			}
		})
	}
}

// TestConfigSearchPathOrder pins the documented precedence: the working
// directory (development) before the packaged /etc location.
func TestConfigSearchPathOrder(t *testing.T) {
	want := []string{"./config.yaml", "/etc/aether/config.yaml"}
	if len(configSearchPaths) != len(want) {
		t.Fatalf("got %v, want %v", configSearchPaths, want)
	}
	for i := range want {
		if configSearchPaths[i] != want[i] {
			t.Fatalf("got %v, want %v", configSearchPaths, want)
		}
	}
}

// TestGetAppCfgMissingMandatoryFile ensures a bad -c path is an error rather
// than a silent fall-through to defaults.
func TestGetAppCfgMissingMandatoryFile(t *testing.T) {
	if _, err := getAppCfg(filepath.Join(t.TempDir(), "nope.yaml"), true); err == nil {
		t.Fatal("expected an error for a missing mandatory config file")
	}
}

// TestGetAppCfgNoFileUsesDefaults verifies the empty-path case yields the
// built-in defaults (DataDir made absolute).
func TestGetAppCfgNoFileUsesDefaults(t *testing.T) {
	cfg, err := getAppCfg("", false)
	if err != nil {
		t.Fatalf("getAppCfg: %v", err)
	}
	if cfg.Server.Port != defaultCfg.Server.Port {
		t.Errorf("port: got %d, want %d", cfg.Server.Port, defaultCfg.Server.Port)
	}
	if !filepath.IsAbs(cfg.DataDir) {
		t.Errorf("DataDir should be absolute, got %q", cfg.DataDir)
	}
}
