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
// bare track number, so duplicates are deduplicated per disc.
//
// It is mass-weighted rather than averaged per disc: one deduped position for
// every slot the run would have to cross. An unweighted per-disc mean is
// bounded but invertible — a compilation matching exactly one file per disc
// scores 1.0 on every disc and so earns full credit for the very scatter this
// term exists to punish.
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
	byDisc := make(map[int][]int)
	for p := range positions {
		byDisc[p.disc] = append(byDisc[p.disc], p.track)
	}
	discs := make([]int, 0, len(byDisc))
	for d := range byDisc {
		discs = append(discs, d)
	}
	sort.Ints(discs)

	// Denominator: every position the run has to cross to include all matches.
	// Within a disc that is its span; entering a later disc it also includes the
	// tracks skipped before that disc's first match, because a run that starts
	// mid-disc is a disc being sampled — the compilation shape. The FIRST disc's
	// offset is deliberately not charged: a folder holding only tracks 10-12 of
	// an album is still one unbroken run, and charging it would tilt the ranking
	// toward whichever compilation happens to carry those songs at tracks 1-3.
	var span int
	for i, d := range discs {
		tracks := byDisc[d]
		sort.Ints(tracks)
		lo, hi := tracks[0], tracks[len(tracks)-1]
		span += hi - lo + 1
		if i > 0 {
			span += lo - 1
		}
	}
	if span <= 0 { // unreachable: every span is at least 1
		return 0
	}
	return float64(len(positions)) / float64(span)
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
	want := normalizeAlbumText(o.Album)
	if want == "" {
		return 0
	}
	var hits int
	for _, in := range inputs {
		if normalizeAlbumText(in.CurrentAlbum) == want {
			hits++
		}
	}
	return float64(hits) / float64(len(inputs))
}

// Two normalisers on purpose: album tags and track titles need opposite
// treatment of parenthetical annotations, and sharing one silently weakened the
// title comparison.

// normalizeAlbumText canonicalises an ALBUM name for the album-tag agreement
// term. Parenthesised content is DROPPED because on an album name it is almost
// always an edition marker — "Nevermind (Remastered)", "(Deluxe Edition)",
// "(Disc 2)" — and the user's existing tag naming a different pressing of the
// same album is agreement, not disagreement.
//
// Do not reuse this for titles: see normalizeTitleText.
func normalizeAlbumText(s string) string {
	var stripped strings.Builder
	depth := 0
	for _, r := range s {
		switch {
		case r == '(':
			depth++
		case r == ')':
			if depth > 0 {
				depth--
			}
		case depth == 0:
			stripped.WriteRune(r)
		}
	}
	return normalizeTitleText(stripped.String())
}

// normalizeTitleText canonicalises a TRACK title for gap-fill's title
// comparison. Case, whitespace and punctuation are normalised (common
// transcription variations), but parenthesised content is KEPT because on a
// track title it is the distinguishing part: "Song (Live)" and "Song (Remix)"
// are different recordings, and a release routinely carries both within the
// duration tolerance. Dropping it would collapse them to an identical title,
// leave the 0.3-weight title term with no separating power, and let gap-fill
// place the file on the wrong slot.
func normalizeTitleText(s string) string {
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
