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
	return s.db.Create(lib).Error
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

// CoverStyleForAlbum returns the cover_style of the library holding the
// album's tracks. When the album spans libraries the lowest library ID wins,
// keeping the choice deterministic. Albums with no tracks yield "auto".
func (s *Store) CoverStyleForAlbum(albumID uint) (string, error) {
	var style string
	err := s.db.Model(&model.Library{}).
		Select("libraries.cover_style").
		Joins("JOIN tracks ON tracks.library_id = libraries.id").
		Where("tracks.album_id = ?", albumID).
		Order("libraries.id ASC").
		Limit(1).
		Scan(&style).Error
	if err != nil || style == "" {
		return "auto", err
	}
	return style, nil
}

// CoverStyleForArtist returns the cover_style of the library holding the
// artist's tracks, resolved like CoverStyleForAlbum.
func (s *Store) CoverStyleForArtist(artistID uint) (string, error) {
	var style string
	err := s.db.Model(&model.Library{}).
		Select("libraries.cover_style").
		Joins("JOIN tracks ON tracks.library_id = libraries.id").
		Joins("JOIN track_artists ON track_artists.track_id = tracks.id").
		Where("track_artists.artist_id = ?", artistID).
		Order("libraries.id ASC").
		Limit(1).
		Scan(&style).Error
	if err != nil || style == "" {
		return "auto", err
	}
	return style, nil
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
