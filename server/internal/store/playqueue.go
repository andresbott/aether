package store

import (
	"errors"
	"time"

	"github.com/andresbott/aether/internal/model"
	"gorm.io/gorm"
)

// PlayQueueState is a saved queue with its tracks hydrated, ready for a handler
// to render as Subsonic Child objects.
type PlayQueueState struct {
	Tracks []model.Track
	// CurrentIndex indexes into Tracks, or -1 when no track is current.
	CurrentIndex int
	PositionMs   int64
	ChangedBy    string
	Changed      time.Time
}

// SavePlayQueue replaces the owner's saved queue wholesale. currentIndex is the
// 0-based slot of the playing track (-1 for none) and positionMs its playback
// offset. Callers pass changed explicitly so the timestamp is testable.
func (s *Store) SavePlayQueue(owner string, trackIDs []uint, currentIndex int, positionMs int64, changedBy string, changed time.Time) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var q model.PlayQueue
		err := tx.Where("owner = ?", owner).First(&q).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			q = model.PlayQueue{Owner: owner}
		case err != nil:
			return err
		}
		q.CurrentIndex = currentIndex
		q.PositionMs = positionMs
		q.ChangedBy = changedBy
		q.Changed = changed
		if err := tx.Save(&q).Error; err != nil {
			return err
		}
		// The whole entry set is rewritten: an incremental diff would have to
		// reconcile duplicate track ids across slots for no gain.
		if err := tx.Where("play_queue_id = ?", q.ID).Delete(&model.PlayQueueEntry{}).Error; err != nil {
			return err
		}
		for i, tid := range trackIDs {
			e := model.PlayQueueEntry{PlayQueueID: q.ID, SortOrder: i, TrackID: tid}
			if err := tx.Create(&e).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetPlayQueue returns the owner's saved queue, or nil when none is saved (not an
// error — an untouched account simply has no queue).
//
// Tracks deleted since the save are dropped from the queue and CurrentIndex
// follows the survivors. If the current track itself is gone, PositionMs resets
// to 0: an offset measured in one track is meaningless in another.
func (s *Store) GetPlayQueue(owner string) (*PlayQueueState, error) {
	var q model.PlayQueue
	err := s.db.Where("owner = ?", owner).First(&q).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var entries []model.PlayQueueEntry
	if err := s.db.Where("play_queue_id = ?", q.ID).Order("sort_order ASC").Find(&entries).Error; err != nil {
		return nil, err
	}

	state := &PlayQueueState{
		Tracks:       []model.Track{},
		CurrentIndex: -1,
		PositionMs:   q.PositionMs,
		ChangedBy:    q.ChangedBy,
		Changed:      q.Changed,
	}
	if len(entries) == 0 {
		return state, nil
	}

	ids := make([]uint, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.TrackID)
	}
	var tracks []model.Track
	if err := s.db.
		Preload("Album").
		Preload("Album.Artists").
		Preload("Artists").
		Preload("Genres").
		Where("id IN ?", ids).
		Find(&tracks).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]model.Track, len(tracks))
	for _, t := range tracks {
		byID[t.ID] = t
	}

	currentPresent := false
	// How many slots before the stored current one survived — the position the
	// track that took over the current slot ends up at.
	survivedBeforeCurrent := 0
	for _, e := range entries {
		t, ok := byID[e.TrackID]
		if !ok {
			continue
		}
		if e.SortOrder < q.CurrentIndex {
			survivedBeforeCurrent++
		}
		// Match on the STORED slot, not this row's position in the loaded slice:
		// orphan cleanup deletes entry rows outright, so the survivors keep their
		// original sort_order and the numbering has gaps. Using the slice index
		// here would nominate the wrong track as current and discard its position.
		if e.SortOrder == q.CurrentIndex {
			state.CurrentIndex = len(state.Tracks)
			currentPresent = true
		}
		state.Tracks = append(state.Tracks, t)
	}
	if !currentPresent {
		state.PositionMs = 0
		// The queue still exists, so point at the track that took over the current
		// slot rather than reporting "no current track" for a non-empty queue.
		// Note this counts SURVIVORS: the stored index is a pre-deletion slot
		// number and clamping it would skip past the replacement.
		if len(state.Tracks) > 0 && q.CurrentIndex >= 0 {
			state.CurrentIndex = min(survivedBeforeCurrent, len(state.Tracks)-1)
		}
	}
	return state, nil
}

// ClearPlayQueue removes the owner's saved queue. Clearing an absent queue is a
// no-op: savePlayQueue with no ids is the spec's clear call and may arrive at any
// time.
func (s *Store) ClearPlayQueue(owner string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var q model.PlayQueue
		err := tx.Where("owner = ?", owner).First(&q).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := tx.Where("play_queue_id = ?", q.ID).Delete(&model.PlayQueueEntry{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.PlayQueue{}, q.ID).Error
	})
}
