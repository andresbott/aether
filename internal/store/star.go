package store

import (
	"time"

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
	Artists   []model.Artist
	Albums    []model.Album
	Tracks    []model.Track
	Playlists []model.Playlist
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

	// Playlists are not scoped to a library — a playlist can hold tracks from
	// several — so StarredFilter.LibraryID deliberately does not apply here.
	var playlistStars []model.StarredItem
	if err := s.db.Where("item_type = 'playlist'").
		Order("created_at DESC").
		Find(&playlistStars).Error; err != nil {
		return nil, err
	}
	if len(playlistStars) > 0 {
		playlistIDs := make([]uint, 0, len(playlistStars))
		for _, st := range playlistStars {
			playlistIDs = append(playlistIDs, st.ItemID)
		}
		var playlists []model.Playlist
		if err := s.db.Where("id IN ?", playlistIDs).Find(&playlists).Error; err != nil {
			return nil, err
		}
		byID := make(map[uint]model.Playlist, len(playlists))
		for _, pl := range playlists {
			byID[pl.ID] = pl
		}
		// Re-apply the star order the IN query lost, skipping stars whose
		// playlist no longer exists.
		result.Playlists = make([]model.Playlist, 0, len(playlists))
		for _, id := range playlistIDs {
			if pl, ok := byID[id]; ok {
				result.Playlists = append(result.Playlists, pl)
			}
		}
	}
	return result, nil
}

// PlaylistStarredAt returns when each of the given playlists was starred, in one
// query. Playlists that are not starred are absent from the map.
func (s *Store) PlaylistStarredAt(playlistIDs []uint) (map[uint]time.Time, error) {
	out := map[uint]time.Time{}
	if len(playlistIDs) == 0 {
		return out, nil
	}
	var stars []model.StarredItem
	if err := s.db.
		Where("item_type = 'playlist' AND item_id IN ?", playlistIDs).
		Find(&stars).Error; err != nil {
		return nil, err
	}
	for _, st := range stars {
		out[st.ItemID] = st.CreatedAt
	}
	return out, nil
}
