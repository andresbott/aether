package model

import "time"

type Library struct {
	ID                uint   `gorm:"primaryKey"`
	Name              string `gorm:"not null;uniqueIndex"`
	Path              string `gorm:"not null;uniqueIndex"`
	ExcludePatterns   string `gorm:"type:text"` // JSON-encoded []string
	FollowSymlinks    bool   `gorm:"default:true"`
	// ShowArtists controls whether this library's artists appear in the
	// artist index. Albums/tracks/search are unaffected.
	ShowArtists       bool   `gorm:"not null;default:true"`
	DefaultView       string `gorm:"not null;default:'albums'"` // "albums" | "artists"
	LastScanStartedAt *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
