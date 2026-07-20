package model

import "time"

type Library struct {
	ID                uint   `gorm:"primaryKey"`
	Name              string `gorm:"not null;uniqueIndex"`
	Path              string `gorm:"not null;uniqueIndex"`
	ExcludePatterns   string `gorm:"type:text"` // JSON-encoded []string
	FollowSymlinks    bool   `gorm:"default:true"`
	// HideArtists, when set, removes this library's artists from the artist
	// index. Albums/tracks/search are unaffected. Zero value = visible.
	HideArtists       bool   `gorm:"not null"`
	DefaultView       string `gorm:"not null;default:'albums'"` // "albums" | "artists"
	LastScanStartedAt *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
