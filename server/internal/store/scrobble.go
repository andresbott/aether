package store

import (
	"time"

	"github.com/andresbott/aether/internal/model"
)

func (s *Store) RecordPlay(owner string, trackID uint, playedAt time.Time) error {
	ph := model.PlayHistory{Owner: owner, TrackID: trackID, PlayedAt: playedAt}
	return s.db.Create(&ph).Error
}

// NowPlayingEntry is one active listener: the track and who is playing it.
type NowPlayingEntry struct {
	Track model.Track
	Owner string
}

// GetNowPlaying is deliberately global — the endpoint exists to show every
// user's current playback — but each entry names its real user.
func (s *Store) GetNowPlaying() ([]NowPlayingEntry, error) {
	threshold := time.Now().Add(-5 * time.Minute)
	type row struct {
		TrackID uint   `gorm:"column:track_id"`
		Owner   string `gorm:"column:owner"`
	}
	var rows []row
	if err := s.db.Model(&model.PlayHistory{}).
		Select("track_id, owner").
		Where("played_at > ?", threshold).
		Order("played_at DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	ids := make([]uint, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.TrackID)
	}
	var tracks []model.Track
	if err := s.db.
		Preload("Album").Preload("Album.Artists").Preload("Artists").Preload("Genres").
		Where("id IN ?", ids).Find(&tracks).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]model.Track, len(tracks))
	for _, t := range tracks {
		byID[t.ID] = t
	}
	out := make([]NowPlayingEntry, 0, len(rows))
	for _, r := range rows {
		if t, ok := byID[r.TrackID]; ok {
			out = append(out, NowPlayingEntry{Track: t, Owner: r.Owner})
		}
	}
	return out, nil
}
