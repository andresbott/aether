package model

import "time"

// Library is a named, filtered view over the catalog — what users browse and
// what Subsonic calls a music folder. It owns no tracks and names no directory:
// the directories the server scans are the scan folders of the config file
// (internal/scanfolder), and a library merely selects from what they index.
type Library struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"not null;uniqueIndex"`
	// HideArtists, when set, removes this library's artists from the artist
	// index. Albums/tracks/search are unaffected. Zero value = visible.
	HideArtists bool   `gorm:"not null;default:false"`
	DefaultView string `gorm:"not null;default:'albums'"` // "albums" | "artists"
	Icon        string `gorm:"not null;default:'folder'"` // PrimeIcons name without the "pi pi-" prefix
	// Filters select the library's tracks: they are AND-ed, and the values of
	// one filter are OR-ed (see store.ScopeOf). None means the whole catalog.
	Filters   []LibraryFilter `gorm:"serializer:json"`
	CreatedAt time.Time
	UpdatedAt time.Time
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
