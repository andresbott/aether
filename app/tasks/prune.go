// app/tasks/prune.go
package tasks

import (
	"context"
	"log/slog"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/imagecache"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/taskrunner"
)

const PruneTaskName = "prune"

var PruneTaskDef = TaskDef{
	ID:   PruneTaskName,
	Name: "Prune Orphaned Covers",
	Description: "Delete cached cover thumbnails and stored cover images whose album, artist, " +
		"genre, playlist or radio station no longer exists. Safe to run any time; " +
		"hand-uploaded covers are never removed.",
}

// pruneKinds is the whitelist of asset kinds the prune task reconciles. It is
// deliberately explicit: the image cache also holds the metadata editor's
// "editor" thumbnails (keyed by source-file path, a different lifecycle), which
// this task must never sweep. Reconcile only ever walks the kinds it is given,
// so anything outside this list is untouched by construction.
var pruneKinds = []string{
	assetstore.KindAlbum,
	assetstore.KindArtist,
	assetstore.KindGenre,
	assetstore.KindPlaylist,
	assetstore.KindRadio,
}

// NewPruneTaskFn builds the orphan-cover GC task body. It computes the set of
// live asset keys from the database, then reconciles both the image cache and
// the asset store against it: cache derivatives and auto-fetched stored covers
// whose entity is gone are removed; hand-uploaded covers are preserved (see
// assetstore.Reconcile). This is the sweep that the inline per-id deletes cannot
// cover — the scanner's bulk aggregate pruning, key drift, a dropped DB.
//
// It shares the library-writes exclusion group with scan/reindex (see
// server.go), so it never runs while the index is being written. Progress is
// reported one unit per kind, with the current kind as the stage label.
func NewPruneTaskFn(s *store.Store, images *imagecache.Cache, assets *assetstore.Store) func(ctx context.Context, log *slog.Logger, prog taskrunner.Progress) error {
	return func(ctx context.Context, log *slog.Logger, prog taskrunner.Progress) error {
		ids, err := s.AssetIdentities()
		if err != nil {
			log.Error("prune: load asset identities", slog.String("error", err.Error()))
			return err
		}
		imagesLive, assetsLive := liveAssetKeys(ids)
		prog.SetTotal(int64(len(pruneKinds)))

		var cacheRemoved, assetRemoved, assetKeptManual int
		for _, kind := range pruneKinds {
			if err := ctx.Err(); err != nil {
				return err
			}
			prog.SetStage("Pruning " + kind + "…")
			cr, err := images.Reconcile(kind, imagesLive[kind])
			if err != nil {
				log.Error("prune: reconcile image cache", slog.String("kind", kind), slog.String("error", err.Error()))
				return err
			}
			ar, akm, err := assets.Reconcile(kind, assetsLive[kind])
			if err != nil {
				log.Error("prune: reconcile asset store", slog.String("kind", kind), slog.String("error", err.Error()))
				return err
			}
			cacheRemoved += cr
			assetRemoved += ar
			assetKeptManual += akm
			if cr > 0 || ar > 0 || akm > 0 {
				log.Info("prune: reconciled kind",
					slog.String("kind", kind),
					slog.Int("cache_removed", cr),
					slog.Int("asset_removed", ar),
					slog.Int("asset_kept_manual", akm))
			}
			prog.Inc(1)
		}

		log.Info("prune complete",
			slog.Int("cache_removed", cacheRemoved),
			slog.Int("asset_removed", assetRemoved),
			slog.Int("asset_kept_manual", assetKeptManual))
		return nil
	}
}

// liveKeys maps each cover-owning kind to the set of asset keys still in use.
type liveKeys map[string]map[string]struct{}

func newLiveKeys() liveKeys {
	return liveKeys{
		assetstore.KindAlbum:    {},
		assetstore.KindArtist:   {},
		assetstore.KindGenre:    {},
		assetstore.KindPlaylist: {},
		assetstore.KindRadio:    {},
	}
}

// add records key as live under kind, dropping empty keys (e.g. a legacy
// playlist with no UUID, which can own no directory).
func (l liveKeys) add(kind, key string) {
	if key == "" {
		return
	}
	l[kind][key] = struct{}{}
}

// liveAssetKeys returns the live key sets for the image cache and the asset
// store, using the same assetkey functions the handlers use so the keys match
// the ones on disk exactly.
//
// They differ only for artists. Derivatives are always cached under the current
// ArtistOf (see media.go), so the image cache's live set is ArtistOf alone — an
// artist that gained an MBID has dead name-hash-slot derivatives that prune must
// reclaim. The asset store, by contrast, still SERVES the name-hash slot as a
// fallback (artistCoverMeta), so that slot stays a live source there and both
// slots are kept.
func liveAssetKeys(ids store.AssetIdentities) (images, assets liveKeys) {
	images, assets = newLiveKeys(), newLiveKeys()
	both := func(kind, key string) {
		images.add(kind, key)
		assets.add(kind, key)
	}
	for i := range ids.Albums {
		both(assetstore.KindAlbum, assetkey.AlbumOf(&ids.Albums[i]))
	}
	for i := range ids.Artists {
		a := &ids.Artists[i]
		both(assetstore.KindArtist, assetkey.ArtistOf(a))
		// The name-hash slot is a live SOURCE only in the asset store; its
		// derivatives live under ArtistOf, so leaving it out of images-live is
		// exactly what lets prune reclaim the MBID-drift leak.
		assets.add(assetstore.KindArtist, assetkey.Artist("", a.NameNorm))
	}
	for i := range ids.Genres {
		both(assetstore.KindGenre, assetkey.GenreOf(&ids.Genres[i]))
	}
	for i := range ids.Playlists {
		both(assetstore.KindPlaylist, assetkey.PlaylistOf(&ids.Playlists[i]))
	}
	for i := range ids.Radios {
		both(assetstore.KindRadio, assetkey.Radio(ids.Radios[i].StreamURL))
	}
	return images, assets
}
