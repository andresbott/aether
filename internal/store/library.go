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

func (s *Store) UpdateLibrary(lib *model.Library) error {
	return s.db.Save(lib).Error
}

// DeleteLibrary removes the library row. A library owns no tracks — they belong
// to scan folders — so nothing else is touched.
func (s *Store) DeleteLibrary(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&model.Library{}, id).Error
}
