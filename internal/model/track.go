package model

import "time"

type Track struct {
	ID                  uint   `gorm:"primaryKey"`
	AlbumID             uint   `gorm:"index;not null"`
	Filename            string `gorm:"not null"`
	FilePath            string `gorm:"uniqueIndex;not null"`
	FileSize            int64
	FileModTime         time.Time
	LastSeenAt          time.Time `gorm:"index"`
	Title               string
	TitleNorm           string
	TrackNumber         int
	DiscNumber          int
	DiscSubtitle        string
	Year                int
	Duration            int
	Bitrate             int
	MBRecordingID       string
	Lyrics              string
	ReplayGainTrackGain *float64
	ReplayGainTrackPeak *float64
	ReplayGainAlbumGain *float64
	ReplayGainAlbumPeak *float64
	HasEmbeddedCover    bool
	CreatedAt           time.Time
	UpdatedAt           time.Time

	Album   *Album    `gorm:"foreignKey:AlbumID"`
	Artists []*Artist `gorm:"many2many:track_artists"`
	Genres  []*Genre  `gorm:"many2many:track_genres"`
}
