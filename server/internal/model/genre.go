package model

// Genre deliberately has no NameNorm column, unlike Artist/Album/Track: there
// are few enough genres that SearchGenres normalizes in Go, which keeps the
// match identical to the other searches without a column that would need
// backfilling on every rename.
type Genre struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"uniqueIndex;not null"`
}
