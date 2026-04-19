package model

import "time"

type StarredItem struct {
	ID        uint   `gorm:"primaryKey"`
	ItemType  string `gorm:"not null;uniqueIndex:idx_starred_item"`
	ItemID    uint   `gorm:"not null;uniqueIndex:idx_starred_item"`
	CreatedAt time.Time
}
