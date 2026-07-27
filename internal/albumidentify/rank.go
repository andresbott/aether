package albumidentify

import (
	"sort"
	"strings"
)

// Ranking weights. Coverage is the heaviest term by design: this feature
// exists to find the release that explains the whole selection. For small
// selections (under ~8 files), covering one more file typically outranks any
// combination of the softer signals. For larger selections, a right-sized
// single-disc release can outrank a broader match by ~7 points (sizeFit 6 +
// singleDisc 1) plus score differences.
const (
	weightCoverage    = 100.0
	weightMeanScore   = 10.0
	weightContiguity  = 8.0
	weightSizeFit     = 6.0
	weightAlbumTag    = 5.0
	weightSingleDisc  = 1.0
)

// rankOptions sorts opts best-first, in place. It runs both before and after
// MusicBrainz enrichment; the size-fit term only participates for enriched
// options, so an un-enriched option is never punished for an unknown tracklist.
func rankOptions(opts []*AlbumOption, inputs []Input) {
	scores := make(map[string]float64, len(opts))
	years := make(map[string]int, len(opts))
	for _, o := range opts {
		scores[o.ReleaseMBID] = optionScore(o, inputs)
		years[o.ReleaseMBID] = o.Year
	}
	sort.SliceStable(opts, func(i, j int) bool {
		a, b := opts[i].ReleaseMBID, opts[j].ReleaseMBID
		if scores[a] != scores[b] {
			return scores[a] > scores[b]
		}
		// A release and its reissues score identically; prefer the original.
		// Year 0 means unknown and must not beat a real year.
		ya, yb := years[a], years[b]
		if ya != yb && ya != 0 && yb != 0 {
			return ya < yb
		}
		return ya != 0 && yb == 0
	})
}

// optionScore is the weighted heuristic sum for one option. Every term is
// normalised to 0..1 before weighting so the weights above are comparable.
func optionScore(o *AlbumOption, inputs []Input) float64 {
	total := len(inputs)
	if total == 0 {
		return 0
	}
	score := weightCoverage * float64(o.MatchedCount) / float64(total)
	score += weightMeanScore * o.MeanScore
	score += weightContiguity * contiguity(o)
	score += weightAlbumTag * albumTagAgreement(o, inputs)
	if o.Enriched {
		score += weightSizeFit * sizeFit(o.TrackCount, total)
		if o.DiscCount <= 1 {
			score += weightSingleDisc
		}
	}
	return score
}

// contiguity is the share of the matched track numbers that form one unbroken
// run — the signature of a folder holding a whole album rather than tracks
// scattered across a compilation. Unknown positions (0) are excluded because
// they carry no spatial information. A position is a (disc, track) pair, not a
// bare track number: duplicates are deduplicated per disc, and contiguity is
// computed per disc and averaged.
func contiguity(o *AlbumOption) float64 {
	type pos struct{ disc, track int }
	positions := make(map[pos]bool, len(o.Assignments))
	for _, a := range o.Assignments {
		if a.TrackNumber > 0 {
			positions[pos{disc: a.DiscNumber, track: a.TrackNumber}] = true
		}
	}
	// Zero positions: nothing is known, nothing is earned.
	if len(positions) == 0 {
		return 0
	}
	// One position: trivially an unbroken run, so neutral (1.0). Returning 0
	// would forfeit the full 8-point weight and penalise legitimately small
	// selections.
	if len(positions) == 1 {
		return 1.0
	}
	// Group positions by disc to compute contiguity per disc.
	byDisc := make(map[int][]int)
	for p := range positions {
		byDisc[p.disc] = append(byDisc[p.disc], p.track)
	}
	// Compute contiguity for each disc and average. This keeps the result 0..1
	// and avoids the overshoot when multiple discs repeat track numbers.
	var sum float64
	for _, tracks := range byDisc {
		if len(tracks) == 0 {
			continue
		}
		sort.Ints(tracks)
		span := tracks[len(tracks)-1] - tracks[0] + 1
		sum += float64(len(tracks)) / float64(span)
	}
	return sum / float64(len(byDisc))
}

// sizeFit rewards a release whose tracklist is about as long as the selection.
// A 40-track compilation that happens to contain all 11 selected songs is a
// worse answer than the 11-track album they came from.
func sizeFit(trackCount, selected int) float64 {
	if trackCount <= 0 || selected <= 0 {
		return 0
	}
	if trackCount < selected {
		// The selection cannot fit: strongly wrong.
		return 0
	}
	return float64(selected) / float64(trackCount)
}

// albumTagAgreement is the share of inputs whose current album tag already
// names this release — the user's existing tags are evidence, not noise.
func albumTagAgreement(o *AlbumOption, inputs []Input) float64 {
	want := normalizeText(o.Album)
	if want == "" {
		return 0
	}
	var hits int
	for _, in := range inputs {
		if normalizeText(in.CurrentAlbum) == want {
			hits++
		}
	}
	return float64(hits) / float64(len(inputs))
}

// normalizeText canonicalises text for comparison. Case, whitespace, and
// punctuation differences are normalised because they are common transcription
// variations, not evidence that the strings differ.
func normalizeText(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
