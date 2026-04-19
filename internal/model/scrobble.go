package model

import "time"

type PlayHistory struct {
	ID       uint `gorm:"primaryKey"`
	TrackID  uint `gorm:"index;not null"`
	PlayedAt time.Time
}
