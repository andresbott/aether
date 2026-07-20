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
	// Bool fields with a gorm default tag (FollowSymlinks, ShowArtists) are
	// omitted by GORM when false, letting the column default win. Execute
	// a raw INSERT to force all column values through, then scan the ID back.
	result := s.db.Exec(`
		INSERT INTO libraries (name, path, exclude_patterns, follow_symlinks,
			show_artists, default_view, last_scan_started_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		lib.Name, lib.Path, lib.ExcludePatterns, lib.FollowSymlinks,
		lib.ShowArtists, lib.DefaultView, lib.LastScanStartedAt,
		lib.CreatedAt, lib.UpdatedAt)
	if result.Error != nil {
		return result.Error
	}
	// SQLite RETURNING clause - scan the ID
	return s.db.Raw(`SELECT id FROM libraries WHERE name = ? AND path = ?`,
		lib.Name, lib.Path).Scan(&lib.ID).Error
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
