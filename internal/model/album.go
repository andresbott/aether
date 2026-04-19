package model

import "time"

type Album struct {
	ID               uint   `gorm:"primaryKey"`
	Name             string `gorm:"not null"`
	NameNorm         string `gorm:"not null"`
	AlbumArtistNorm  string `gorm:"not null"`
	MBReleaseID      string
	Year             int
	Compilation      bool
	ReleaseType      string
	CoverPath        string
	HasEmbeddedCover bool
	CreatedAt        time.Time
	UpdatedAt        time.Time

	Artists []*Artist `gorm:"many2many:album_artists"`
	Genres  []*Genre  `gorm:"many2many:album_genres"`
	Tracks  []*Track
}
