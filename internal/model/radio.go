package model

import "time"

type InternetRadioStation struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"not null"`
	StreamURL   string `gorm:"not null"`
	HomepageURL string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
