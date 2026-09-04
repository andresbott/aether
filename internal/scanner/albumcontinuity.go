// internal/scanner/albumcontinuity.go
package scanner

import (
	"fmt"
	"log/slog"
	"sort"

	"github.com/andresbott/aether/internal/assetkey"
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
//
// Shape: read a snapshot, plan off it in a pure function, then commit one album
// per transaction. The proof is per album, so the unit of work is too — one
// album's DB error no longer discards every other album's retag in the batch,
// and the scanner never holds a write transaction across a whole library. That
// matches reconcile's own per-track loop, which is one transaction per track.
// Splitting the reads from the writes is only safe because applyAlbumRetag
// re-proves every plan against the live rows inside the writing transaction.
func (s *Scanner) planAlbumContinuity(results []tagResult) error {
	if len(results) == 0 {
		return nil
	}

	snap, err := s.readAlbumSnapshot(results)
	if err != nil {
		return err
	}

	for _, plan := range planAlbumRetags(snap) {
		applied, err := s.applyAlbumRetag(plan)
		if err != nil {
			// Per album, so the rest of the batch still gets its retag. This
			// one degrades to the old behaviour: a new row with a new id.
			slog.Warn("album retag failed, skipping; this album may get a new id",
				"album_id", plan.albumID, "err", err)
			continue
		}
		if !applied {
			continue // the plan no longer held; see applyAlbumRetag
		}

		slog.Info("album retagged in place",
			"album_id", plan.albumID, "merged_from", plan.mergedFrom,
			"prev_name_norm", plan.oldIdent.NameNorm,
			"prev_album_artist_norm", plan.oldIdent.AlbumArtistNorm,
			"prev_mb_release_id", plan.oldIdent.MBReleaseID,
			"name_norm", plan.newIdent.NameNorm,
			"album_artist_norm", plan.newIdent.AlbumArtistNorm,
			"mb_release_id", plan.newIdent.MBReleaseID)

		// After the album's own transaction commits, never inside it: the
		// asset store is not transactional and must not be moved for a write
		// that could still roll back.
		s.rekeyAlbumImages(plan.albumID, plan.oldIdent, plan.newIdent)
	}
	return nil
}

// albumSnapshot is everything planAlbumRetags reasons about. Read in one
// transaction so the counts and the identities describe a single moment: a
// count taken before an insert and an identity taken after it would prove
// something that was never true at once.
type albumSnapshot struct {
	// current maps an already-indexed path to the album row it belongs to.
	// Paths this batch walked for the first time are absent.
	current map[string]uint
	// want maps every path in the batch to the identity its tags resolve to.
	want map[string]store.AlbumIdentity
	// counts is how many tracks each candidate album currently holds, and is
	// the load-bearing half of the proof.
	counts map[uint]int
	// held is the identity each candidate album currently carries.
	held map[uint]store.AlbumIdentity
}

// readAlbumSnapshot reads the state planning needs. Read-only.
func (s *Scanner) readAlbumSnapshot(results []tagResult) (albumSnapshot, error) {
	snap := albumSnapshot{want: make(map[string]store.AlbumIdentity, len(results))}
	paths := make([]string, 0, len(results))
	for _, tr := range results {
		paths = append(paths, tr.walk.FilePath)
		snap.want[tr.walk.FilePath] = AlbumIdentityOf(tr.meta)
	}

	err := s.store.Transaction(func(tx *store.Store) error {
		current, err := tx.TrackAlbumIDs(paths)
		if err != nil {
			return err
		}
		if len(current) == 0 {
			return nil // nothing indexed yet — every track in this batch is new
		}
		ids := make([]uint, 0, len(current))
		seen := make(map[uint]bool, len(current))
		for _, albumID := range current {
			if seen[albumID] {
				continue
			}
			seen[albumID] = true
			ids = append(ids, albumID)
		}
		counts, err := tx.AlbumTrackCounts(ids)
		if err != nil {
			return err
		}
		held, err := tx.AlbumIdentities(ids)
		if err != nil {
			return err
		}
		snap.current, snap.counts, snap.held = current, counts, held
		return nil
	})
	if err != nil {
		return albumSnapshot{}, err
	}
	return snap, nil
}

// albumRetagPlan is one provable in-place retag: album albumID holds trackCount
// tracks and the identity oldIdent, every one of those tracks is in this batch,
// and they all resolve to newIdent. mergedFrom is how many other albums claimed
// the same target and therefore lose their row to this one; their tracks are
// repointed by FindOrCreateAlbum during the per-track pass.
//
// Every field except mergedFrom is re-checked by applyAlbumRetag before it
// writes: the plan is a proposal, not a promise.
type albumRetagPlan struct {
	albumID    uint
	trackCount int
	mergedFrom int
	oldIdent   store.AlbumIdentity
	newIdent   store.AlbumIdentity
}

// planAlbumRetags decides which albums can be retagged in place, in a
// deterministic order. Pure by design: it touches no DB, so every leg of the
// proof — a split, a batch disagreeing with itself, an unchanged identity,
// several albums collapsing into one — is testable without a store.
func planAlbumRetags(snap albumSnapshot) []albumRetagPlan {
	if len(snap.current) == 0 {
		return nil
	}

	batch := map[uint][]store.AlbumIdentity{}
	for path, albumID := range snap.current {
		batch[albumID] = append(batch[albumID], snap.want[path])
	}

	// One row per identity: several albums claiming the same target is a merge,
	// and the map iteration order must not decide who survives.
	claims := map[store.AlbumIdentityKey][]uint{}
	targets := map[store.AlbumIdentityKey]store.AlbumIdentity{}
	for albumID, idents := range batch {
		if len(idents) != snap.counts[albumID] {
			continue // part of the album is not in this batch: a split
		}
		if !sameAlbumIdentity(idents) {
			continue // the batch disagrees with itself: a split
		}
		target := idents[0]
		if target.Key() == snap.held[albumID].Key() {
			continue // identity unchanged: nothing to plan
		}
		claims[target.Key()] = append(claims[target.Key()], albumID)
		targets[target.Key()] = target
	}

	// Stable order so chained renames are deterministic.
	plans := make([]albumRetagPlan, 0, len(claims))
	for _, key := range sortedIdentityKeys(claims) {
		sources := claims[key]
		survivor := pickAlbumSurvivor(sources, snap.counts)
		plans = append(plans, albumRetagPlan{
			albumID:    survivor,
			trackCount: snap.counts[survivor],
			mergedFrom: len(sources) - 1,
			oldIdent:   snap.held[survivor],
			newIdent:   targets[key],
		})
	}
	return plans
}

// applyAlbumRetag commits one plan in its own transaction, re-proving it
// against the live rows first. The re-check is what makes planning off a
// snapshot safe: by the time we write, the snapshot is history, and the counts
// ARE the proof — acting on a stale one would rename an album that has since
// gained or lost tracks, merging two albums that should stay apart. That is the
// one failure this whole path exists to avoid, so every leg is re-read under
// the same transaction as the UPDATE.
//
// Reports whether the retag happened. False with a nil error is a decline, not
// a failure: the plan no longer holds, and FindOrCreateAlbum then creates a new
// row exactly as it did before continuity existed.
func (s *Scanner) applyAlbumRetag(plan albumRetagPlan) (bool, error) {
	applied := false
	err := s.store.Transaction(func(tx *store.Store) error {
		counts, err := tx.AlbumTrackCounts([]uint{plan.albumID})
		if err != nil {
			return err
		}
		if counts[plan.albumID] != plan.trackCount {
			return nil // the album gained or lost tracks: the proof is stale
		}
		held, err := tx.AlbumIdentities([]uint{plan.albumID})
		if err != nil {
			return err
		}
		if held[plan.albumID].Key() != plan.oldIdent.Key() {
			return nil // the row moved under us, or is gone
		}
		taken, err := tx.AlbumIDForIdentity(plan.newIdent.Key())
		if err != nil {
			return err
		}
		if taken != 0 {
			return nil // another row already holds it: a merge, not a rename
		}
		if err := tx.RetagAlbum(plan.albumID, plan.newIdent); err != nil {
			if store.IsUniqueViolation(err) {
				return nil // a concurrent pass got there first
			}
			return fmt.Errorf("retag album %d: %w", plan.albumID, err)
		}
		applied = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return applied, nil
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

// pickAlbumSurvivor chooses which of several albums collapsing into one
// identity keeps its row: the one with the most tracks, lowest id as a
// tiebreak. The others' tracks are repointed at the survivor by
// FindOrCreateAlbum during the per-track pass, and their now-empty rows are
// removed by DeleteOrphanedAggregates. Deliberately independent of map
// iteration order — the survivor must not depend on which tag reader finished
// first.
func pickAlbumSurvivor(sources []uint, counts map[uint]int) uint {
	best := sources[0]
	for _, id := range sources[1:] {
		if counts[id] > counts[best] || (counts[id] == counts[best] && id < best) {
			best = id
		}
	}
	return best
}

// sortedIdentityKeys returns the keys of claims sorted by (NameNorm,
// AlbumArtistNorm, MBReleaseID), making iteration over claims deterministic.
func sortedIdentityKeys(claims map[store.AlbumIdentityKey][]uint) []store.AlbumIdentityKey {
	keys := make([]store.AlbumIdentityKey, 0, len(claims))
	for key := range claims {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].NameNorm != keys[j].NameNorm {
			return keys[i].NameNorm < keys[j].NameNorm
		}
		if keys[i].AlbumArtistNorm != keys[j].AlbumArtistNorm {
			return keys[i].AlbumArtistNorm < keys[j].AlbumArtistNorm
		}
		return keys[i].MBReleaseID < keys[j].MBReleaseID
	})
	return keys
}

// rekeyAlbumImages moves the album's stored images from the old identity's
// key to the new one, so a manual cover survives the retag. It is called
// after a successful RetagAlbum and is optional (no hook, no error). Any
// failure is tolerated: the row moved and the image did not, which is today's
// behaviour and recoverable.
func (s *Scanner) rekeyAlbumImages(albumID uint, oldIdent, newIdent store.AlbumIdentity) {
	if s.cfg.AssetRekeyer == nil {
		return
	}
	oldKey := assetkey.Album(oldIdent.NameNorm, oldIdent.AlbumArtistNorm, oldIdent.MBReleaseID)
	newKey := assetkey.Album(newIdent.NameNorm, newIdent.AlbumArtistNorm, newIdent.MBReleaseID)
	// "album" is assetstore.KindAlbum, duplicated to keep this package free of an assetstore import.
	if err := s.cfg.AssetRekeyer.Rekey("album", oldKey, newKey); err != nil {
		slog.Warn("album image re-key failed; the row moved but the stored images did not",
			"album_id", albumID, "old_key", oldKey, "new_key", newKey, "err", err)
	}
}
