package discovery

import (
	"sort"
	"time"
)

// QuotaEvery is the interleave period for the rediscovery pool: every 4th slot
// (rank%4 == 3) is drawn from never-played candidates. Never-played items score
// zero on both play terms, so without a reserved slot they would be crowded out
// — and surfacing forgotten music is half the point of a discovery feed.
//
// Fixed positions rather than a random mix-in, because the merge has to be
// deterministic: page 2 must continue the sequence page 1 established.
const QuotaEvery = 4

// Ranked is one placed item: which entity, where it landed, and why.
type Ranked struct {
	Kind   Kind
	ID     uint
	Rank   int
	Reason Reason
}

// scored pairs a candidate with its computed terms so sorting can consult both.
type scored struct {
	cand  Candidate
	terms Terms
	score float64
}

// Rank scores every candidate, splits them into the main and rediscovery pools,
// interleaves them at QuotaEvery, and returns the [offset, offset+size) window
// with absolute ranks.
//
// The window is cut after the full interleave rather than during it, so the
// sequence is a pure function of (candidates, seed, now). A deeper offset
// gathers a wider candidate pool, which is a superset of the previous page's —
// it re-scores to the same values and therefore cannot reorder ranks already
// served.
func Rank(cands []Candidate, p TasteProfile, seed int64, now time.Time, offset, size int) []Ranked {
	var main, rediscover []scored
	for _, c := range cands {
		terms := ComputeTerms(c, p, seed, now)
		s := scored{cand: c, terms: terms, score: terms.Score()}
		if NeverPlayed(c) {
			// The rediscovery pool is ranked on taste and jitter alone: its
			// members are unplayed by definition, so the play terms carry no
			// signal and the added-recency term would just re-sort by import date.
			s.score = WeightGenreAffinity*terms.GenreAffinity + WeightJitter*terms.Jitter
			rediscover = append(rediscover, s)
		} else {
			main = append(main, s)
		}
	}
	sortScored(main)
	sortScored(rediscover)

	total := len(main) + len(rediscover)
	out := make([]Ranked, 0, size)
	var mi, ri int
	for rank := 0; rank < total; rank++ {
		pool := &main
		idx := &mi
		if rank%QuotaEvery == QuotaEvery-1 && ri < len(rediscover) {
			pool, idx = &rediscover, &ri
		}
		// Either pool may drain; the other fills the remaining slots.
		if *idx >= len(*pool) {
			if pool == &main {
				pool, idx = &rediscover, &ri
			} else {
				pool, idx = &main, &mi
			}
			if *idx >= len(*pool) {
				break
			}
		}
		picked := (*pool)[*idx]
		*idx++
		if rank < offset {
			continue
		}
		reason := picked.terms.Reason()
		if pool == &rediscover {
			reason = ReasonRediscover
		}
		out = append(out, Ranked{
			Kind:   picked.cand.Kind,
			ID:     picked.cand.ID,
			Rank:   rank,
			Reason: reason,
		})
		if len(out) == size {
			break
		}
	}
	return out
}

// sortScored orders by descending score, breaking ties by kind then id. The
// tiebreak is not cosmetic: without a total order, sort instability could
// reshuffle equal-scoring items between two requests and break the paging
// guarantee Rank documents.
func sortScored(s []scored) {
	sort.SliceStable(s, func(i, j int) bool {
		if s[i].score != s[j].score {
			return s[i].score > s[j].score
		}
		if s[i].cand.Kind != s[j].cand.Kind {
			return s[i].cand.Kind < s[j].cand.Kind
		}
		return s[i].cand.ID < s[j].cand.ID
	})
}
