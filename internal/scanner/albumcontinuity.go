// internal/scanner/albumcontinuity.go
package scanner

import (
	"log/slog"

	"github.com/andresbott/aether/internal/store"
)

// planAlbumContinuity retags album rows in place when this batch retags an
// album in its entirety, so the row — and everything keyed on its id: stars,
// the manual cover in the asset store, created_at and therefore both the
// "newest" ordering and the discovery feed's recency term, plus every
// client-cached /album/:id — survives an edit that changes the album's identity.
//
// It runs before the per-track loop and only ever rewrites identity columns:
// once the row carries the new tuple, FindOrCreateAlbum matches it and the
// whole per-track path is unchanged. Anything this cannot prove is left to
// FindOrCreateAlbum, which creates a new row exactly as it always did.
//
// The proof, per album: every track the album currently holds is in this batch,
// and all of them resolve to the same new identity. That is deliberately
// conservative — a false rename would merge two albums that should stay apart,
// while a missed one only reproduces today's behaviour. Known conservative
// misses: a track deleted from disk still counts until Cleanup runs, and an
// album spanning two libraries is only ever half a batch because reconcile runs
// per library.
func (s *Scanner) planAlbumContinuity(results []tagResult) error {
	if len(results) == 0 {
		return nil
	}
	paths := make([]string, 0, len(results))
	want := make(map[string]store.AlbumIdentity, len(results))
	for _, tr := range results {
		paths = append(paths, tr.walk.FilePath)
		want[tr.walk.FilePath] = AlbumIdentityOf(tr.meta)
	}

	return s.store.Transaction(func(tx *store.Store) error {
		// Reads and the UPDATE share one transaction: the counts below are only
		// a valid proof if nothing inserts a track into the album in between.
		current, err := tx.TrackAlbumIDs(paths)
		if err != nil {
			return err
		}
		if len(current) == 0 {
			return nil // nothing indexed yet — every track in this batch is new
		}

		batch := map[uint][]store.AlbumIdentity{}
		for path, albumID := range current {
			batch[albumID] = append(batch[albumID], want[path])
		}
		ids := make([]uint, 0, len(batch))
		for id := range batch {
			ids = append(ids, id)
		}
		counts, err := tx.AlbumTrackCounts(ids)
		if err != nil {
			return err
		}
		held, err := tx.AlbumIdentities(ids)
		if err != nil {
			return err
		}

		for albumID, idents := range batch {
			if len(idents) != counts[albumID] {
				continue // part of the album is not in this batch: a split
			}
			if !sameAlbumIdentity(idents) {
				continue // the batch disagrees with itself: a split
			}
			target := idents[0]
			if target.Key() == held[albumID].Key() {
				continue // identity unchanged: nothing to plan
			}
			taken, err := tx.AlbumIDForIdentity(target.Key())
			if err != nil {
				return err
			}
			if taken != 0 {
				continue // another row already holds it: a merge, not a rename
			}
			if err := tx.RetagAlbum(albumID, target); err != nil {
				if store.IsUniqueViolation(err) {
					continue // a concurrent pass got there first
				}
				return err
			}
			slog.Debug("album retagged in place",
				"album_id", albumID, "name", target.Name, "album_artist_norm", target.AlbumArtistNorm)
		}
		return nil
	})
}

// sameAlbumIdentity reports whether every entry resolves to the same album.
func sameAlbumIdentity(idents []store.AlbumIdentity) bool {
	if len(idents) == 0 {
		return false
	}
	for _, i := range idents[1:] {
		if i.Key() != idents[0].Key() {
			return false
		}
	}
	return true
}
