package model

import "time"

type Playlist struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"not null"`
	Comment   string
	Owner     string
	Public    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PlaylistTrack struct {
	PlaylistID uint `gorm:"primaryKey"`
	TrackID    uint `gorm:"primaryKey"`
	SortOrder  int
}
