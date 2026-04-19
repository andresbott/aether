package store

import (
	"time"

	"github.com/andresbott/aether/internal/model"
)

func (s *Store) RecordPlay(trackID uint, playedAt time.Time) error {
	ph := model.PlayHistory{TrackID: trackID, PlayedAt: playedAt}
	return s.db.Create(&ph).Error
}

func (s *Store) GetNowPlaying() ([]model.Track, error) {
	threshold := time.Now().Add(-5 * time.Minute)
	var trackIDs []uint
	if err := s.db.Model(&model.PlayHistory{}).
		Where("played_at > ?", threshold).
		Order("played_at DESC").
		Pluck("track_id", &trackIDs).Error; err != nil {
		return nil, err
	}
	if len(trackIDs) == 0 {
		return nil, nil
	}
	var tracks []model.Track
	err := s.db.
		Preload("Album").
		Preload("Album.Artists").
		Preload("Artists").
		Preload("Genres").
		Where("id IN ?", trackIDs).
		Find(&tracks).Error
	if err != nil {
		return nil, err
	}
	idxMap := make(map[uint]int, len(trackIDs))
	for i, id := range trackIDs {
		idxMap[id] = i
	}
	ordered := make([]model.Track, len(tracks))
	for _, t := range tracks {
		ordered[idxMap[t.ID]] = t
	}
	return ordered, nil
}
