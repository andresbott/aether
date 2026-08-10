package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// writeCfg writes a config file into a temp dir and returns its path.
func writeCfg(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLibrariesLoadedFromFile(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	path := writeCfg(t, `
Libraries:
  - Name: "Rock"
    Path: "`+dirA+`"
    ExcludePatterns:
      - ".*/covers/.*"
      - "^\\..*"
    FollowSymlinks: false
    ShowArtists: false
    DefaultView: "artists"
    Icon: "folder-open"
    CoverStyle: "bauhaus"
  - Name: "Jazz"
    Path: "`+dirB+`"
`)
	cfg, err := getAppCfg(path, true)
	if err != nil {
		t.Fatalf("getAppCfg: %v", err)
	}
	if len(cfg.Libraries) != 2 {
		t.Fatalf("expected 2 libraries, got %d: %+v", len(cfg.Libraries), cfg.Libraries)
	}

	rock := cfg.Libraries[0]
	if rock.Name != "Rock" || rock.Path != dirA {
		t.Fatalf("unexpected first library: %+v", rock)
	}
	if len(rock.ExcludePatterns) != 2 || rock.ExcludePatterns[0] != ".*/covers/.*" {
		t.Fatalf("exclude patterns not loaded: %+v", rock.ExcludePatterns)
	}
	if rock.FollowSymlinks == nil || *rock.FollowSymlinks {
		t.Fatalf("expected FollowSymlinks=false, got %v", rock.FollowSymlinks)
	}
	if rock.ShowArtists == nil || *rock.ShowArtists {
		t.Fatalf("expected ShowArtists=false, got %v", rock.ShowArtists)
	}
	if rock.DefaultView != "artists" || rock.Icon != "folder-open" || rock.CoverStyle != "bauhaus" {
		t.Fatalf("display fields not loaded: %+v", rock)
	}

	// The second entry omits every optional field: the bools must stay unset so
	// the reconcile step can apply their "true" defaults.
	jazz := cfg.Libraries[1]
	if jazz.Name != "Jazz" || jazz.Path != dirB {
		t.Fatalf("unexpected second library: %+v", jazz)
	}
	if jazz.FollowSymlinks != nil {
		t.Fatalf("expected FollowSymlinks unset, got %v", *jazz.FollowSymlinks)
	}
	if jazz.ShowArtists != nil {
		t.Fatalf("expected ShowArtists unset, got %v", *jazz.ShowArtists)
	}
}

func TestNoLibrariesSectionYieldsNone(t *testing.T) {
	cfg, err := getAppCfg(writeCfg(t, "DataDir: \"./data\"\n"), true)
	if err != nil {
		t.Fatalf("getAppCfg: %v", err)
	}
	if len(cfg.Libraries) != 0 {
		t.Fatalf("expected no libraries, got %+v", cfg.Libraries)
	}
}
