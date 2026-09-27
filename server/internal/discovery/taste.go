package discovery

import (
	"math"
	"time"
)

// Taste-profile tuning. TasteHorizon exists to bound the query the store runs:
// at a 90-day half-life a 730-day-old play is worth ~0.4% of a fresh one, so
// cutting the scan there costs nothing measurable and keeps SQLite from walking
// millions of near-zero-weight rows.
const (
	TasteHalfLife = 90 * 24 * time.Hour
	TasteHorizon  = 730 * 24 * time.Hour
)

// GenrePlay is one play of a track that carries one genre. A track with three
// genres yields three GenrePlay values for the same play.
type GenrePlay struct {
	GenreID  uint
	PlayedAt time.Time
}

// GenreStar is one genre reached through a starred item. Stars carry no age:
// starring is a durable statement of taste, unlike a play.
type GenreStar struct {
	GenreID uint
}

// TasteProfile holds an L1-normalised genre id -> weight map, so Affinity is a
// fraction in [0,1] and the score's genre term needs no further scaling.
type TasteProfile struct {
	weights map[uint]float64
}

// Decay returns 0.5^(age/halfLife), clamped to 1 for non-positive ages so a
// future timestamp from clock skew cannot produce a weight above 1.
func Decay(age, halfLife time.Duration) float64 {
	if age <= 0 {
		return 1
	}
	return math.Pow(0.5, age.Seconds()/halfLife.Seconds())
}

// BuildTasteProfile weights each play by recency (TasteHalfLife) and each star
// at 1.0, then L1-normalises. Plays older than TasteHorizon are dropped, so the
// pure code agrees with the store's cutoff predicate.
func BuildTasteProfile(plays []GenrePlay, stars []GenreStar, now time.Time) TasteProfile {
	raw := make(map[uint]float64, len(plays)+len(stars))
	for _, p := range plays {
		age := now.Sub(p.PlayedAt)
		if age > TasteHorizon {
			continue
		}
		raw[p.GenreID] += Decay(age, TasteHalfLife)
	}
	for _, s := range stars {
		raw[s.GenreID] += 1
	}
	var total float64
	for _, w := range raw {
		total += w
	}
	if total == 0 {
		return TasteProfile{weights: map[uint]float64{}}
	}
	norm := make(map[uint]float64, len(raw))
	for id, w := range raw {
		norm[id] = w / total
	}
	return TasteProfile{weights: norm}
}

// Affinity is the mean profile weight across an item's genres. Genres absent
// from the profile contribute 0 rather than being skipped: an album that is half
// in a loved genre and half in an unknown one should not score as if it were
// wholly loved. An item with no genres scores 0.
func (p TasteProfile) Affinity(genreIDs []uint) float64 {
	if len(genreIDs) == 0 || len(p.weights) == 0 {
		return 0
	}
	var sum float64
	for _, id := range genreIDs {
		sum += p.weights[id]
	}
	return sum / float64(len(genreIDs))
}
