package store

import (
	"github.com/andresbott/aether/internal/model"
	"gorm.io/gorm"
)

func (s *Store) CreatePlaylist(name, owner string, public bool, trackIDs []uint) (*model.Playlist, error) {
	pl := model.Playlist{Name: name, Owner: owner, Public: public}
	if err := s.db.Create(&pl).Error; err != nil {
		return nil, err
	}
	for i, tid := range trackIDs {
		pt := model.PlaylistTrack{PlaylistID: pl.ID, TrackID: tid, SortOrder: i}
		if err := s.db.Create(&pt).Error; err != nil {
			return nil, err
		}
	}
	return &pl, nil
}

// GetPlaylists returns the playlists visible to owner: their own plus every
// public one. There is no unscoped variant — a caller without an identity is
// the single-user "admin".
func (s *Store) GetPlaylists(owner string) ([]model.Playlist, error) {
	var playlists []model.Playlist
	err := s.db.Where("owner = ? OR public = ?", owner, true).Order("name ASC").Find(&playlists).Error
	return playlists, err
}

func (s *Store) GetPlaylist(id uint) (*model.Playlist, error) {
	var pl model.Playlist
	err := s.db.First(&pl, id).Error
	return &pl, err
}

func (s *Store) GetPlaylistTracks(playlistID uint) ([]model.Track, error) {
	var pts []model.PlaylistTrack
	if err := s.db.Where("playlist_id = ?", playlistID).Order("sort_order ASC").Find(&pts).Error; err != nil {
		return nil, err
	}
	if len(pts) == 0 {
		return nil, nil
	}
	trackIDs := make([]uint, len(pts))
	for i, pt := range pts {
		trackIDs[i] = pt.TrackID
	}
	var tracks []model.Track
	if err := s.db.
		Preload("Album").
		Preload("Album.Artists").
		Preload("Artists").
		Preload("Genres").
		Where("id IN ?", trackIDs).
		Find(&tracks).Error; err != nil {
		return nil, err
	}
	trackMap := make(map[uint]model.Track, len(tracks))
	for _, t := range tracks {
		trackMap[t.ID] = t
	}
	ordered := make([]model.Track, 0, len(pts))
	for _, pt := range pts {
		if t, ok := trackMap[pt.TrackID]; ok {
			ordered = append(ordered, t)
		}
	}
	return ordered, nil
}

func (s *Store) GetPlaylistTrackCount(playlistID uint) (int64, error) {
	var count int64
	err := s.db.Model(&model.PlaylistTrack{}).Where("playlist_id = ?", playlistID).Count(&count).Error
	return count, err
}

func (s *Store) GetPlaylistDuration(playlistID uint) (int, error) {
	var total int
	err := s.db.
		Table("playlist_tracks").
		Joins("JOIN tracks ON tracks.id = playlist_tracks.track_id").
		Where("playlist_tracks.playlist_id = ?", playlistID).
		Select("COALESCE(SUM(tracks.duration), 0)").
		Scan(&total).Error
	return total, err
}

// UpdatePlaylist applies a partial update: only the non-nil fields are written.
// A no-op (all nil) returns nil without touching the row.
func (s *Store) UpdatePlaylist(id uint, name, comment *string, public *bool) error {
	fields := map[string]any{}
	if name != nil {
		fields["name"] = *name
	}
	if comment != nil {
		fields["comment"] = *comment
	}
	if public != nil {
		fields["public"] = *public
	}
	if len(fields) == 0 {
		return nil
	}
	return s.db.Model(&model.Playlist{}).Where("id = ?", id).Updates(fields).Error
}

// SetPlaylistTracks replaces the entire ordered track set of a playlist with the
// given track IDs (used by createPlaylist's update-by-id path per the spec).
func (s *Store) SetPlaylistTracks(playlistID uint, trackIDs []uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("playlist_id = ?", playlistID).Delete(&model.PlaylistTrack{}).Error; err != nil {
			return err
		}
		for i, tid := range trackIDs {
			pt := model.PlaylistTrack{PlaylistID: playlistID, TrackID: tid, SortOrder: i}
			if err := tx.Create(&pt).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) DeletePlaylist(id uint) error {
	if err := s.db.Where("playlist_id = ?", id).Delete(&model.PlaylistTrack{}).Error; err != nil {
		return err
	}
	return s.db.Delete(&model.Playlist{}, id).Error
}

func (s *Store) AddTracksToPlaylist(playlistID uint, trackIDs []uint) error {
	var maxSort int
	s.db.Model(&model.PlaylistTrack{}).
		Where("playlist_id = ?", playlistID).
		Select("COALESCE(MAX(sort_order), -1)").
		Scan(&maxSort)
	for i, tid := range trackIDs {
		pt := model.PlaylistTrack{PlaylistID: playlistID, TrackID: tid, SortOrder: maxSort + 1 + i}
		if err := s.db.Create(&pt).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) RemoveTrackFromPlaylist(playlistID uint, index int) error {
	var pts []model.PlaylistTrack
	if err := s.db.Where("playlist_id = ?", playlistID).Order("sort_order ASC").Find(&pts).Error; err != nil {
		return err
	}
	if index < 0 || index >= len(pts) {
		return nil
	}
	target := pts[index]
	return s.db.Where("playlist_id = ? AND track_id = ? AND sort_order = ?", target.PlaylistID, target.TrackID, target.SortOrder).Delete(&model.PlaylistTrack{}).Error
}
