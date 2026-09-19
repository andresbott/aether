package model

import "gorm.io/gorm"

func Migrate(db *gorm.DB) error {
	err := db.AutoMigrate(
		&Library{},
		&Artist{},
		&Album{},
		&Track{},
		&Genre{},
		&AlbumArtist{},
		&TrackArtist{},
		&TrackGenre{},
		&AlbumGenre{},
		&Playlist{},
		&PlaylistTrack{},
		&StarredItem{},
		&PlayHistory{},
		&PlaylistPlay{},
		&InternetRadioStation{},
		&PlayQueue{},
		&PlayQueueEntry{},
	)
	if err != nil {
		return err
	}
	// Composite unique index for album identity
	migrator := db.Migrator()
	if !migrator.HasIndex(&Album{}, "idx_album_identity") {
		if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_album_identity ON albums(name_norm, album_artist_norm, mb_release_id)").Error; err != nil {
			return err
		}
	}
	// Partial unique index for playlist UUID: only non-empty UUIDs must be unique.
	// Empty UUIDs are a legacy/error state (rows created before this change or
	// direct GORM creates bypassing the store), and multiple empty values must not
	// collide — they represent distinct playlists without durable keys.
	if !migrator.HasIndex(&Playlist{}, "idx_playlist_uuid") {
		if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_playlist_uuid ON playlists(uuid) WHERE uuid != ''").Error; err != nil {
			return err
		}
	}
	return nil
}
