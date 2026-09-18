package model

// CoverSettings is the singleton (ID always 1) holding the server-wide
// generated-cover configuration. DefaultStyles is the pool auto-generation
// picks from for an entity with no image; AvailableStyles is the pool offered
// in the editor's Generate picker. Both hold covergen style names. Stored as
// JSON columns (serializer:json) like other list-valued columns.
type CoverSettings struct {
	ID              uint     `gorm:"primaryKey"`
	DefaultStyles   []string `gorm:"serializer:json"`
	AvailableStyles []string `gorm:"serializer:json"`
}
