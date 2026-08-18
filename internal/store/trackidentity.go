package store

import (
	"path/filepath"
	"time"

	"github.com/andresbott/aether/internal/model"
)

// TrackRow is the subset of a track row the move proof reads: enough to decide
// whether the file that vanished from one path is the file that appeared at
// another.
type TrackRow struct {
	ID          uint      `gorm:"column:id"`
	FilePath    string    `gorm:"column:file_path"`
	LibraryID   uint      `gorm:"column:library_id"`
	FileSize    int64     `gorm:"column:file_size"`
	FileModTime time.Time `gorm:"column:file_mod_time"`
	Duration    int       `gorm:"column:duration"`
	Title       string    `gorm:"column:title"`
}

// KnownTrackPaths reports which of paths already have a track row. The
// complement — the paths absent from the result — is the set that a move could
// have produced, and the only set the re-link pre-pass considers.
func (s *Store) KnownTrackPaths(paths []string) (map[string]bool, error) {
	out := make(map[string]bool, len(paths))
	for i := 0; i < len(paths); i += identityChunkSize {
		end := i + identityChunkSize
		if end > len(paths) {
			end = len(paths)
		}
		var found []string
		if err := s.db.Table("tracks").
			Where("file_path IN ?", paths[i:end]).
			Pluck("file_path", &found).Error; err != nil {
			return nil, err
		}
		for _, p := range found {
			out[p] = true
		}
	}
	return out, nil
}

// TracksByFileSizes returns every track row whose file_size is one of sizes.
// Size is the cheap discriminator of a moved file; the rest of the proof is
// checked by the caller. There is deliberately no index on the column — this
// runs at most once per reconcile batch, next to reading every file's tags.
func (s *Store) TracksByFileSizes(sizes []int64) ([]TrackRow, error) {
	var out []TrackRow
	for i := 0; i < len(sizes); i += identityChunkSize {
		end := i + identityChunkSize
		if end > len(sizes) {
			end = len(sizes)
		}
		var rows []TrackRow
		if err := s.db.Table("tracks").
			Select("id, file_path, library_id, file_size, file_mod_time, duration, title").
			Where("file_size IN ?", sizes[i:end]).
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	return out, nil
}

// RelinkTrack re-points a track row at the path its file moved to, keeping the
// row's id and therefore everything keyed on it: playlist memberships, play
// history, stars and play-queue entries, all of which DeleteOrphanedAggregates
// hard-deletes when a row dies and a new one takes its place.
//
// oldPath is in the WHERE clause on purpose. The caller proves the move with
// filesystem reads it must not hold a write transaction across, so the update
// itself is the check: a row whose path changed underneath reports
// relinked=false and is skipped rather than overwritten.
func (s *Store) RelinkTrack(id uint, oldPath, newPath string, libraryID uint) (bool, error) {
	res := s.db.Model(&model.Track{}).
		Where("id = ? AND file_path = ?", id, oldPath).
		Updates(map[string]any{
			"file_path":  newPath,
			"filename":   filepath.Base(newPath),
			"library_id": libraryID,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}
