package store

import (
	"github.com/andresbott/aether/internal/model"
	"gorm.io/gorm/clause"
)

func (s *Store) Star(itemType string, itemID uint) error {
	item := model.StarredItem{ItemType: itemType, ItemID: itemID}
	return s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&item).Error
}

func (s *Store) Unstar(itemType string, itemID uint) error {
	return s.db.Where("item_type = ? AND item_id = ?", itemType, itemID).Delete(&model.StarredItem{}).Error
}

type StarredResult struct {
	Artists []model.Artist
	Albums  []model.Album
	Tracks  []model.Track
}

func (s *Store) GetStarred(filter *StarredFilter) (*StarredResult, error) {
	result := &StarredResult{}

	var artistIDs []uint
	if err := s.db.Model(&model.StarredItem{}).Where("item_type = 'artist'").Pluck("item_id", &artistIDs).Error; err != nil {
		return nil, err
	}
	if len(artistIDs) > 0 {
		q := s.db.Model(&model.Artist{}).Where("artists.id IN ?", artistIDs)
		if filter != nil && filter.LibraryID != nil {
			q = q.
				Distinct().
				Joins("JOIN track_artists ON track_artists.artist_id = artists.id").
				Joins("JOIN tracks ON tracks.id = track_artists.track_id").
				Where("tracks.library_id = ?", *filter.LibraryID)
		}
		if err := q.Find(&result.Artists).Error; err != nil {
			return nil, err
		}
	}

	var albumIDs []uint
	if err := s.db.Model(&model.StarredItem{}).Where("item_type = 'album'").Pluck("item_id", &albumIDs).Error; err != nil {
		return nil, err
	}
	if len(albumIDs) > 0 {
		q := s.db.Preload("Artists").Where("albums.id IN ?", albumIDs)
		if filter != nil && filter.LibraryID != nil {
			q = q.Where("EXISTS (SELECT 1 FROM tracks WHERE tracks.album_id = albums.id AND tracks.library_id = ?)", *filter.LibraryID)
		}
		if err := q.Find(&result.Albums).Error; err != nil {
			return nil, err
		}
	}

	var trackIDs []uint
	if err := s.db.Model(&model.StarredItem{}).Where("item_type = 'track'").Pluck("item_id", &trackIDs).Error; err != nil {
		return nil, err
	}
	if len(trackIDs) > 0 {
		q := s.db.Preload("Album").Preload("Album.Artists").Preload("Artists").Preload("Genres").Where("tracks.id IN ?", trackIDs)
		if filter != nil && filter.LibraryID != nil {
			q = q.Where("tracks.library_id = ?", *filter.LibraryID)
		}
		if err := q.Find(&result.Tracks).Error; err != nil {
			return nil, err
		}
	}
	return result, nil
}
