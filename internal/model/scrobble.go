package model

import "time"

type PlayHistory struct {
	ID       uint   `gorm:"primaryKey"`
	Owner    string `gorm:"index;not null"`
	TrackID  uint   `gorm:"index;not null"`
	PlayedAt time.Time
}
