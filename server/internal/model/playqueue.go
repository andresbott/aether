package model

import "time"

// PlayQueue is one user's saved playback session: the queue, which slot is
// playing, and how far into that track playback had got. Exactly one row per
// owner — savePlayQueue replaces it rather than appending.
//
// The current track is stored as an INDEX, not a track id: a queue may hold the
// same track more than once, and an id could not say which copy is playing. The
// spec's id-based savePlayQueue resolves to an index at the handler boundary.
type PlayQueue struct {
	ID    uint   `gorm:"primaryKey"`
	Owner string `gorm:"uniqueIndex;not null"`
	// CurrentIndex is the 0-based slot of the playing track, or -1 when the queue
	// is saved without one.
	CurrentIndex int
	// PositionMs is the offset within the current track, in milliseconds — the
	// field that lets another device resume mid-song.
	PositionMs int64
	// ChangedBy is the Subsonic client name that last saved the queue, so a client
	// can tell its own writes from another device's.
	ChangedBy string
	Changed   time.Time
}

// PlayQueueEntry is one slot of a saved queue. SortOrder carries the queue
// order and is part of the key, because the same TrackID may legitimately
// appear in several slots.
type PlayQueueEntry struct {
	PlayQueueID uint `gorm:"primaryKey"`
	SortOrder   int  `gorm:"primaryKey"`
	TrackID     uint `gorm:"index;not null"`
}
