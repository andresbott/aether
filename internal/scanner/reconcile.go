// internal/scanner/reconcile.go
package scanner

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/unidecode"
)

type reconcileStats struct {
	Processed int
	New       int
	Updated   int
}

func (s *Scanner) reconcile(ctx context.Context, results []tagResult, scanStart time.Time) (reconcileStats, error) {
	var stats reconcileStats

	for _, tr := range results {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}

		if err := s.store.Transaction(func(tx *store.Store) error {
			return s.reconcileTrack(tx, tr, scanStart, &stats)
		}); err != nil {
			slog.Warn("reconcile track failed, skipping", "path", tr.walk.FilePath, "err", err)
			continue
		}
		stats.Processed++
	}

	return stats, nil
}

func (s *Scanner) reconcileTrack(tx *store.Store, tr tagResult, scanStart time.Time, stats *reconcileStats) error {
	meta := tr.meta

	// Resolve artists — tag values are taken as-is; multi-value frames come
	// through the reader as separate list entries already.
	artistNames := nonEmpty(meta.Artist)
	if len(artistNames) == 0 {
		artistNames = []string{"Unknown Artist"}
	}
	artists, err := tx.FindOrCreateArtists(artistNames, alignMBIDs(artistNames, meta.MBArtistID))
	if err != nil {
		return err
	}

	// Resolve album artists
	albumArtistNames := nonEmpty(meta.AlbumArtist)
	if len(albumArtistNames) == 0 {
		if meta.Compilation {
			albumArtistNames = []string{"Various Artists"}
		} else {
			albumArtistNames = artistNames
		}
	}
	albumArtists, err := tx.FindOrCreateArtists(albumArtistNames, alignMBIDs(albumArtistNames, meta.MBAlbumArtistID))
	if err != nil {
		return err
	}

	// Resolve genres
	genreNames := nonEmpty(meta.Genre)
	genres, err := tx.FindOrCreateGenres(genreNames)
	if err != nil {
		return err
	}

	// Resolve album
	albumName := meta.Album
	if albumName == "" {
		albumName = "Unknown Album"
	}
	albumArtistNorm := unidecode.Normalize(albumArtistNames[0])
	album, err := tx.FindOrCreateAlbum(albumName, albumArtistNorm, meta.MBReleaseID)
	if err != nil {
		return err
	}

	// Update album metadata
	album.Year = meta.Year
	album.Compilation = meta.Compilation
	album.ReleaseType = meta.ReleaseType
	album.HasEmbeddedCover = meta.HasCover

	// Detect external cover
	dir := filepath.Dir(tr.walk.FilePath)
	if album.CoverPath == "" {
		if cover := detectCoverInDir(dir); cover != "" {
			album.CoverPath = cover
		}
	}

	db := tx.DB()
	if err := db.Save(album).Error; err != nil {
		return err
	}

	// Update album artists association
	if err := db.Model(album).Association("Artists").Replace(albumArtists); err != nil {
		return err
	}

	// Update album genres association
	if err := db.Model(album).Association("Genres").Replace(genres); err != nil {
		return err
	}

	// Upsert track
	var track model.Track
	result := db.Where("file_path = ?", tr.walk.FilePath).First(&track)
	isNew := result.Error != nil

	track.AlbumID = album.ID
	track.LibraryID = tr.walk.LibraryID
	track.Filename = filepath.Base(tr.walk.FilePath)
	track.FilePath = tr.walk.FilePath
	track.FileSize = fileSize(tr.walk.FilePath)
	track.FileModTime = tr.walk.ModTime
	track.LastSeenAt = scanStart
	track.Title = meta.Title
	track.TitleNorm = unidecode.Normalize(meta.Title)
	track.TrackNumber = meta.TrackNumber
	track.DiscNumber = meta.DiscNumber
	track.DiscSubtitle = meta.DiscSubtitle
	track.Year = meta.Year
	track.Duration = int(meta.Duration.Seconds())
	track.Bitrate = meta.Bitrate
	track.MBRecordingID = meta.MBRecordingID
	track.Lyrics = meta.Lyrics
	track.ReplayGainTrackGain = meta.ReplayGain.TrackGain
	track.ReplayGainTrackPeak = meta.ReplayGain.TrackPeak
	track.ReplayGainAlbumGain = meta.ReplayGain.AlbumGain
	track.ReplayGainAlbumPeak = meta.ReplayGain.AlbumPeak
	track.HasEmbeddedCover = meta.HasCover

	if err := tx.UpsertTrack(&track, artists, genres); err != nil {
		return err
	}

	if isNew {
		stats.New++
	} else {
		stats.Updated++
	}

	return nil
}

func detectCoverInDir(dir string) string {
	entries, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return ""
	}
	var candidates []string
	for _, e := range entries {
		candidates = append(candidates, filepath.Base(e))
	}
	best := BestCover(candidates)
	if best != "" {
		return filepath.Join(dir, best)
	}
	return ""
}

// nonEmpty drops blank entries from a tag value list.
func nonEmpty(ss []string) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// alignMBIDs returns mbids only when they line up 1:1 with names; otherwise nil.
// Multi-value splitting can desync the two lists, so we assign MBIDs only in the
// unambiguous case and fall back to the generated avatar otherwise.
func alignMBIDs(names, mbids []string) []string {
	if len(mbids) == len(names) {
		return mbids
	}
	return nil
}
