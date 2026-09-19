package store

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/andresbott/aether/internal/model"
	"gorm.io/gorm"
)

// BulkMarkSeen marks paths as seen by the scan that started at scanTime and
// stamps them with the scan folder they were walked under.
//
// It advances the liveness marker on paths an incremental scan found unchanged
// on disk, and it is the one statement that touches EVERY walked file on EVERY
// scan — which is why the scan-folder stamp rides along here: a folder renamed
// in configuration is healed by the next scan, incremental included, at no
// extra cost.
//
// The update is monotonic — `last_seen_at < scanTime` in the WHERE clause — for
// the same reason reconcileTrack's assignment is: kept as a cheap safety net —
// scan and reindex are serialized by the `library-writes` exclusion group so
// they don't actually overlap. Lowering a newer marker would make a live track
// look stale to a scan already in flight, and its Cleanup would delete the row
// along with the track's playlist memberships, play history and stars. Within a
// single scan every row is either already at scanTime (no-op) or older
// (advances), so the added predicate never skips a row that needs the bump. A
// row the predicate skips was stamped by the scan that holds the newer marker.
func (s *Store) BulkMarkSeen(paths []string, scanFolder string, scanTime time.Time) error {
	for i := 0; i < len(paths); i += chunkSize {
		end := i + chunkSize
		if end > len(paths) {
			end = len(paths)
		}
		if err := s.db.Table("tracks").
			Where("file_path IN ? AND last_seen_at < ?", paths[i:end], scanTime).
			Updates(map[string]any{"last_seen_at": scanTime, "scan_folder": scanFolder}).Error; err != nil {
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
		`DELETE FROM playlist_plays WHERE playlist_id NOT IN (SELECT id FROM playlists)`,
		`DELETE FROM play_queue_entries WHERE track_id NOT IN (SELECT id FROM tracks)`,
		`DELETE FROM starred_items WHERE item_type = 'track' AND item_id NOT IN (SELECT id FROM tracks)`,
		`DELETE FROM starred_items WHERE item_type = 'album' AND item_id NOT IN (SELECT id FROM albums)`,
		`DELETE FROM starred_items WHERE item_type = 'artist' AND item_id NOT IN (SELECT id FROM artists)`,
		`DELETE FROM starred_items WHERE item_type = 'playlist' AND item_id NOT IN (SELECT id FROM playlists)`,
	}
	for _, q := range queries {
		if err := s.db.Exec(q).Error; err != nil {
			return fmt.Errorf("cleanup query %q: %w", q, err)
		}
	}
	return nil
}

func (s *Store) Cleanup(ctx context.Context, scanStart time.Time) error {
	// One transaction for the track delete and the 16-statement aggregate sweep:
	// otherwise a failure partway through the sweep commits a subset, leaving the
	// DB partially cleaned (tracks gone but their join/starred rows dangling, or
	// albums gone with album_artists left behind) until the next full scan.
	return s.TransactionContext(ctx, func(tx *Store) error {
		if err := tx.DeleteTracksNotSeenSince(scanStart); err != nil {
			return err
		}
		return tx.DeleteOrphanedAggregates()
	})
}

// TouchedAggregates holds the album, artist and genre ids that the tracks at a
// set of paths belonged to *before* a targeted rescan rewrote them. A rescan can
// only empty an aggregate it moves a track away from, so this "before" set is the
// only set its prune has to check — every aggregate the edit moves a track *to*
// is, by definition, still populated.
// It snapshots only what the touched tracks pointed at before the edit; an
// aggregate orphaned as collateral of a track moving into a different
// pre-existing album (whose credits reconcile then overwrites) is not captured
// here and is left to the scheduled scan's Cleanup.
type TouchedAggregates struct {
	AlbumIDs  []uint
	ArtistIDs []uint
	GenreIDs  []uint
}

// TouchedAggregatesForPaths captures the album/artist/genre ids reachable from
// the tracks currently stored at paths. Call it before reconcile runs: afterwards
// the rows have already been re-pointed and the "before" membership is gone. Ids
// are de-duplicated and returned ascending so the result is deterministic.
func (s *Store) TouchedAggregatesForPaths(paths []string) (TouchedAggregates, error) {
	albumSet := map[uint]struct{}{}
	artistSet := map[uint]struct{}{}
	genreSet := map[uint]struct{}{}

	type trackRow struct {
		ID      uint
		AlbumID uint
	}

	for i := 0; i < len(paths); i += chunkSize {
		end := i + chunkSize
		if end > len(paths) {
			end = len(paths)
		}
		chunk := paths[i:end]

		var rows []trackRow
		if err := s.db.Model(&model.Track{}).Select("id, album_id").
			Where("file_path IN ?", chunk).Scan(&rows).Error; err != nil {
			return TouchedAggregates{}, fmt.Errorf("touched aggregates: tracks: %w", err)
		}

		trackIDs := make([]uint, 0, len(rows))
		for _, r := range rows {
			trackIDs = append(trackIDs, r.ID)
			albumSet[r.AlbumID] = struct{}{}
		}
		if len(trackIDs) == 0 {
			continue
		}
		if err := pluckInto(s.db.Model(&model.TrackArtist{}).Where("track_id IN ?", trackIDs), "artist_id", artistSet); err != nil {
			return TouchedAggregates{}, fmt.Errorf("touched aggregates: track artists: %w", err)
		}
		if err := pluckInto(s.db.Model(&model.TrackGenre{}).Where("track_id IN ?", trackIDs), "genre_id", genreSet); err != nil {
			return TouchedAggregates{}, fmt.Errorf("touched aggregates: track genres: %w", err)
		}
	}

	albumIDs := sortedKeys(albumSet)
	for i := 0; i < len(albumIDs); i += chunkSize {
		end := i + chunkSize
		if end > len(albumIDs) {
			end = len(albumIDs)
		}
		chunk := albumIDs[i:end]
		if err := pluckInto(s.db.Model(&model.AlbumArtist{}).Where("album_id IN ?", chunk), "artist_id", artistSet); err != nil {
			return TouchedAggregates{}, fmt.Errorf("touched aggregates: album artists: %w", err)
		}
		if err := pluckInto(s.db.Model(&model.AlbumGenre{}).Where("album_id IN ?", chunk), "genre_id", genreSet); err != nil {
			return TouchedAggregates{}, fmt.Errorf("touched aggregates: album genres: %w", err)
		}
	}

	return TouchedAggregates{
		AlbumIDs:  albumIDs,
		ArtistIDs: sortedKeys(artistSet),
		GenreIDs:  sortedKeys(genreSet),
	}, nil
}

// pluckInto reads a single uint column from tx and adds every value to set.
func pluckInto(tx *gorm.DB, column string, set map[uint]struct{}) error {
	var ids []uint
	if err := tx.Distinct().Pluck(column, &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return nil
}

// sortedKeys returns the map's keys ascending.
func sortedKeys(set map[uint]struct{}) []uint {
	out := make([]uint, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// PruneOrphanedAggregates deletes only the album/artist/genre rows in t that the
// edit could have emptied, plus the album join rows and album/artist stars that
// dangle when one of those albums or artists is deleted. It is the targeted
// counterpart to DeleteOrphanedAggregates — same emptiness rules, restricted to a
// candidate set — so its cost scales with the edit, not the library. The
// exhaustive whole-DB sweep stays in Cleanup, run off the request path by the
// scheduled scan.
//
// A targeted rescan never deletes a track row (the metadata editor is file-only),
// so no track-keyed join can dangle here. If that ever changes, the scheduled
// scan's Cleanup is the backstop.
func (s *Store) PruneOrphanedAggregates(ctx context.Context, t TouchedAggregates) error {
	return s.TransactionContext(ctx, func(tx *Store) error {
		// Albums first, then the album-keyed join and starred rows, so the
		// "album no longer exists" predicate below sees the deletions.
		if err := execInChunks(tx.db, t.AlbumIDs,
			`DELETE FROM albums WHERE id IN ? AND id NOT IN (SELECT DISTINCT album_id FROM tracks)`); err != nil {
			return fmt.Errorf("prune albums: %w", err)
		}
		if err := execInChunks(tx.db, t.AlbumIDs,
			`DELETE FROM album_artists WHERE album_id IN ? AND album_id NOT IN (SELECT id FROM albums)`); err != nil {
			return fmt.Errorf("prune album artists: %w", err)
		}
		if err := execInChunks(tx.db, t.AlbumIDs,
			`DELETE FROM album_genres WHERE album_id IN ? AND album_id NOT IN (SELECT id FROM albums)`); err != nil {
			return fmt.Errorf("prune album genres: %w", err)
		}
		if err := execInChunks(tx.db, t.AlbumIDs,
			`DELETE FROM starred_items WHERE item_type = 'album' AND item_id IN ? AND item_id NOT IN (SELECT id FROM albums)`); err != nil {
			return fmt.Errorf("prune album stars: %w", err)
		}
		// Artists: orphaned only once their album_artists rows above are gone.
		if err := execInChunks(tx.db, t.ArtistIDs,
			`DELETE FROM artists WHERE id IN ? AND id NOT IN (SELECT DISTINCT artist_id FROM album_artists) AND id NOT IN (SELECT DISTINCT artist_id FROM track_artists)`); err != nil {
			return fmt.Errorf("prune artists: %w", err)
		}
		if err := execInChunks(tx.db, t.ArtistIDs,
			`DELETE FROM starred_items WHERE item_type = 'artist' AND item_id IN ? AND item_id NOT IN (SELECT id FROM artists)`); err != nil {
			return fmt.Errorf("prune artist stars: %w", err)
		}
		if err := execInChunks(tx.db, t.GenreIDs,
			`DELETE FROM genres WHERE id IN ? AND id NOT IN (SELECT DISTINCT genre_id FROM track_genres) AND id NOT IN (SELECT DISTINCT genre_id FROM album_genres)`); err != nil {
			return fmt.Errorf("prune genres: %w", err)
		}
		return nil
	})
}

// execInChunks runs query once per chunkSize-sized slice of ids, binding the
// chunk to the query's single `IN ?`. An empty ids slice runs nothing.
func execInChunks(db *gorm.DB, ids []uint, query string) error {
	for i := 0; i < len(ids); i += chunkSize {
		end := i + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		if err := db.Exec(query, ids[i:end]).Error; err != nil {
			return err
		}
	}
	return nil
}
