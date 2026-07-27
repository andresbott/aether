package model

import "time"

type Artist struct {
	ID               uint   `gorm:"primaryKey"`
	Name             string `gorm:"not null"`
	NameNorm         string `gorm:"uniqueIndex;not null"`
	MBArtistID       string `gorm:"index"`
	// ImagePath points at an image found next to the artist's albums on disk
	// (`<collection>/<artist>/artist.jpg`), detected at scan time. It is the
	// last fallback before the generated avatar — a fetched or uploaded image
	// in the asset store wins.
	ImagePath        string
	LastImageFetchAt *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
