package model

import "time"

type Library struct {
	ID              uint   `gorm:"primaryKey"`
	Name            string `gorm:"not null;uniqueIndex"`
	Path            string `gorm:"not null;uniqueIndex"`
	ExcludePatterns string `gorm:"type:text"` // JSON-encoded []string
	// FollowSymlinks deliberately carries no DB-level default:true. GORM skips
	// zero values on insert when a default is declared, so "follow: false"
	// would silently persist as true. The default lives in application code
	// (the libraries API and the config reconcile both set it explicitly).
	FollowSymlinks bool `gorm:"not null;default:false"`
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
	CoverStyle string `gorm:"not null;default:'auto'"`
	// Source records who owns this library's configuration: SourceDB for one
	// created through the admin UI, SourceConfig for one declared in the
	// config file's Libraries list and materialized here at startup. A config
	// library is read-only over the API — startup rewrites its fields from the
	// file on every boot, so an API write would be silently reverted.
	// LastScanStartedAt is runtime state, not configuration, and stays on the
	// row for both sources.
	Source            string `gorm:"not null;default:'db';index"`
	LastScanStartedAt *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Library.Source values.
const (
	// SourceDB is a library created and owned through the admin UI.
	SourceDB = "db"
	// SourceConfig is a library declared in the config file.
	SourceConfig = "config"
)

// IsConfigManaged reports whether the library is owned by the config file and
// therefore not editable through the API.
func (l Library) IsConfigManaged() bool {
	return l.Source == SourceConfig
}
