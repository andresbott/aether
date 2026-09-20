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

// A blank FollowSymlinks: key (no value) must be treated as "not declared" —
// the default (true) — not as an explicit false. go-bumbu/config allocates the
// bool pointer while decoding either way, so only asking the handler for the
// key's raw STRING value, and treating a blank one like an absent one, tells
// the two apart (normalizeScanFolderBools).
func TestFollowSymlinksBlankValueDefaultsToTrue(t *testing.T) {
	dir := t.TempDir()
	cfg, err := getAppCfg(writeCfg(t, "ScanFolders:\n  - Name: \"A\"\n    Path: \""+dir+"\"\n    FollowSymlinks:\n"), true)
	if err != nil {
		t.Fatalf("getAppCfg: %v", err)
	}
	if len(cfg.ScanFolders) != 1 {
		t.Fatalf("expected 1 scan folder, got %d", len(cfg.ScanFolders))
	}
	if cfg.ScanFolders[0].FollowSymlinks != nil {
		t.Fatalf("a blank FollowSymlinks: must decode as unset (nil), got %v", *cfg.ScanFolders[0].FollowSymlinks)
	}
	set, err := scanFolderSet(cfg.ScanFolders)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := set.ByName("A")
	if !f.FollowSymlinks {
		t.Fatalf("expected the default (true) to apply to a blank value, got FollowSymlinks=%v", f.FollowSymlinks)
	}
}

// An EXPLICIT false must still yield false: the blank-value fix must not
// swallow a deliberate opt-out.
func TestFollowSymlinksExplicitFalseStaysFalse(t *testing.T) {
	dir := t.TempDir()
	cfg, err := getAppCfg(writeCfg(t, "ScanFolders:\n  - Name: \"A\"\n    Path: \""+dir+"\"\n    FollowSymlinks: false\n"), true)
	if err != nil {
		t.Fatalf("getAppCfg: %v", err)
	}
	if cfg.ScanFolders[0].FollowSymlinks == nil || *cfg.ScanFolders[0].FollowSymlinks {
		t.Fatalf("expected FollowSymlinks=false, got %v", cfg.ScanFolders[0].FollowSymlinks)
	}
	set, err := scanFolderSet(cfg.ScanFolders)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := set.ByName("A")
	if f.FollowSymlinks {
		t.Fatal("expected an explicit FollowSymlinks: false to be honored")
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
	// An EMPTY marker means "not yet stamped" (a brand new row mid-scan), not
	// orphaned: the next scan stamps or removes it, so it must never be warned
	// about either (deferred minor T3).
	db.Create(&model.Track{AlbumID: album.ID, Filename: "3.mp3", FilePath: "/new/3.mp3", ScanFolder: ""})

	presentPath := t.TempDir()
	// Two sibling temp dirs: roots must not nest.
	set, err := scanfolder.NewSet([]scanfolder.Folder{
		{Name: "Present", Path: presentPath, ExcludePatterns: []string{`^\.`}, FollowSymlinks: false},
		{Name: "Unmounted", Path: filepath.Join(t.TempDir(), "missing")},
	})
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	warnScanFolders(slog.New(slog.NewTextHandler(&buf, nil)), s, set)
	out := buf.String()
	warnOut := warnLevelLines(out)

	if !strings.Contains(warnOut, "Unmounted") || !strings.Contains(warnOut, "unavailable") || !strings.Contains(warnOut, "not usable") {
		t.Errorf("expected an unavailable-directory warning for Unmounted, got:\n%s", out)
	}
	if !strings.Contains(warnOut, "Removed") || !strings.Contains(warnOut, "no longer configured") {
		t.Errorf("expected an orphaned-marker warning for Removed, got:\n%s", out)
	}
	if strings.Contains(warnOut, "scan_folder=Present") {
		t.Errorf("a configured, available folder must not be warned about, got:\n%s", out)
	}
	if strings.Contains(warnOut, `scan_folder=""`) {
		t.Errorf("a track with an empty (not yet stamped) marker must never be warned about, got:\n%s", out)
	}

	// One Info line per configured folder — name, path, exclude-pattern count,
	// follow-symlinks — so a silently misread config is visible.
	if !strings.Contains(out, "scan folder loaded") ||
		!strings.Contains(out, "scan_folder=Present") || !strings.Contains(out, "path="+presentPath) ||
		!strings.Contains(out, "exclude_patterns=1") || !strings.Contains(out, "follow_symlinks=false") {
		t.Errorf("expected a per-folder Info line for Present, got:\n%s", out)
	}
}

// TestWarnDanglingLibraryFilters proves the startup WARN that names a library
// whose scan_folder filter no longer matches a configured folder — what a
// renamed or removed scan folder leaves behind (see Dangling in
// internal/libraryfilter). A library filtered on a folder that IS configured
// must stay silent.
func TestWarnDanglingLibraryFilters(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := store.New(db)
	db.Create(&model.Library{Name: "Ghosts", Filters: []model.LibraryFilter{
		{Field: model.FilterScanFolder, Values: []string{"Gone"}},
	}})
	db.Create(&model.Library{Name: "Healthy", Filters: []model.LibraryFilter{
		{Field: model.FilterScanFolder, Values: []string{"Present"}},
	}})

	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Present", Path: t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	warnDanglingLibraryFilters(slog.New(slog.NewTextHandler(&buf, nil)), s, set)
	out := buf.String()

	ghostLine := lineContaining(out, "library=Ghosts")
	if ghostLine == "" || !strings.Contains(ghostLine, "Gone") || !strings.Contains(ghostLine, "matches nothing") {
		t.Errorf("expected a WARN naming Ghosts and its dangling Gone filter, got:\n%s", out)
	}
	if strings.Contains(out, "Healthy") {
		t.Errorf("a library whose filter matches a configured folder must not be warned about, got:\n%s", out)
	}
}

// lineContaining returns the first line of a slog dump that contains want, so a
// test can assert what ONE message says rather than what the whole dump does —
// the zero-folders Info line and the orphan WARN share phrases.
func lineContaining(out, want string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, want) {
			return line
		}
	}
	return ""
}

