package model

import "time"

type StarredItem struct {
	ID        uint   `gorm:"primaryKey"`
	Owner     string `gorm:"not null;uniqueIndex:idx_starred_item"`
	ItemType  string `gorm:"not null;uniqueIndex:idx_starred_item"`
	ItemID    uint   `gorm:"not null;uniqueIndex:idx_starred_item"`
	CreatedAt time.Time
}
