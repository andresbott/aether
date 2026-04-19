package model

import "gorm.io/gorm"

func Migrate(db *gorm.DB) error {
	err := db.AutoMigrate(
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
	return nil
}
