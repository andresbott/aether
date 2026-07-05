package model

import "time"

type Artist struct {
	ID               uint   `gorm:"primaryKey"`
	Name             string `gorm:"not null"`
	NameNorm         string `gorm:"uniqueIndex;not null"`
	MBArtistID       string `gorm:"index"`
	LastImageFetchAt *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
