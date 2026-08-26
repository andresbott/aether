package model

import "time"

type Track struct {
	ID            uint   `gorm:"primaryKey"`
	AlbumID       uint   `gorm:"index;not null"`
	LibraryID     uint   `gorm:"index;not null;constraint:OnDelete:CASCADE"`
	Filename      string `gorm:"not null"`
	FilePath      string `gorm:"uniqueIndex;not null"`
	FileSize      int64
	FileModTime   time.Time
	LastSeenAt    time.Time `gorm:"index"`
	Title         string
	TitleNorm     string
	TrackNumber   int
	DiscNumber    int
	DiscSubtitle  string
	Year          int
	Duration      int
	Bitrate       int
	MBRecordingID string
	// AudioHash is a metadata-invariant hash of the file's audio payload
	// (libs/audiohash). It survives a tag rewrite and changes only when the
	// audio does, which is what lets the scanner recognise a file that was
	// moved AND retagged — the case the size-and-title move proof cannot
	// anchor, because a tag edit changes both of its parts. Empty for formats
	// audiohash does not cover and for rows indexed before it existed; the
	// re-link falls back to its other signals then. Indexed because the
	// re-link looks rows up by it once per scan batch.
	AudioHash           string `gorm:"index"`
	Lyrics              string
	ReplayGainTrackGain *float64
	ReplayGainTrackPeak *float64
	ReplayGainAlbumGain *float64
	ReplayGainAlbumPeak *float64
	HasEmbeddedCover    bool
	CreatedAt           time.Time
	UpdatedAt           time.Time

	Album   *Album    `gorm:"foreignKey:AlbumID"`
	Library *Library  `gorm:"foreignKey:LibraryID"`
	Artists []*Artist `gorm:"many2many:track_artists"`
	Genres  []*Genre  `gorm:"many2many:track_genres"`
}
