package store

import (
	"github.com/andresbott/aether/internal/model"
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
		return model.Library{}, err
	}
	return lib, nil
}

func (s *Store) CreateLibrary(lib *model.Library) error {
	// Workaround for GORM v2 bool zero-value handling with default tags:
	// GORM omits false bool values when the tag has default:true, letting the DB
	// default take over. Use a map to force the value through.
	showArtists := lib.ShowArtists
	if err := s.db.Create(lib).Error; err != nil {
		return err
	}
	// If ShowArtists was explicitly false, update it after creation
	if !showArtists {
		return s.db.Model(lib).Update("show_artists", false).Error
	}
	return nil
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

func (s *Store) DeleteLibrary(id uint) error {
	return s.Transaction(func(tx *Store) error {
		if err := tx.DeleteTracksForLibrary(id); err != nil {
			return err
		}
		if err := tx.DeleteOrphanedAggregates(); err != nil {
			return err
		}
		return tx.db.Delete(&model.Library{}, id).Error
	})
}
