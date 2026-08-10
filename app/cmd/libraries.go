package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/andresbott/aether/app/router/handlers/libraries"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"gorm.io/gorm"
)

// reconcileLibraries materializes the config file's Libraries list into the
// libraries table, so that the rest of the app keeps reading libraries from one
// place (scanner, /rest getMusicFolders, per-library SQL joins) and a config
// library gets a real ID the tracks table can point at.
//
// The two sources are additive: the table ends up holding the union of
// config-declared libraries and ones created in the admin UI. Where they
// collide, config wins — an existing UI-created row matching a declared name or
// path is adopted (flagged config-owned, fields overwritten) rather than
// duplicated, keeping its already-scanned tracks.
//
// Config libraries are rewritten from the file on every startup, which is what
// makes them read-only over the API: an API write would be reverted on the next
// boot. Dropping an entry from config therefore does NOT delete its library; the
// row is handed back to the UI as an ordinary editable one, tracks intact. A
// config edit never destroys scanned data — deleting a library stays a
// deliberate UI action.
func reconcileLibraries(s *store.Store, cfgLibs []LibraryCfg, l *slog.Logger) error {
	declared := make(map[uint]bool, len(cfgLibs))
	for i := range cfgLibs {
		lib, err := resolveConfigLibrary(s, cfgLibs[i])
		if err != nil {
			return err
		}
		created := lib.ID == 0
		if err := saveConfigLibrary(s, lib); err != nil {
			return fmt.Errorf("library %q from config: %w", cfgLibs[i].Name, err)
		}
		declared[lib.ID] = true
		if created {
			l.Info("provisioned library from config",
				slog.String("component", "startup"),
				slog.String("library", lib.Name), slog.String("path", lib.Path))
		}
	}
	return releaseUndeclaredLibraries(s, declared, l)
}

// resolveConfigLibrary maps one config entry onto the row it should write:
// either an existing library it matches (by path, else by name) or a fresh one.
// The two lookups can disagree — a declared name on one row and the declared
// path on another — and there is no safe way to guess which the admin meant, so
// that ambiguity is reported instead of resolved.
func resolveConfigLibrary(s *store.Store, cfg LibraryCfg) (*model.Library, error) {
	if err := validateConfigLibrary(cfg); err != nil {
		return nil, err
	}

	byPath, pathErr := s.FindLibraryByPath(cfg.Path)
	if pathErr != nil && !errors.Is(pathErr, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("look up library by path %q: %w", cfg.Path, pathErr)
	}
	byName, nameErr := s.FindLibraryByName(cfg.Name)
	if nameErr != nil && !errors.Is(nameErr, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("look up library by name %q: %w", cfg.Name, nameErr)
	}

	pathFound := pathErr == nil
	nameFound := nameErr == nil
	if pathFound && nameFound && byPath.ID != byName.ID {
		return nil, fmt.Errorf(
			"config library %q (%s) is ambiguous: the name belongs to library %d (%s) "+
				"and the path to library %d (%s); rename or remove one of them",
			cfg.Name, cfg.Path, byName.ID, byName.Path, byPath.ID, byPath.Name)
	}

	existing := &model.Library{}
	switch {
	case pathFound:
		existing = &byPath
	case nameFound:
		existing = &byName
	}
	applyConfigLibrary(existing, cfg)
	return existing, nil
}

// validateConfigLibrary holds a config entry to the same rules as the API, so a
// typo in the file fails at startup with the same message a bad request gets
// rather than landing an unusable row in the table.
func validateConfigLibrary(cfg LibraryCfg) error {
	if err := libraries.ValidateName(cfg.Name); err != nil {
		return fmt.Errorf("config library %q: %w", cfg.Name, err)
	}
	if _, err := libraries.ValidatePath(cfg.Path); err != nil {
		return fmt.Errorf("config library %q: %w", cfg.Name, err)
	}
	if err := libraries.ValidateExcludePatterns(cfg.ExcludePatterns); err != nil {
		return fmt.Errorf("config library %q: %w", cfg.Name, err)
	}
	if err := libraries.ValidateDefaultView(cfg.DefaultView); err != nil {
		return fmt.Errorf("config library %q: %w", cfg.Name, err)
	}
	if err := libraries.ValidateIcon(cfg.Icon); err != nil {
		return fmt.Errorf("config library %q: %w", cfg.Name, err)
	}
	if err := libraries.ValidateCoverStyle(cfg.CoverStyle); err != nil {
		return fmt.Errorf("config library %q: %w", cfg.Name, err)
	}
	return nil
}

// applyConfigLibrary copies a config entry onto a library row. Every
// configurable field is written — including the ones the entry omits, which get
// their defaults — because config owns the whole row: a field dropped from the
// file must revert to its default rather than keep whatever was last there.
// LastScanStartedAt is left alone; it is runtime state, not configuration.
func applyConfigLibrary(lib *model.Library, cfg LibraryCfg) {
	lib.Name = cfg.Name
	lib.Path = cfg.Path
	lib.ExcludePatterns = encodeExcludes(cfg.ExcludePatterns)
	lib.FollowSymlinks = boolOr(cfg.FollowSymlinks, true)
	lib.HideArtists = !boolOr(cfg.ShowArtists, true)
	lib.DefaultView = orDefault(cfg.DefaultView, "albums")
	lib.Icon = orDefault(cfg.Icon, "folder")
	lib.CoverStyle = orDefault(cfg.CoverStyle, "auto")
	lib.Source = model.SourceConfig
}

// saveConfigLibrary creates or updates the row. A path change wipes the
// library's tracks for the same reason the API does it: the old files are gone
// from this library, and their rows would otherwise linger unreachable until a
// full scan.
func saveConfigLibrary(s *store.Store, lib *model.Library) error {
	if lib.ID == 0 {
		return s.CreateLibrary(lib)
	}
	existing, err := s.GetLibrary(lib.ID)
	if err != nil {
		return err
	}
	pathChanged := existing.Path != lib.Path
	return s.Transaction(func(tx *store.Store) error {
		if pathChanged {
			if err := tx.DeleteTracksForLibrary(lib.ID); err != nil {
				return err
			}
			if err := tx.DeleteOrphanedAggregates(); err != nil {
				return err
			}
		}
		return tx.UpdateLibrary(lib)
	})
}

// releaseUndeclaredLibraries hands back any library still flagged config-owned
// that this run did not declare — its entry was removed from the file. The row
// and its tracks stay; it simply becomes editable and deletable in the UI again,
// so a commented-out config entry can never silently wipe a scanned library.
func releaseUndeclaredLibraries(s *store.Store, declared map[uint]bool, l *slog.Logger) error {
	managed, err := s.ListLibrariesBySource(model.SourceConfig)
	if err != nil {
		return fmt.Errorf("list config libraries: %w", err)
	}
	for _, lib := range managed {
		if declared[lib.ID] {
			continue
		}
		if err := s.SetLibrarySource(lib.ID, model.SourceDB); err != nil {
			return fmt.Errorf("release library %q: %w", lib.Name, err)
		}
		l.Info("library no longer declared in config; it is now editable in the UI",
			slog.String("component", "startup"),
			slog.String("library", lib.Name), slog.String("path", lib.Path))
	}
	return nil
}

// encodeExcludes stores the patterns the way the libraries API does: a
// JSON-encoded array, empty string when there are none.
func encodeExcludes(patterns []string) string {
	if len(patterns) == 0 {
		return ""
	}
	b, err := json.Marshal(patterns)
	if err != nil {
		// Marshalling []string cannot fail; an empty list is the safe reading.
		return ""
	}
	return string(b)
}

func boolOr(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
