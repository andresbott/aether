// internal/scanner/reconcile.go
package scanner

import (
	"context"
	"log/slog"
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

func (s *Scanner) reconcile(ctx context.Context, libRoot string, results []tagResult, scanStart time.Time) (reconcileStats, error) {
	var stats reconcileStats
	// One directory listing per artist folder is enough for the whole pass.
	imageCache := map[string]string{}

	// Preserve album rows across a wholesale retag before any track is
	// reconciled: once a row carries the new identity, FindOrCreateAlbum below
	// finds it instead of creating a second row. A failure here is not fatal —
	// it degrades to the old behaviour (a new row and a new id), which is worse
	// than a preserved id but better than a failed scan.
	if ctx.Err() == nil {
		if err := s.planAlbumContinuity(results); err != nil {
			slog.Warn("album continuity planning failed; retagged albums may get new ids", "err", err)
		}
	}

	for _, tr := range results {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}

		if err := s.store.Transaction(func(tx *store.Store) error {
			return s.reconcileTrack(tx, libRoot, imageCache, tr, scanStart, &stats)
		}); err != nil {
			slog.Warn("reconcile track failed, skipping", "path", tr.walk.FilePath, "err", err)
			continue
		}
		stats.Processed++
	}

	return stats, nil
}

func (s *Scanner) reconcileTrack(tx *store.Store, libRoot string, imageCache map[string]string, tr tagResult, scanStart time.Time, stats *reconcileStats) error {
	meta := tr.meta

	// Resolve artists — tag values are taken as-is; multi-value frames come
	// through the reader as separate list entries already.
	artistNames := TrackArtistNames(meta)
	artists, err := tx.FindOrCreateArtists(artistNames, alignMBIDs(artistNames, meta.MBArtistID))
	if err != nil {
		return err
	}

	// Resolve album artists
	albumArtistNames := AlbumArtistNames(meta)
	albumArtists, err := tx.FindOrCreateArtists(albumArtistNames, alignMBIDs(albumArtistNames, meta.MBAlbumArtistID))
	if err != nil {
		return err
	}

	// Detect an artist-folder image for every artist this track mentions.
	if err := syncArtistImages(tx, libRoot, imageCache, tr.walk.FilePath, artists, albumArtists); err != nil {
		return err
	}

	// Resolve genres
	genreNames := nonEmpty(meta.Genre)
	genres, err := tx.FindOrCreateGenres(genreNames)
	if err != nil {
		return err
	}

	// Resolve album. AlbumIdentityOf is the same function planAlbumContinuity
	// used to decide whether this album's row could be retagged in place, so a
	// row it retagged is found here rather than duplicated.
	ident := AlbumIdentityOf(meta)
	album, err := tx.FindOrCreateAlbum(ident)
	if err != nil {
		return err
	}

	// Update album metadata
	album.Year = meta.Year
	album.Compilation = meta.Compilation
	album.ReleaseType = meta.ReleaseType
	album.HasEmbeddedCover = meta.HasCover

	// Detect external cover. Always re-detect for art in THIS directory (so
	// cover.jpg supersedes folder.jpg and a deleted file clears), but never let
	// a disc folder with no art blank out a cover found in a sibling folder of
	// the same album: an album can span several directories (a multi-disc release
	// laid out as CD 1/, CD 2/). Re-detection is the only thing that sets this
	// field — the metadata editor writes art files and lets the rescan pick them up.
	dir := filepath.Dir(tr.walk.FilePath)
	detected := detectCoverInDir(dir)
	// Always re-detect for art in THIS directory (so cover.jpg supersedes
	// folder.jpg and a deleted file clears), but never let a disc folder with no
	// art blank out a cover found in a sibling folder of the same album: an album
	// can span several directories (a multi-disc release laid out as CD 1/, CD 2/).
	if detected != "" || !IsUsableCoverPath(album.CoverPath) || filepath.Dir(album.CoverPath) == dir {
		album.CoverPath = detected
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
	track.FileSize = tr.walk.FileSize
	track.FileModTime = tr.walk.ModTime
	// LastSeenAt is monotonic: only ever advanced, never moved backwards.
	// It is the liveness marker store.Cleanup uses to delete "tracks nobody
	// saw this run" (last_seen_at < scanStart), and a targeted rescan
	// (RescanPaths) runs concurrently with scheduled scans using its own,
	// possibly older, scanStart. Overwriting a newer marker with an older one
	// would make a live track look stale to a scan that is already in flight
	// and get it deleted — taking its playlist memberships, play history and
	// stars with it. A brand-new track has the zero time, so it still gets its
	// marker set here.
	if scanStart.After(track.LastSeenAt) {
		track.LastSeenAt = scanStart
	}
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

// syncArtistImages records, for each artist the track mentions, the image found
// in the artist's own folder on disk (`<collection>/<artist>/artist.jpg`). A
// path already on record is re-checked rather than trusted: the file may have
// been removed or the tree reorganised. imageCache holds one detection result
// per (artist folder scope) so a 200-track library does not list the same
// directory 200 times.
func syncArtistImages(tx *store.Store, libRoot string, imageCache map[string]string, trackPath string, artistSets ...[]*model.Artist) error {
	seen := map[uint]bool{}
	for _, set := range artistSets {
		for _, a := range set {
			if seen[a.ID] {
				continue
			}
			seen[a.ID] = true

			key := filepath.Dir(trackPath) + "\x00" + a.NameNorm
			img, cached := imageCache[key]
			if !cached {
				img = DetectArtistImage(libRoot, trackPath, a.Name)
				imageCache[key] = img
			}
			// Keep a path that is still valid when this track's own tree yields
			// nothing: another library layout may have supplied it.
			if img == "" && IsUsableArtistImagePath(a.ImagePath) {
				continue
			}
			if img == a.ImagePath {
				continue
			}
			if err := tx.SetArtistImagePath(a.ID, img); err != nil {
				return err
			}
			a.ImagePath = img
		}
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

// alignMBIDs returns mbids only when they line up 1:1 with names; otherwise nil.
// Multi-value splitting can desync the two lists, so we assign MBIDs only in the
// unambiguous case and fall back to the generated avatar otherwise.
func alignMBIDs(names, mbids []string) []string {
	if len(mbids) == len(names) {
		return mbids
	}
	return nil
}
