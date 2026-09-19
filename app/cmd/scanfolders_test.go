package cmd

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
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

func TestScanFoldersLoadedFromFile(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	path := writeCfg(t, `
ScanFolders:
  - Name: "Rock"
    Path: "`+dirA+`"
    ExcludePatterns:
      - ".*/covers/.*"
      - "^\\..*"
    FollowSymlinks: false
  - Name: "Jazz"
    Path: "`+dirB+`"
`)
	cfg, err := getAppCfg(path, true)
	if err != nil {
		t.Fatalf("getAppCfg: %v", err)
	}
	if len(cfg.ScanFolders) != 2 {
		t.Fatalf("expected 2 scan folders, got %d: %+v", len(cfg.ScanFolders), cfg.ScanFolders)
	}
	rock := cfg.ScanFolders[0]
	if rock.Name != "Rock" || rock.Path != dirA {
		t.Fatalf("unexpected first folder: %+v", rock)
	}
	if len(rock.ExcludePatterns) != 2 || rock.ExcludePatterns[0] != ".*/covers/.*" {
		t.Fatalf("exclude patterns not loaded: %+v", rock.ExcludePatterns)
	}
	if rock.FollowSymlinks == nil || *rock.FollowSymlinks {
		t.Fatalf("expected FollowSymlinks=false, got %v", rock.FollowSymlinks)
	}
	// The second entry omits the optional bool: it must stay unset so the "true"
	// default applies (go-bumbu/config allocates every pointer it walks).
	if jazz := cfg.ScanFolders[1]; jazz.FollowSymlinks != nil {
		t.Fatalf("expected FollowSymlinks unset, got %v", *jazz.FollowSymlinks)
	}

	set, err := scanFolderSet(cfg.ScanFolders)
	if err != nil {
		t.Fatal(err)
	}
	jazz, _ := set.ByName("Jazz")
	rockF, _ := set.ByName("Rock")
	if !jazz.FollowSymlinks || rockF.FollowSymlinks {
		t.Fatalf("FollowSymlinks defaults wrong: Jazz=%v (want true), Rock=%v (want false)", jazz.FollowSymlinks, rockF.FollowSymlinks)
	}
}

func TestNoScanFoldersSectionYieldsNone(t *testing.T) {
	cfg, err := getAppCfg(writeCfg(t, "DataDir: \"./data\"\n"), true)
	if err != nil {
		t.Fatalf("getAppCfg: %v", err)
	}
	if len(cfg.ScanFolders) != 0 {
		t.Fatalf("expected no scan folders, got %+v", cfg.ScanFolders)
	}
}

// A structural mistake must fail at load, as loudly as a bad request would.
func TestScanFoldersConfigErrorsFailTheLoad(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"duplicate name": "ScanFolders:\n  - Name: \"M\"\n    Path: \"" + dir + "/a\"\n  - Name: \"M\"\n    Path: \"" + dir + "/b\"\n",
		"nested roots":   "ScanFolders:\n  - Name: \"A\"\n    Path: \"" + dir + "\"\n  - Name: \"B\"\n    Path: \"" + dir + "/kids\"\n",
		"missing path":   "ScanFolders:\n  - Name: \"A\"\n",
		"bad regex":      "ScanFolders:\n  - Name: \"A\"\n    Path: \"" + dir + "\"\n    ExcludePatterns:\n      - \"(\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := getAppCfg(writeCfg(t, body), true)
			if err == nil || !strings.Contains(err.Error(), "ScanFolders") {
				t.Fatalf("err = %v, want a ScanFolders config error", err)
			}
		})
	}
}

// A directory that is not there yet (an unmounted share) is NOT a config error.
func TestScanFolderWithMissingDirectoryLoads(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "not-mounted")
	if _, err := getAppCfg(writeCfg(t, "ScanFolders:\n  - Name: \"A\"\n    Path: \""+missing+"\"\n"), true); err != nil {
		t.Fatalf("a missing directory must not fail the load: %v", err)
	}
}

func TestWarnScanFolders(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := store.New(db)
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	db.Create(&model.Track{AlbumID: album.ID, Filename: "1.mp3", FilePath: "/old/1.mp3", ScanFolder: "Removed"})
	db.Create(&model.Track{AlbumID: album.ID, Filename: "2.mp3", FilePath: "/ok/2.mp3", ScanFolder: "Present"})

	// Two sibling temp dirs: roots must not nest.
	set, err := scanfolder.NewSet([]scanfolder.Folder{
		{Name: "Present", Path: t.TempDir()},
		{Name: "Unmounted", Path: filepath.Join(t.TempDir(), "missing")},
	})
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	warnScanFolders(slog.New(slog.NewTextHandler(&buf, nil)), s, set)
	out := buf.String()

	if !strings.Contains(out, "Unmounted") || !strings.Contains(out, "unavailable") {
		t.Errorf("expected an unavailable-directory warning for Unmounted, got:\n%s", out)
	}
	if !strings.Contains(out, "Removed") || !strings.Contains(out, "no longer configured") {
		t.Errorf("expected an orphaned-marker warning for Removed, got:\n%s", out)
	}
	if strings.Contains(out, "scan_folder=Present") {
		t.Errorf("a configured, available folder must not be warned about, got:\n%s", out)
	}
}
