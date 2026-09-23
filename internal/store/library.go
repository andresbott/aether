package store

import (
	"context"

	"github.com/andresbott/aether/internal/model"
	"gorm.io/gorm/clause"
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

func (s *Store) UpdateLibrary(lib *model.Library) error {
	return s.db.Save(lib).Error
}

// DeleteLibrary removes the library row. A library owns no tracks — they belong
// to scan folders — so nothing else is touched.
func (s *Store) DeleteLibrary(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&model.Library{}, id).Error
}

// GetCatalogSettings answers the root library's settings: the saved row, or the
// defaults when none has been saved yet.
func (s *Store) GetCatalogSettings() (model.CatalogSettings, error) {
	var cs model.CatalogSettings
	err := s.db.Limit(1).Find(&cs, 1).Error
	if err != nil {
		return model.CatalogSettings{}, err
	}
	if cs.ID == 0 {
		return model.DefaultCatalogSettings(), nil
	}
	return cs, nil
}

// SaveCatalogSettings stores the root library's settings, creating the row on
// first save.
func (s *Store) SaveCatalogSettings(cs model.CatalogSettings) error {
	cs.ID = 1
	return s.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&cs).Error
}
