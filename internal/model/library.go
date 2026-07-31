package model

import "time"

type Library struct {
	ID              uint   `gorm:"primaryKey"`
	Name            string `gorm:"not null;uniqueIndex"`
	Path            string `gorm:"not null;uniqueIndex"`
	ExcludePatterns string `gorm:"type:text"` // JSON-encoded []string
	FollowSymlinks  bool   `gorm:"default:true"`
	// HideArtists, when set, removes this library's artists from the artist
	// index. Albums/tracks/search are unaffected. Zero value = visible.
	// default:false is safe with GORM zero-value handling (omitting false
	// yields false) and lets SQLite ALTER TABLE add the NOT NULL column.
	HideArtists bool   `gorm:"not null;default:false"`
	DefaultView string `gorm:"not null;default:'albums'"` // "albums" | "artists"
	Icon        string `gorm:"not null;default:'folder'"` // PrimeIcons name without the "pi pi-" prefix
	// CoverStyle selects the covergen style for generated (placeholder)
	// covers of this library's albums and artists: "auto" picks a style
	// deterministically per seed; other values are covergen style names.
	CoverStyle        string `gorm:"not null;default:'auto'"`
	LastScanStartedAt *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
