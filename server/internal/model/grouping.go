package model

import "time"

// Grouping is a named container for albums — a franchise, a series, a box set —
// read from the GROUPING tag of the files. An album belongs to at most one
// (Album.GroupingID). Identity is NameNorm, so spellings that differ only in
// case or accents are the same grouping.
type Grouping struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"not null"`
	NameNorm  string `gorm:"not null;uniqueIndex"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
