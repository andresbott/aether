package albumidentify

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

// durationToleranceSeconds bounds how far a file's measured length may sit from
// a tracklist entry's and still be considered the same track. Encoders, gapless
// rips and MusicBrainz itself disagree by a few seconds routinely; beyond this
// it is a different piece of music.
const durationToleranceSeconds = 12.0

// minFillScore is the confidence a candidate placement must reach to be
// proposed at all. Below it the file is reported as unplaced (SourceNone)
// rather than guessed at — a wrong inferred track number is worse than none,
// because the user may not notice it.
const minFillScore = 0.35

// minAccumulatedWeight is the minimum sum of weights that must participate in
// a placement decision. A bare track-number agreement (weight 0.1) is the
// weakest evidence and is exactly what a mis-ripped file gets wrong; at least
// one stronger signal (duration or title) must have participated for a
// placement to be proposed.
const minAccumulatedWeight = 0.3

// fillGaps assigns the files that no fingerprint placed on this release to its
// remaining tracklist slots, appending one assignment per input so every
// selected file has a row. Slots already claimed by a fingerprint match, or by
// an earlier (better-scoring) inference, are never reused.
//
// MatchedCount and MeanScore are deliberately left alone: they measure
// fingerprint evidence, and inferences must not inflate a release's ranking.
func fillGaps(o *AlbumOption, results []fileResult) {
	assigned := make(map[string]bool, len(o.Assignments))
	taken := make(map[string]bool, len(o.Assignments))
	for _, a := range o.Assignments {
		assigned[a.Path] = true
		taken[slotKey(a.DiscNumber, a.TrackNumber)] = true
	}

	// Score every (pending file, free slot) pair, then take them best-first so
	// the most confident placements claim their slots before weaker ones.
	type pairing struct {
		result fileResult
		slot   Slot
		score  float64
	}
	var pairs []pairing
	var pending []fileResult
	for _, res := range results {
		if assigned[res.input.Path] {
			continue
		}
		pending = append(pending, res)
		if res.err != nil {
			continue
		}
		for _, s := range o.Tracks {
			if score := fillScore(res, s); score >= minFillScore {
				pairs = append(pairs, pairing{result: res, slot: s, score: score})
			}
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].score > pairs[j].score })

	placed := make(map[string]Slot, len(pending))
	for _, p := range pairs {
		key := slotKey(p.slot.DiscNumber, p.slot.TrackNumber)
		if taken[key] {
			continue
		}
		if _, done := placed[p.result.input.Path]; done {
			continue
		}
		placed[p.result.input.Path] = p.slot
		taken[key] = true
	}

	for _, res := range pending {
		if s, ok := placed[res.input.Path]; ok {
			o.Assignments = append(o.Assignments, Assignment{
				Path:          res.input.Path,
				Source:        SourceInferred,
				Title:         s.Title,
				RecordingMBID: s.RecordingMBID,
				DiscNumber:    s.DiscNumber,
				TrackNumber:   s.TrackNumber,
			})
			continue
		}
		a := Assignment{Path: res.input.Path, Source: SourceNone}
		if res.err != nil {
			a.Error = res.err.Error()
		}
		o.Assignments = append(o.Assignments, a)
	}
}

func slotKey(disc, track int) string {
	return strconv.Itoa(disc) + "/" + strconv.Itoa(track)
}

// fillScore rates how well one file fits one tracklist slot, 0..1. Duration is
// the strongest signal (it is measured, not typed), title similarity next, and
// the file's current (disc, track) position last — it is often right for a clean
// rip and meaningless otherwise.
func fillScore(res fileResult, s Slot) float64 {
	var score, weight float64

	if res.duration > 0 && s.DurationSeconds > 0 {
		delta := math.Abs(res.duration - s.DurationSeconds)
		if delta > durationToleranceSeconds {
			// Definitively a different track: no title agreement can save it.
			return 0
		}
		score += 0.6 * (1 - delta/durationToleranceSeconds)
		weight += 0.6
	}

	fileTitle := normalizeText(res.input.CurrentTitle)
	slotTitle := normalizeText(s.Title)
	if fileTitle != "" && slotTitle != "" {
		score += 0.3 * titleSimilarity(fileTitle, slotTitle)
		weight += 0.3
	}

	// Track-number hint earns credit only when the disc also agrees.
	if res.input.CurrentTrackNumber > 0 {
		discAgrees := res.input.CurrentDiscNumber == 0 || res.input.CurrentDiscNumber == s.DiscNumber
		if discAgrees && res.input.CurrentTrackNumber == s.TrackNumber {
			score += 0.1
		}
		weight += 0.1
	}

	if weight < minAccumulatedWeight {
		return 0
	}
	// Normalise so a file missing a signal is judged on the ones it has.
	return score / weight
}

// titleSimilarity is a cheap 0..1 agreement between two already-normalized
// titles: 1 for an exact match, otherwise the share of the shorter title's
// words that appear in the longer one. Good enough to separate "In Bloom" from
// "Polly" without pulling in an edit-distance dependency.
func titleSimilarity(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	wa, wb := strings.Fields(a), strings.Fields(b)
	if len(wa) > len(wb) {
		wa, wb = wb, wa
	}
	set := make(map[string]bool, len(wb))
	for _, w := range wb {
		set[w] = true
	}
	var hits int
	for _, w := range wa {
		if set[w] {
			hits++
		}
	}
	return float64(hits) / float64(len(wa))
}

