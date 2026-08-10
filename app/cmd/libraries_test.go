package cmd

import (
	"io"
	"log/slog"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return store.New(db)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// reconcile runs the startup reconcile and fails the test on error.
func reconcile(t *testing.T, s *store.Store, libs ...LibraryCfg) {
	t.Helper()
	if err := reconcileLibraries(s, libs, discardLogger()); err != nil {
		t.Fatalf("reconcileLibraries: %v", err)
	}
}

func libByName(t *testing.T, s *store.Store, name string) model.Library {
	t.Helper()
	lib, err := s.FindLibraryByName(name)
	if err != nil {
		t.Fatalf("library %q not found: %v", name, err)
	}
	return lib
}

func TestReconcileCreatesConfigLibrary(t *testing.T) {
	s := testStore(t)
	dir := t.TempDir()
	no := false
	reconcile(t, s, LibraryCfg{
		Name:            "Rock",
		Path:            dir,
		ExcludePatterns: []string{".*/covers/.*"},
		FollowSymlinks:  &no,
		ShowArtists:     &no,
		DefaultView:     "artists",
		Icon:            "folder-open",
		CoverStyle:      "bauhaus",
	})

	lib := libByName(t, s, "Rock")
	if lib.Source != model.SourceConfig {
		t.Fatalf("expected source %q, got %q", model.SourceConfig, lib.Source)
	}
	if lib.Path != dir {
		t.Fatalf("expected path %q, got %q", dir, lib.Path)
	}
	if lib.ExcludePatterns != `[".*/covers/.*"]` {
		t.Fatalf("unexpected exclude patterns: %q", lib.ExcludePatterns)
	}
	if lib.FollowSymlinks {
		t.Fatal("expected FollowSymlinks=false")
	}
	if !lib.HideArtists {
		t.Fatal("expected HideArtists=true (ShowArtists false)")
	}
	if lib.DefaultView != "artists" || lib.Icon != "folder-open" || lib.CoverStyle != "bauhaus" {
		t.Fatalf("display fields not applied: %+v", lib)
	}
}

func TestReconcileAppliesDefaultsForOmittedFields(t *testing.T) {
	s := testStore(t)
	reconcile(t, s, LibraryCfg{Name: "Jazz", Path: t.TempDir()})

	lib := libByName(t, s, "Jazz")
	if !lib.FollowSymlinks {
		t.Fatal("expected FollowSymlinks to default to true")
	}
	if lib.HideArtists {
		t.Fatal("expected artists visible by default")
	}
	if lib.DefaultView != "albums" || lib.Icon != "folder" || lib.CoverStyle != "auto" {
		t.Fatalf("expected defaults, got %+v", lib)
	}
	if lib.ExcludePatterns != "" {
		t.Fatalf("expected no exclude patterns, got %q", lib.ExcludePatterns)
	}
}

// The two sources are additive: a UI-created library and a config one coexist.
func TestReconcileLeavesDBLibrariesAlone(t *testing.T) {
	s := testStore(t)
	mine := &model.Library{Name: "Jazz", Path: t.TempDir(), Icon: "star", Source: model.SourceDB}
	if err := s.CreateLibrary(mine); err != nil {
		t.Fatal(err)
	}

	reconcile(t, s, LibraryCfg{Name: "Rock", Path: t.TempDir()})

	libs, err := s.ListLibraries()
	if err != nil {
		t.Fatal(err)
	}
	if len(libs) != 2 {
		t.Fatalf("expected both libraries, got %d", len(libs))
	}
	if got := libByName(t, s, "Jazz"); got.Source != model.SourceDB || got.Icon != "star" {
		t.Fatalf("UI-created library was modified: %+v", got)
	}
	if got := libByName(t, s, "Rock"); got.Source != model.SourceConfig {
		t.Fatalf("expected config source, got %q", got.Source)
	}
}

// Re-running with the same config must not duplicate rows, and must overwrite
// any drift on the managed row.
func TestReconcileIsIdempotentAndOverwritesDrift(t *testing.T) {
	s := testStore(t)
	dir := t.TempDir()
	cfg := LibraryCfg{Name: "Rock", Path: dir, Icon: "folder-open"}
	reconcile(t, s, cfg)

	// Simulate drift: something changed the row behind config's back.
	drifted := libByName(t, s, "Rock")
	drifted.Icon = "star"
	drifted.DefaultView = "artists"
	if err := s.UpdateLibrary(&drifted); err != nil {
		t.Fatal(err)
	}

	reconcile(t, s, cfg)

	libs, err := s.ListLibraries()
	if err != nil {
		t.Fatal(err)
	}
	if len(libs) != 1 {
		t.Fatalf("expected 1 library after re-reconcile, got %d", len(libs))
	}
	lib := libs[0]
	if lib.ID != drifted.ID {
		t.Fatalf("row was replaced: id %d -> %d", drifted.ID, lib.ID)
	}
	if lib.Icon != "folder-open" {
		t.Fatalf("config did not win over drift: icon=%q", lib.Icon)
	}
	if lib.DefaultView != "albums" {
		t.Fatalf("a field dropped from config should revert to its default, got %q", lib.DefaultView)
	}
}

// Config wins over a colliding UI-created library by adopting the row, so its
// scanned tracks survive.
func TestReconcileAdoptsCollidingDBLibraryByPath(t *testing.T) {
	s := testStore(t)
	dir := t.TempDir()
	mine := &model.Library{Name: "My Rock", Path: dir, Source: model.SourceDB}
	if err := s.CreateLibrary(mine); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().Create(&model.Track{
		LibraryID: mine.ID, Filename: "a.mp3", FilePath: dir + "/a.mp3",
	}).Error; err != nil {
		t.Fatal(err)
	}

	reconcile(t, s, LibraryCfg{Name: "Rock", Path: dir})

	libs, err := s.ListLibraries()
	if err != nil {
		t.Fatal(err)
	}
	if len(libs) != 1 {
		t.Fatalf("expected the row to be adopted, not duplicated; got %d libraries", len(libs))
	}
	lib := libs[0]
	if lib.ID != mine.ID {
		t.Fatalf("expected to adopt library %d, got %d", mine.ID, lib.ID)
	}
	if lib.Source != model.SourceConfig || lib.Name != "Rock" {
		t.Fatalf("adopted row not updated from config: %+v", lib)
	}
	count, err := s.CountTracksForLibrary(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("adopting an unchanged path must keep tracks, got %d", count)
	}
}

func TestReconcileAdoptsCollidingDBLibraryByName(t *testing.T) {
	s := testStore(t)
	oldDir := t.TempDir()
	newDir := t.TempDir()
	mine := &model.Library{Name: "Rock", Path: oldDir, Source: model.SourceDB}
	if err := s.CreateLibrary(mine); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().Create(&model.Track{
		LibraryID: mine.ID, Filename: "a.mp3", FilePath: oldDir + "/a.mp3",
	}).Error; err != nil {
		t.Fatal(err)
	}

	reconcile(t, s, LibraryCfg{Name: "Rock", Path: newDir})

	lib := libByName(t, s, "Rock")
	if lib.ID != mine.ID {
		t.Fatalf("expected to adopt library %d, got %d", mine.ID, lib.ID)
	}
	if lib.Path != newDir {
		t.Fatalf("expected path from config %q, got %q", newDir, lib.Path)
	}
	// A path change wipes tracks, exactly as it does through the API.
	count, err := s.CountTracksForLibrary(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected tracks wiped on path change, got %d", count)
	}
}

// Name matches one row and path another: unresolvable, so it must fail loudly.
func TestReconcileAmbiguousCollisionFails(t *testing.T) {
	s := testStore(t)
	dirA := t.TempDir()
	dirB := t.TempDir()
	if err := s.CreateLibrary(&model.Library{Name: "Rock", Path: dirA, Source: model.SourceDB}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateLibrary(&model.Library{Name: "Other", Path: dirB, Source: model.SourceDB}); err != nil {
		t.Fatal(err)
	}

	err := reconcileLibraries(s, []LibraryCfg{{Name: "Rock", Path: dirB}}, discardLogger())
	if err == nil {
		t.Fatal("expected an error for an ambiguous collision")
	}
}

// Dropping an entry from config keeps the library, but hands it back to the UI.
func TestReconcileReleasesLibraryDroppedFromConfig(t *testing.T) {
	s := testStore(t)
	dir := t.TempDir()
	reconcile(t, s, LibraryCfg{Name: "Rock", Path: dir})
	created := libByName(t, s, "Rock")
	if err := s.DB().Create(&model.Track{
		LibraryID: created.ID, Filename: "a.mp3", FilePath: dir + "/a.mp3",
	}).Error; err != nil {
		t.Fatal(err)
	}

	// Next startup: config no longer declares it.
	reconcile(t, s)

	lib := libByName(t, s, "Rock")
	if lib.ID != created.ID {
		t.Fatalf("library was replaced: %d -> %d", created.ID, lib.ID)
	}
	if lib.Source != model.SourceDB {
		t.Fatalf("expected the row to become UI-owned, got source %q", lib.Source)
	}
	count, err := s.CountTracksForLibrary(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("a config edit must not destroy tracks, got %d", count)
	}
}

func TestReconcileRejectsInvalidConfigLibrary(t *testing.T) {
	tests := []struct {
		name string
		cfg  LibraryCfg
	}{
		{"missing path", LibraryCfg{Name: "X", Path: "/nonexistent-aether-cfg-xyz"}},
		{"bad regex", LibraryCfg{Name: "X", ExcludePatterns: []string{"["}}},
		{"bad default view", LibraryCfg{Name: "X", DefaultView: "songs"}},
		{"bad icon", LibraryCfg{Name: "X", Icon: "Not An Icon"}},
		{"bad cover style", LibraryCfg{Name: "X", CoverStyle: "nope"}},
		{"empty name", LibraryCfg{Name: ""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testStore(t)
			cfg := tc.cfg
			if cfg.Path == "" {
				cfg.Path = t.TempDir()
			}
			if err := reconcileLibraries(s, []LibraryCfg{cfg}, discardLogger()); err == nil {
				t.Fatal("expected a validation error")
			}
			libs, err := s.ListLibraries()
			if err != nil {
				t.Fatal(err)
			}
			if len(libs) != 0 {
				t.Fatalf("an invalid config entry must not create a row, got %d", len(libs))
			}
		})
	}
}
