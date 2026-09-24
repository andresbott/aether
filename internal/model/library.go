package model

import "time"

// Library is a named, filtered view over the catalog — what users browse and
// what Subsonic calls a music folder. It owns no tracks and names no directory:
// the directories the server scans are the scan folders of the config file
// (internal/scanfolder), and a library merely selects from what they index.
type Library struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"not null;uniqueIndex"`
	// Views are the ways the library can be browsed, in LibraryViews order;
	// DefaultView is the one of them it opens on. The column defaults give a
	// library created with just a name every view, opening on Discover — the
	// same browsing the whole catalog offers.
	Views       []LibraryView `gorm:"serializer:json;not null;default:'[\"discover\",\"artists\",\"albums\"]'"`
	DefaultView LibraryView   `gorm:"not null;default:'discover'"`
	// HideFromArtistIndex keeps this library's artists out of the cross-library
	// artist index (the main Artists page). It does not touch the library
	// itself: its own Artists view, when it has one, still lists them.
	HideFromArtistIndex bool `gorm:"not null;default:false"`
	// SplitViews gives the library a sidebar section of its own, one entry per
	// view, instead of a single entry whose page switches between them. It is
	// presentation only: what the library selects and offers is unchanged.
	SplitViews bool   `gorm:"not null;default:false"`
	Icon       string `gorm:"not null;default:'folder'"` // Material Symbols name, snake_case, e.g. "queue_music"
	// Filters select the library's tracks: they are AND-ed, and the values of
	// one filter are OR-ed (see store.ScopeOf). None means the whole catalog.
	Filters   []LibraryFilter `gorm:"serializer:json"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// LibraryView names one way of browsing a library.
type LibraryView string

// The view vocabulary.
const (
	// ViewDiscover is the ranked feed of the library's albums and playlists.
	ViewDiscover LibraryView = "discover"
	// ViewArtists is the index of the artists credited on its albums.
	ViewArtists LibraryView = "artists"
	// ViewAlbums lists its albums, filterable by release type.
	ViewAlbums LibraryView = "albums"
)

// LibraryViews returns every view, in the order a library stores and shows
// them.
func LibraryViews() []LibraryView {
	return []LibraryView{ViewDiscover, ViewArtists, ViewAlbums}
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

// CatalogSettings configures the root library — the whole catalog, browsed
// without a library. It is a single row (ID 1); store.GetCatalogSettings answers
// the defaults below until one is saved.
type CatalogSettings struct {
	ID uint `gorm:"primaryKey"`
	// SplitViews lists each of the root's views as its own sidebar entry, as a
	// library's SplitViews does; false gives the root a single entry whose page
	// switches views. No gorm default: it would turn a saved false back into
	// true on insert.
	SplitViews bool `gorm:"not null"`
}

// DefaultCatalogSettings is what an install that never saved any gets: every
// root view its own sidebar entry.
func DefaultCatalogSettings() CatalogSettings {
	return CatalogSettings{ID: 1, SplitViews: true}
}
