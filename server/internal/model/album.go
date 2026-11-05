package model

import "time"

type Album struct {
	ID              uint   `gorm:"primaryKey"`
	Name            string `gorm:"not null"`
	NameNorm        string `gorm:"not null"`
	AlbumArtistNorm string `gorm:"not null"`
	MBReleaseID     string
	// MBReleaseGroupID is the MusicBrainz release group (the album across its
	// editions). Not part of the album's identity — refreshed from the tags on
	// every reconcile — and what an album wishlist entry matches on.
	MBReleaseGroupID string `gorm:"index"`
	Year             int
	Compilation      bool
	ReleaseTypes     []string `gorm:"serializer:json"`
	CoverPath        string
	HasEmbeddedCover bool
	// GroupingID is the album's grouping (the GROUPING tag), or nil. Not part of
	// the album's identity: it is recomputed from the tracks after every scan
	// (store.ReconcileGroupings), so re-tagging moves the album instead of
	// creating a second one.
	GroupingID *uint `gorm:"index"`
	CreatedAt  time.Time
	UpdatedAt  time.Time

	Grouping *Grouping
	Artists  []*Artist `gorm:"many2many:album_artists"`
	Genres   []*Genre  `gorm:"many2many:album_genres"`
	Tracks   []*Track
}
