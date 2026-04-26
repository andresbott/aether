package store

import (
	"fmt"
	"time"

	"github.com/andresbott/aether/internal/model"
)

func (s *Store) BulkUpdateLastSeen(paths []string, scanTime time.Time) error {
	const chunkSize = 500
	for i := 0; i < len(paths); i += chunkSize {
		end := i + chunkSize
		if end > len(paths) {
			end = len(paths)
		}
		if err := s.db.Table("tracks").Where("file_path IN ?", paths[i:end]).Update("last_seen_at", scanTime).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) FilterChanged(paths []string) (map[string]time.Time, error) {
	type trackMod struct {
		FilePath    string
		FileModTime time.Time
	}
	modMap := make(map[string]time.Time, len(paths))
	const chunkSize = 500
	for i := 0; i < len(paths); i += chunkSize {
		end := i + chunkSize
		if end > len(paths) {
			end = len(paths)
		}
		var rows []trackMod
		if err := s.db.Table("tracks").Select("file_path, file_mod_time").Where("file_path IN ?", paths[i:end]).Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			modMap[r.FilePath] = r.FileModTime
		}
	}
	return modMap, nil
}

func (s *Store) DeleteTracksNotSeenSince(scanStart time.Time) error {
	if err := s.db.Where("last_seen_at < ?", scanStart).Delete(&model.Track{}).Error; err != nil {
		return fmt.Errorf("delete orphaned tracks: %w", err)
	}
	return nil
}

func (s *Store) DeleteOrphanedAggregates() error {
	queries := []string{
		`DELETE FROM albums WHERE id NOT IN (SELECT DISTINCT album_id FROM tracks)`,
		`DELETE FROM album_artists WHERE album_id NOT IN (SELECT id FROM albums)`,
		`DELETE FROM track_artists WHERE track_id NOT IN (SELECT id FROM tracks)`,
		`DELETE FROM track_genres WHERE track_id NOT IN (SELECT id FROM tracks)`,
		`DELETE FROM album_genres WHERE album_id NOT IN (SELECT id FROM albums)`,
		`DELETE FROM artists WHERE id NOT IN (SELECT DISTINCT artist_id FROM album_artists) AND id NOT IN (SELECT DISTINCT artist_id FROM track_artists)`,
		`DELETE FROM genres WHERE id NOT IN (SELECT DISTINCT genre_id FROM track_genres) AND id NOT IN (SELECT DISTINCT genre_id FROM album_genres)`,
		`DELETE FROM playlist_tracks WHERE track_id NOT IN (SELECT id FROM tracks)`,
		`DELETE FROM play_histories WHERE track_id NOT IN (SELECT id FROM tracks)`,
		`DELETE FROM starred_items WHERE item_type = 'track' AND item_id NOT IN (SELECT id FROM tracks)`,
		`DELETE FROM starred_items WHERE item_type = 'album' AND item_id NOT IN (SELECT id FROM albums)`,
		`DELETE FROM starred_items WHERE item_type = 'artist' AND item_id NOT IN (SELECT id FROM artists)`,
	}
	for _, q := range queries {
		if err := s.db.Exec(q).Error; err != nil {
			return fmt.Errorf("cleanup query %q: %w", q, err)
		}
	}
	return nil
}

func (s *Store) Cleanup(scanStart time.Time) error {
	if err := s.DeleteTracksNotSeenSince(scanStart); err != nil {
		return err
	}
	return s.DeleteOrphanedAggregates()
}
