package model

import "time"

// PlaylistPlay records one play of a playlist, mirroring PlayHistory for tracks.
// Playlists are played as a unit, so the count is per playlist, not per track.
type PlaylistPlay struct {
	ID         uint   `gorm:"primaryKey"`
	Owner      string `gorm:"index;not null"`
	PlaylistID uint   `gorm:"index;not null"`
	PlayedAt   time.Time
}
