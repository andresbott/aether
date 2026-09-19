package store

import (
	"context"

	"github.com/andresbott/aether/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func (s *Store) ListLibraries() ([]model.Library, error) {
	var libs []model.Library
	if err := s.db.Order("name ASC").Find(&libs).Error; err != nil {
		return nil, err
	}
	return libs, nil
}

// LibraryRoots returns the filesystem root of every configured library. Used to
// confine which files the media handlers will serve — see internal/pathguard.
func (s *Store) LibraryRoots() ([]string, error) {
	var roots []string
	if err := s.db.Model(&model.Library{}).Order("id ASC").Pluck("path", &roots).Error; err != nil {
		return nil, err
	}
	return roots, nil
}

func (s *Store) GetLibrary(id uint) (model.Library, error) {
	var lib model.Library
	if err := s.db.First(&lib, id).Error; err != nil {
		return model.Library{}, notFound(err)
	}
	return lib, nil
}

func (s *Store) CreateLibrary(lib *model.Library) error {
	return s.db.Create(lib).Error
}

// FindLibraryByName returns the library with this exact name, or ErrNotFound. A
// miss is an ordinary answer for these two lookups (the config reconcile asks
// "does this library exist yet?"), so neither logs the not-found as an error.
func (s *Store) FindLibraryByName(name string) (model.Library, error) {
	var lib model.Library
	if err := s.db.Session(&gorm.Session{Logger: s.db.Logger.LogMode(logger.Silent)}).
		Where("name = ?", name).First(&lib).Error; err != nil {
		return model.Library{}, notFound(err)
	}
	return lib, nil
}

// FindLibraryByPath returns the library rooted at this exact path, or
// ErrNotFound.
func (s *Store) FindLibraryByPath(path string) (model.Library, error) {
	var lib model.Library
	if err := s.db.Session(&gorm.Session{Logger: s.db.Logger.LogMode(logger.Silent)}).
		Where("path = ?", path).First(&lib).Error; err != nil {
		return model.Library{}, notFound(err)
	}
	return lib, nil
}

// ListLibrariesBySource returns every library owned by the given source
// (model.SourceDB / model.SourceConfig), ordered by name.
func (s *Store) ListLibrariesBySource(source string) ([]model.Library, error) {
	var libs []model.Library
	if err := s.db.Where("source = ?", source).Order("name ASC").Find(&libs).Error; err != nil {
		return nil, err
	}
	return libs, nil
}

// SetLibrarySource changes which source owns a library. Used by the startup
// reconcile to adopt a UI-created library into config ownership, and to hand a
// library back to the UI when its config entry disappears.
func (s *Store) SetLibrarySource(id uint, source string) error {
	return s.db.Model(&model.Library{}).Where("id = ?", id).Update("source", source).Error
}

func (s *Store) UpdateLibrary(lib *model.Library) error {
	return s.db.Save(lib).Error
}

func (s *Store) DeleteTracksForLibrary(id uint) error {
	return s.db.Where("library_id = ?", id).Delete(&model.Track{}).Error
}

func (s *Store) CountTracksForLibrary(id uint) (int64, error) {
	var count int64
	if err := s.db.Model(&model.Track{}).Where("library_id = ?", id).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) DeleteLibrary(ctx context.Context, id uint) error {
	return s.TransactionContext(ctx, func(tx *Store) error {
		if err := tx.DeleteTracksForLibrary(id); err != nil {
			return err
		}
		if err := tx.DeleteOrphanedAggregates(); err != nil {
			return err
		}
		return tx.db.Delete(&model.Library{}, id).Error
	})
}
