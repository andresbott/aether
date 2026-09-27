package store

import (
	"time"

	"github.com/andresbott/aether/internal/model"
)

// RecordPlaylistPlay appends one play row for a playlist. Mirrors RecordPlay for
// tracks: append-only, so the count is the row count.
func (s *Store) RecordPlaylistPlay(owner string, playlistID uint, playedAt time.Time) error {
	pp := model.PlaylistPlay{Owner: owner, PlaylistID: playlistID, PlayedAt: playedAt}
	return s.db.Create(&pp).Error
}

// PlaylistStat holds aggregate play figures for one playlist.
type PlaylistStat struct {
	PlaylistID uint
	PlayCount  int
	LastPlayed time.Time
}

// playlistStatRow is a temporary struct for scanning raw query results.
type playlistStatRow struct {
	PlaylistID uint   `gorm:"column:playlist_id"`
	PlayCount  int    `gorm:"column:play_count"`
	LastPlayed string `gorm:"column:last_played"`
}

// PlaylistStats returns play count and last-played time per playlist for the
// given IDs, in one grouped query. Playlists that were never played are absent
// from the map (same contract as AlbumTrackStats).
func (s *Store) PlaylistStats(owner string, playlistIDs []uint) (map[uint]PlaylistStat, error) {
	out := map[uint]PlaylistStat{}
	if len(playlistIDs) == 0 {
		return out, nil
	}
	var rows []playlistStatRow
	if err := s.db.Model(&model.PlaylistPlay{}).
		Select("playlist_id, COUNT(*) AS play_count, datetime(MAX(played_at)) AS last_played").
		Where("playlist_id IN ?", playlistIDs).
		Where("owner = ?", owner).
		Group("playlist_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		stat := PlaylistStat{
			PlaylistID: r.PlaylistID,
			PlayCount:  r.PlayCount,
		}
		// datetime() in SQLite normalizes the timestamp to "YYYY-MM-DD HH:MM:SS" format.
		// If the string is malformed (e.g. a far-future year unparseable by Go's time
		// package), degrade gracefully: keep the play count and leave LastPlayed zero.
		t, err := time.Parse("2006-01-02 15:04:05", r.LastPlayed)
		if err == nil {
			stat.LastPlayed = t
		}
		out[r.PlaylistID] = stat
	}
	return out, nil
}