// warnLevelLines returns only the WARN-level lines of a slog text-handler
// dump, so a test can assert something is (or is not) warned about without
// tripping on the unconditional per-folder Info line, which also names every
// configured folder.
func warnLevelLines(out string) string {
	var b strings.Builder
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "level=WARN") {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// With no scan folder configured at all, warnScanFolders must say so plainly
// and must not promise a removal that cannot happen — with nothing configured
// to walk, Scan returns before Cleanup ever runs.
func TestWarnScanFoldersWithNoneConfigured(t *testing.T) {
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
	db.Create(&model.Track{AlbumID: album.ID, Filename: "1.mp3", FilePath: "/old/1.mp3", ScanFolder: "Orphaned"})

	set, err := scanfolder.NewSet(nil)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	warnScanFolders(slog.New(slog.NewTextHandler(&buf, nil)), s, set)
	out := buf.String()

	info := lineContaining(out, "no scan folders configured")
	if !strings.Contains(info, "scans do nothing") {
		t.Errorf("expected the no-scan-folders-configured Info line, got:\n%s", out)
	}
	// Failing closed must not be silent: an operator whose config names no scan
	// folder gets a server that lists everything and plays nothing, so the line
	// that says scans do nothing must say that too.
	if !strings.Contains(info, "no on-disk media is served") {
		t.Errorf("the no-scan-folders Info line must say no on-disk media is served, got:\n%s", out)
	}
	if !strings.Contains(out, "Orphaned") || !strings.Contains(out, "neither re-stamped nor removed") ||
		!strings.Contains(out, "cannot be played") {
		t.Errorf("expected the orphaned-marker warning to say scans do nothing and the tracks cannot be played, got:\n%s", out)
	}
	if strings.Contains(out, "will remove") || strings.Contains(out, "removed otherwise") {
		t.Errorf("with no folder configured, the warning must not promise a removal, got:\n%s", out)
	}
}
