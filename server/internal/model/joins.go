package model

type AlbumArtist struct {
	AlbumID  uint `gorm:"primaryKey"`
	ArtistID uint `gorm:"primaryKey"`
}

type TrackArtist struct {
	TrackID  uint `gorm:"primaryKey"`
	ArtistID uint `gorm:"primaryKey"`
}

type TrackGenre struct {
	TrackID uint `gorm:"primaryKey"`
	GenreID uint `gorm:"primaryKey"`
}

type AlbumGenre struct {
	AlbumID uint `gorm:"primaryKey"`
	GenreID uint `gorm:"primaryKey"`
}
