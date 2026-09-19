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
	// Source records who owns this library's configuration: SourceDB for one
	// created through the admin UI, SourceConfig for one declared in the
	// config file's Libraries list and materialized here at startup. A config
	// library is read-only over the API — startup rewrites its fields from the
	// file on every boot, so an API write would be silently reverted.
	Source string `gorm:"not null;default:'db';index"`
	// LastScanStartedAt is a legacy column nothing writes any more: the
	// scanner reads scan folders from the config file instead of library
	// rows, so no scan is ever attributed to a library. It goes away together
	// with the library's other disk-era fields (Path, ExcludePatterns,
	// FollowSymlinks, Source) once libraries become purely filter-based.
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

// LibraryFilterField names what a LibraryFilter tests.
type LibraryFilterField string

// The filter vocabulary. Track-level fields test the track row; album-level
// ones (release type, compilation) test the track's album.
const (
	// FilterScanFolder matches tracks indexed under one of the named scan folders.
	FilterScanFolder LibraryFilterField = "scan_folder"
	// FilterPath matches tracks whose file lies under one of the absolute directories.
	FilterPath LibraryFilterField = "path"
	// FilterFormat matches tracks by lowercase file extension ("flac").
	FilterFormat LibraryFilterField = "format"
	// FilterReleaseType matches tracks whose album carries one of the release
	// types, case-insensitively; the empty string matches an album with none.
	FilterReleaseType LibraryFilterField = "release_type"
	// FilterCompilation takes exactly one value, "true" or "false".
	FilterCompilation LibraryFilterField = "compilation"
	// FilterGenre matches tracks tagged with one of the genre names.
	FilterGenre LibraryFilterField = "genre"
)

// LibraryFilter is one condition of a library. A library's filters are AND-ed;
// the Values of one filter are OR-ed.
type LibraryFilter struct {
	Field  LibraryFilterField `json:"field"`
	Values []string           `json:"values"`
}
