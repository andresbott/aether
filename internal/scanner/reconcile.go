// internal/scanner/reconcile.go
package scanner

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/andresbott/aether/internal/artistimage"
	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/unidecode"
)

type reconcileStats struct {
	Processed int
	New       int
	Updated   int
}

type artistRekey struct {
	nameNorm string
	mbid     string
}

func (s *Scanner) reconcile(ctx context.Context, libRoot string, results []tagResult, scanStart time.Time) (reconcileStats, error) {
	var stats reconcileStats
	// Artist-folder images are reconciled in one pass after every track is in
	// (reconcileArtistImages), not per track. probes collects, per artist touched
	// this run, the track directories to search and the path already on the row.
	probes := map[uint]*artistImageProbe{}

	// Re-link tracks whose file moved before anything else looks at paths: a
	// re-pointed row keeps its id, so playlists, play history, stars and queue
	// entries survive. Doing it first also means planAlbumContinuity below sees
	// the moved tracks at their new paths instead of counting them as missing and
	// mistaking a move for a split. A failure here is not fatal — it degrades to
	// the old behaviour (delete plus insert, and the user-authored rows go with
	// it), which is worse than a re-link but better than a failed scan.
	if ctx.Err() == nil {
		if err := s.planTrackContinuity(results); err != nil {
			slog.Warn("track continuity planning failed; moved files lose playlists, history and stars", "err", err)
		}
	}

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

	var pendingArtistRekeys []artistRekey

	for _, tr := range results {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}

		pendingArtistRekeys = pendingArtistRekeys[:0]
		if err := s.store.Transaction(func(tx *store.Store) error {
			return s.reconcileTrack(tx, probes, tr, scanStart, &stats, &pendingArtistRekeys)
		}); err != nil {
			slog.Warn("reconcile track failed, skipping", "path", tr.walk.FilePath, "err", err)
			continue
		}
		for _, rk := range pendingArtistRekeys {
			s.rekeyArtistImages(rk)
		}
		stats.Processed++
	}

	// Every artist folder is listed at most once per run here, instead of once
	// per track inside the loop above.
	s.reconcileArtistImages(libRoot, probes)

	return stats, nil
}

func (s *Scanner) reconcileTrack(tx *store.Store, probes map[uint]*artistImageProbe, tr tagResult, scanStart time.Time, stats *reconcileStats, pendingArtistRekeys *[]artistRekey) error {
	meta := tr.meta

	// Resolve artists — tag values are taken as-is; multi-value frames come
	// through the reader as separate list entries already.
	artistNames := TrackArtistNames(meta)
	artists, gainedTrackArtists, err := tx.FindOrCreateArtists(artistNames, alignMBIDs(artistNames, meta.MBArtistID))
	if err != nil {
		return err
	}
	for _, a := range gainedTrackArtists {
		*pendingArtistRekeys = append(*pendingArtistRekeys, artistRekey{nameNorm: a.NameNorm, mbid: a.MBArtistID})
	}

	// Resolve album artists
	albumArtistNames := AlbumArtistNames(meta)
	albumArtists, gainedAlbumArtists, err := tx.FindOrCreateArtists(albumArtistNames, alignMBIDs(albumArtistNames, meta.MBAlbumArtistID))
	if err != nil {
		return err
	}
	for _, a := range gainedAlbumArtists {
		*pendingArtistRekeys = append(*pendingArtistRekeys, artistRekey{nameNorm: a.NameNorm, mbid: a.MBArtistID})
	}

	// Record every artist this track mentions for the post-loop artist-image pass.
	recordArtistProbes(probes, tr.walk.FilePath, artists, albumArtists)

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
	// Only ever overwrite the stored hash with a real one. An unsupported format
	// or an unreadable payload yields "", and erasing a value an earlier scan
	// recorded would silently disarm the move proof for that track.
	if tr.audioHash != "" {
		track.AudioHash = tr.audioHash
	}
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

// artistImageProbe holds what the post-loop artist-image pass needs for one
// artist: the name (for the folder-name match), the ImagePath already on the row
// (so a still-valid path is kept when the disk yields nothing), and the distinct
// track directories seen this run to search from.
type artistImageProbe struct {
	name        string
	currentPath string
	dirs        []string
}

// recordArtistProbes notes, for every artist a track credits, the directory the
// track sits in. Directories are de-duplicated per artist so an artist with many
// albums — or a multi-disc release split over CD 1/, CD 2/ — lists each folder
// once. The ImagePath is captured the first time an artist is seen.
func recordArtistProbes(probes map[uint]*artistImageProbe, trackPath string, artistSets ...[]*model.Artist) {
	dir := filepath.Dir(trackPath)
	for _, set := range artistSets {
		for _, a := range set {
			p := probes[a.ID]
			if p == nil {
				p = &artistImageProbe{name: a.Name, currentPath: a.ImagePath}
				probes[a.ID] = p
			}
			seen := false
			for _, d := range p.dirs {
				if d == dir {
					seen = true
					break
				}
			}
			if !seen {
				p.dirs = append(p.dirs, dir)
			}
		}
	}
}

// reconcileArtistImages runs once per library after every track is reconciled:
// for each artist touched this run it records the artist-folder image found on
// disk (<collection>/<artist>/artist.jpg). A path already on the row is
// re-checked, not trusted, and kept only when the disk yields nothing — another
// library's layout may still hold it. Empty detection with no usable stored path
// clears the row. Failures are logged, never fatal: the field is a soft fallback.
func (s *Scanner) reconcileArtistImages(libRoot string, probes map[uint]*artistImageProbe) {
	for id, p := range probes {
		img := ""
		// First directory that yields an image wins (deterministic, first-seen
		// order) — the old per-track code instead let the last-processed track win.
		for _, dir := range p.dirs {
			if got := artistimage.Detect(libRoot, dir, p.name); got != "" {
				img = got
				break
			}
		}
		if img == "" && artistimage.IsUsablePath(p.currentPath) {
			continue
		}
		if img == p.currentPath {
			continue
		}
		if err := s.store.SetArtistImagePath(id, img); err != nil {
			slog.Warn("set artist image path failed", "artist_id", id, "err", err)
		}
	}
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

// rekeyArtistImages moves the artist's stored images from the name-hash key to
// the MBID key, so a manual cover survives the MBID gain. It is called after a
// successful per-track transaction and is optional (no hook, no error). Any
// failure is tolerated: the row moved and the image did not, which is today's
// behaviour and recoverable.
func (s *Scanner) rekeyArtistImages(rk artistRekey) {
	if s.cfg.AssetRekeyer == nil {
		return
	}
	oldKey := assetkey.Artist("", rk.nameNorm)
	newKey := assetkey.Artist(rk.mbid, rk.nameNorm)
	// "artist" is assetstore.KindArtist, duplicated to keep this package free of an assetstore import.
	if err := s.cfg.AssetRekeyer.Rekey("artist", oldKey, newKey); err != nil {
		slog.Warn("artist image re-key failed; the row moved but the stored images did not",
			"name_norm", rk.nameNorm, "mbid", rk.mbid, "old_key", oldKey, "new_key", newKey, "err", err)
	}
}
