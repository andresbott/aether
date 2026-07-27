package albumidentify

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/andresbott/aether/internal/artistimage"
	"github.com/andresbott/aether/internal/upstream"
	"github.com/andresbott/aether/libs/acoustid"
)

// errFake stands in for a fingerprint failure in table tests.
var errFake = errors.New("fingerprint failed")

// rec builds one fingerprint candidate placing a recording on the given
// releases.
func rec(score float64, mbid, title string, rels ...acoustid.Release) acoustid.Recording {
	return acoustid.Recording{
		Score:   score,
		MBID:    mbid,
		Title:   title,
		Artists: []acoustid.ArtistCredit{{MBID: "art-1", Name: "Artist"}},
		Release: rels,
	}
}

func rel(mbid, album string, year, disc, track int) acoustid.Release {
	return acoustid.Release{
		MBID:             mbid,
		ReleaseGroupMBID: "rg-" + mbid,
		Title:            album,
		Year:             year,
		DiscNumber:       disc,
		TrackNumber:      track,
	}
}

func TestUnionCollapsesOneReleaseAcrossSongs(t *testing.T) {
	results := []fileResult{
		{
			input:      Input{Path: "01.flac", AbsPath: "/lib/01.flac"},
			recordings: []acoustid.Recording{rec(0.9, "rec-1", "One", rel("rel-A", "Album A", 1991, 1, 1))},
			duration:   180,
		},
		{
			input:      Input{Path: "02.flac", AbsPath: "/lib/02.flac"},
			recordings: []acoustid.Recording{rec(0.8, "rec-2", "Two", rel("rel-A", "Album A", 1991, 1, 2))},
			duration:   200,
		},
	}

	opts := unionReleases(results)
	if len(opts) != 1 {
		t.Fatalf("expected 1 option, got %d", len(opts))
	}
	o := opts[0]
	if o.ReleaseMBID != "rel-A" || o.Album != "Album A" || o.Year != 1991 {
		t.Fatalf("unexpected option: %+v", o)
	}
	if o.ReleaseGroupMBID != "rg-rel-A" {
		t.Fatalf("expected release group carried over, got %q", o.ReleaseGroupMBID)
	}
	if o.MatchedCount != 2 {
		t.Fatalf("expected 2 matched songs, got %d", o.MatchedCount)
	}
	if len(o.Assignments) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(o.Assignments))
	}
	a := o.Assignments[1]
	if a.Path != "02.flac" || a.Source != SourceFingerprint || a.Title != "Two" ||
		a.RecordingMBID != "rec-2" || a.TrackNumber != 2 || a.DiscNumber != 1 {
		t.Fatalf("unexpected assignment: %+v", a)
	}
	if len(a.Artists) != 1 || a.Artists[0].Name != "Artist" || a.Artists[0].MBID != "art-1" {
		t.Fatalf("unexpected assignment artists: %+v", a.Artists)
	}
}

func TestUnionSplitsDistinctReleasesAndKeepsBestCandidatePerSong(t *testing.T) {
	results := []fileResult{
		{
			input: Input{Path: "01.flac"},
			recordings: []acoustid.Recording{
				rec(0.9, "rec-1", "One", rel("rel-A", "Album A", 1991, 1, 1)),
				// Same song, weaker candidate, also on release A: the stronger
				// one must win rather than producing two assignments.
				rec(0.4, "rec-1b", "One (alt)", rel("rel-A", "Album A", 1991, 1, 7)),
				rec(0.5, "rec-1c", "One (live)", rel("rel-B", "Live", 2005, 1, 4)),
			},
		},
	}

	opts := unionReleases(results)
	if len(opts) != 2 {
		t.Fatalf("expected 2 options, got %d", len(opts))
	}
	byMBID := map[string]*AlbumOption{}
	for _, o := range opts {
		byMBID[o.ReleaseMBID] = o
	}
	a := byMBID["rel-A"]
	if a == nil || len(a.Assignments) != 1 {
		t.Fatalf("expected one assignment on rel-A, got %+v", a)
	}
	if a.Assignments[0].RecordingMBID != "rec-1" || a.Assignments[0].TrackNumber != 1 {
		t.Fatalf("expected the stronger candidate to win, got %+v", a.Assignments[0])
	}
	if a.MeanScore != 0.9 {
		t.Fatalf("expected mean score 0.9, got %v", a.MeanScore)
	}
	if byMBID["rel-B"] == nil || byMBID["rel-B"].MeanScore != 0.5 {
		t.Fatalf("unexpected rel-B option: %+v", byMBID["rel-B"])
	}
}

func TestUnionIgnoresFailedAndUnmatchedFiles(t *testing.T) {
	results := []fileResult{
		{input: Input{Path: "bad.flac"}, err: errFake},
		{input: Input{Path: "none.flac"}},
		{
			input:      Input{Path: "01.flac"},
			recordings: []acoustid.Recording{rec(0.9, "rec-1", "One", rel("rel-A", "Album A", 1991, 1, 1))},
		},
	}

	opts := unionReleases(results)
	if len(opts) != 1 || opts[0].MatchedCount != 1 {
		t.Fatalf("unexpected options: %+v", opts)
	}
	if len(opts[0].Assignments) != 1 {
		t.Fatalf("expected only the matched file to be assigned, got %d", len(opts[0].Assignments))
	}
}

func TestUnionSkipsReleasesWithoutMBID(t *testing.T) {
	results := []fileResult{{
		input:      Input{Path: "01.flac"},
		recordings: []acoustid.Recording{rec(0.9, "rec-1", "One", rel("", "No MBID", 1991, 1, 1))},
	}}
	if opts := unionReleases(results); len(opts) != 0 {
		t.Fatalf("expected no options, got %+v", opts)
	}
}

func TestUnionDeterministicOrderingWhenFileMatchesMultipleReleases(t *testing.T) {
	// One file matching several releases must produce a stable order.
	// Without sorting the release MBIDs, map iteration would randomize this.
	results := []fileResult{
		{
			input: Input{Path: "track.flac"},
			recordings: []acoustid.Recording{
				rec(0.8, "rec-1", "Song",
					rel("rel-E", "Release E", 2005, 1, 1),
					rel("rel-A", "Release A", 2001, 1, 1),
					rel("rel-D", "Release D", 2004, 1, 1),
					rel("rel-C", "Release C", 2003, 1, 1),
					rel("rel-B", "Release B", 2002, 1, 1),
				),
			},
		},
	}

	// Run the union multiple times to catch nondeterminism (though Go's map
	// iteration is already randomized, so even one run is usually enough).
	for i := 0; i < 10; i++ {
		opts := unionReleases(results)
		if len(opts) != 5 {
			t.Fatalf("run %d: expected 5 options, got %d", i, len(opts))
		}
		// Expect alphabetical order by release MBID.
		expected := []string{"rel-A", "rel-B", "rel-C", "rel-D", "rel-E"}
		for j, mbid := range expected {
			if opts[j].ReleaseMBID != mbid {
				t.Fatalf("run %d: expected opts[%d].ReleaseMBID=%q, got %q", i, j, mbid, opts[j].ReleaseMBID)
			}
		}
	}
}

// opt builds an option with explicit ranking inputs; assignments are synthetic
// positions, one per matched song.
func opt(mbid, album string, year int, positions []int, mean float64) *AlbumOption {
	o := &AlbumOption{ReleaseMBID: mbid, Album: album, Year: year, MeanScore: mean}
	for i, p := range positions {
		o.Assignments = append(o.Assignments, Assignment{
			Path:        string(rune('a'+i)) + ".flac",
			Source:      SourceFingerprint,
			TrackNumber: p,
			Score:       mean,
		})
	}
	o.MatchedCount = len(positions)
	return o
}

func mbids(opts []*AlbumOption) []string {
	out := make([]string, 0, len(opts))
	for _, o := range opts {
		out = append(out, o.ReleaseMBID)
	}
	return out
}

func TestRankCoverageBeatsScore(t *testing.T) {
	inputs := []Input{{Path: "a.flac"}, {Path: "b.flac"}, {Path: "c.flac"}}
	opts := []*AlbumOption{
		opt("weak-but-broad", "Album", 1991, []int{1, 2, 3}, 0.55),
		opt("strong-but-narrow", "Album", 1991, []int{1}, 0.99),
	}
	rankOptions(opts, inputs)
	if got := mbids(opts); got[0] != "weak-but-broad" {
		t.Fatalf("expected coverage to win, got %v", got)
	}
}

func TestRankContiguityBreaksCoverageTie(t *testing.T) {
	inputs := []Input{{Path: "a.flac"}, {Path: "b.flac"}, {Path: "c.flac"}}
	opts := []*AlbumOption{
		opt("scattered", "Compilation", 1991, []int{2, 9, 17}, 0.8),
		opt("contiguous", "Album", 1991, []int{1, 2, 3}, 0.8),
	}
	rankOptions(opts, inputs)
	if got := mbids(opts); got[0] != "contiguous" {
		t.Fatalf("expected the contiguous run to win, got %v", got)
	}
}

func TestRankCurrentAlbumTagBreaksTie(t *testing.T) {
	inputs := []Input{
		{Path: "a.flac", CurrentAlbum: "Nevermind"},
		{Path: "b.flac", CurrentAlbum: "Nevermind"},
	}
	opts := []*AlbumOption{
		opt("other", "Best Of Nirvana", 1991, []int{1, 2}, 0.8),
		opt("tagged", "Nevermind", 1991, []int{1, 2}, 0.8),
	}
	rankOptions(opts, inputs)
	if got := mbids(opts); got[0] != "tagged" {
		t.Fatalf("expected the option matching the current album tag to win, got %v", got)
	}
}

// Pre-fix, albumTagAgreement used a weaker normaliser than gap-fill, so
// "Nevermind (Remastered)" vs "Nevermind" disagreed despite being the same album.
func TestRankAlbumTagAgreesToleratesPunctuation(t *testing.T) {
	inputs := []Input{
		{Path: "a.flac", CurrentAlbum: "Nevermind (Remastered)"},
		{Path: "b.flac", CurrentAlbum: "Nevermind (Remastered)"},
	}
	opts := []*AlbumOption{
		opt("other", "Best Of", 1991, []int{1, 2}, 0.8),
		opt("tagged", "Nevermind", 1991, []int{1, 2}, 0.8),
	}
	rankOptions(opts, inputs)
	if got := mbids(opts); got[0] != "tagged" {
		t.Fatalf("expected the option matching the current album tag (ignoring punctuation) to win, got %v", got)
	}
}

func TestRankTracklistSizeFavoursTheAlbumOverTheCompilation(t *testing.T) {
	inputs := []Input{{Path: "a.flac"}, {Path: "b.flac"}, {Path: "c.flac"}}
	album := opt("album", "Album", 1991, []int{1, 2, 3}, 0.8)
	album.Enriched, album.TrackCount, album.DiscCount = true, 3, 1
	comp := opt("comp", "Mega Hits", 1991, []int{1, 2, 3}, 0.8)
	comp.Enriched, comp.TrackCount, comp.DiscCount = true, 40, 1

	opts := []*AlbumOption{comp, album}
	rankOptions(opts, inputs)
	if got := mbids(opts); got[0] != "album" {
		t.Fatalf("expected the same-size release to win, got %v", got)
	}
}

func TestRankEarliestYearIsTheFinalTiebreak(t *testing.T) {
	inputs := []Input{{Path: "a.flac"}}
	opts := []*AlbumOption{
		opt("reissue", "Album", 2011, []int{1}, 0.8),
		opt("original", "Album", 1991, []int{1}, 0.8),
	}
	rankOptions(opts, inputs)
	if got := mbids(opts); got[0] != "original" {
		t.Fatalf("expected the earliest year to win, got %v", got)
	}
}

// Equal options must not reorder run to run: the dialog preselects rank 1 and a
// flapping order would silently change what the user stages.
func TestRankIsStableForEqualOptions(t *testing.T) {
	inputs := []Input{{Path: "a.flac"}}
	build := func() []*AlbumOption {
		return []*AlbumOption{
			opt("first", "Album", 1991, []int{1}, 0.8),
			opt("second", "Album", 1991, []int{1}, 0.8),
		}
	}
	opts := build()
	rankOptions(opts, inputs)
	want := mbids(opts)
	for i := 0; i < 5; i++ {
		again := build()
		rankOptions(again, inputs)
		if got := mbids(again); got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("unstable order: %v then %v", want, got)
		}
	}
}

func TestRankKnownYearBeatsUnknown(t *testing.T) {
	inputs := []Input{{Path: "a.flac"}}
	opts := []*AlbumOption{
		opt("unknown-year", "Album", 0, []int{1}, 0.8),
		opt("known-year", "Album", 1991, []int{1}, 0.8),
	}
	rankOptions(opts, inputs)
	if got := mbids(opts); got[0] != "known-year" {
		t.Fatalf("expected known year to win over unknown, got %v", got)
	}
}

func TestRankContiguityHandlesZeroAndSinglePosition(t *testing.T) {
	// Zero known positions: nothing is known, so contiguity should be 0.
	optZero := opt("zero", "Album", 1991, []int{0, 0}, 0.8)
	if got := contiguity(optZero); got != 0 {
		t.Fatalf("expected contiguity=0 for zero known positions, got %v", got)
	}

	// One known position: trivially an unbroken run, so neutral 1.0.
	optOne := opt("one", "Album", 1991, []int{1}, 0.8)
	if got := contiguity(optOne); got != 1.0 {
		t.Fatalf("expected contiguity=1.0 for single position, got %v", got)
	}
}

// Pre-fix, multi-disc albums with repeated track numbers across discs had
// contiguity >1.0, inverting the signal.
func TestRankContiguityStaysNormalizedForMultiDisc(t *testing.T) {
	o := &AlbumOption{
		ReleaseMBID: "multi-disc",
		Assignments: []Assignment{
			{Path: "d1-t1.flac", DiscNumber: 1, TrackNumber: 1, Source: SourceFingerprint},
			{Path: "d1-t2.flac", DiscNumber: 1, TrackNumber: 2, Source: SourceFingerprint},
			{Path: "d1-t3.flac", DiscNumber: 1, TrackNumber: 3, Source: SourceFingerprint},
			{Path: "d2-t1.flac", DiscNumber: 2, TrackNumber: 1, Source: SourceFingerprint},
			{Path: "d2-t2.flac", DiscNumber: 2, TrackNumber: 2, Source: SourceFingerprint},
			{Path: "d2-t3.flac", DiscNumber: 2, TrackNumber: 3, Source: SourceFingerprint},
		},
	}
	got := contiguity(o)
	if got < 0 || got > 1.0 {
		t.Fatalf("contiguity must stay 0..1, got %v", got)
	}
	// Both discs are contiguous 1-2-3: average is 1.0.
	if got != 1.0 {
		t.Fatalf("expected contiguity=1.0 for contiguous multi-disc, got %v", got)
	}
}

// Pre-fix, two files on the same track number (mis-rip, re-encode, etc.) also
// produced contiguity >1.0.
func TestRankContiguityDeduplicatesPositions(t *testing.T) {
	o := &AlbumOption{
		ReleaseMBID: "dup",
		Assignments: []Assignment{
			{Path: "a.flac", DiscNumber: 1, TrackNumber: 1, Source: SourceFingerprint},
			{Path: "b.flac", DiscNumber: 1, TrackNumber: 1, Source: SourceFingerprint},
			{Path: "c.flac", DiscNumber: 1, TrackNumber: 2, Source: SourceFingerprint},
		},
	}
	got := contiguity(o)
	if got < 0 || got > 1.0 {
		t.Fatalf("contiguity must stay 0..1, got %v", got)
	}
	// Deduplicated: positions 1 and 2, contiguous.
	if got != 1.0 {
		t.Fatalf("expected contiguity=1.0 after dedup, got %v", got)
	}
}

// Verify that a fully-covering single-disc option outranks a partial multi-disc
// option after the contiguity overshoot is fixed.
func TestRankFullCoverageWinsOverMultiDiscPartialWithContiguityFixed(t *testing.T) {
	inputs := []Input{
		{Path: "01.flac"}, {Path: "02.flac"}, {Path: "03.flac"}, {Path: "04.flac"},
		{Path: "05.flac"}, {Path: "06.flac"}, {Path: "07.flac"}, {Path: "08.flac"},
		{Path: "09.flac"}, {Path: "10.flac"}, {Path: "11.flac"}, {Path: "12.flac"},
	}
	// Full coverage: single disc, all 12 files matched.
	fullCoverage := &AlbumOption{
		ReleaseMBID: "full",
		Assignments: []Assignment{
			{Path: "01.flac", DiscNumber: 1, TrackNumber: 1, Source: SourceFingerprint, Score: 0.8},
			{Path: "02.flac", DiscNumber: 1, TrackNumber: 2, Source: SourceFingerprint, Score: 0.8},
			{Path: "03.flac", DiscNumber: 1, TrackNumber: 3, Source: SourceFingerprint, Score: 0.8},
			{Path: "04.flac", DiscNumber: 1, TrackNumber: 4, Source: SourceFingerprint, Score: 0.8},
			{Path: "05.flac", DiscNumber: 1, TrackNumber: 5, Source: SourceFingerprint, Score: 0.8},
			{Path: "06.flac", DiscNumber: 1, TrackNumber: 6, Source: SourceFingerprint, Score: 0.8},
			{Path: "07.flac", DiscNumber: 1, TrackNumber: 7, Source: SourceFingerprint, Score: 0.8},
			{Path: "08.flac", DiscNumber: 1, TrackNumber: 8, Source: SourceFingerprint, Score: 0.8},
			{Path: "09.flac", DiscNumber: 1, TrackNumber: 9, Source: SourceFingerprint, Score: 0.8},
			{Path: "10.flac", DiscNumber: 1, TrackNumber: 10, Source: SourceFingerprint, Score: 0.8},
			{Path: "11.flac", DiscNumber: 1, TrackNumber: 11, Source: SourceFingerprint, Score: 0.8},
			{Path: "12.flac", DiscNumber: 1, TrackNumber: 12, Source: SourceFingerprint, Score: 0.8},
		},
		MatchedCount: 12,
		MeanScore:    0.8,
	}
	// Partial multi-disc: 11 of 12, but on two discs with repeated track numbers.
	// Pre-fix, the repeated tracks gave contiguity=2.0 → 16 points, outranking
	// the full-coverage option purely on the overshoot.
	partialMulti := &AlbumOption{
		ReleaseMBID: "partial",
		Assignments: []Assignment{
			{Path: "01.flac", DiscNumber: 1, TrackNumber: 1, Source: SourceFingerprint, Score: 0.8},
			{Path: "02.flac", DiscNumber: 1, TrackNumber: 2, Source: SourceFingerprint, Score: 0.8},
			{Path: "03.flac", DiscNumber: 1, TrackNumber: 3, Source: SourceFingerprint, Score: 0.8},
			{Path: "04.flac", DiscNumber: 1, TrackNumber: 4, Source: SourceFingerprint, Score: 0.8},
			{Path: "05.flac", DiscNumber: 1, TrackNumber: 5, Source: SourceFingerprint, Score: 0.8},
			{Path: "06.flac", DiscNumber: 1, TrackNumber: 6, Source: SourceFingerprint, Score: 0.8},
			{Path: "07.flac", DiscNumber: 2, TrackNumber: 1, Source: SourceFingerprint, Score: 0.8},
			{Path: "08.flac", DiscNumber: 2, TrackNumber: 2, Source: SourceFingerprint, Score: 0.8},
			{Path: "09.flac", DiscNumber: 2, TrackNumber: 3, Source: SourceFingerprint, Score: 0.8},
			{Path: "10.flac", DiscNumber: 2, TrackNumber: 4, Source: SourceFingerprint, Score: 0.8},
			{Path: "11.flac", DiscNumber: 2, TrackNumber: 5, Source: SourceFingerprint, Score: 0.8},
		},
		MatchedCount: 11,
		MeanScore:    0.8,
	}
	opts := []*AlbumOption{partialMulti, fullCoverage}
	rankOptions(opts, inputs)
	if opts[0].ReleaseMBID != "full" {
		t.Fatalf("expected full-coverage to rank first, got %v", mbids(opts))
	}
}

// A multi-disc compilation matching exactly one file per disc is the scatter
// this term exists to punish. Pre-fix, contiguity averaged the per-disc values
// (each a lone position = 1.0) and handed it the full 8-point weight, tying a
// perfectly contiguous full-album match.
func TestRankContiguityDeniesCreditToOneFilePerDisc(t *testing.T) {
	o := &AlbumOption{
		ReleaseMBID: "box-set",
		Assignments: []Assignment{
			{Path: "a.flac", DiscNumber: 1, TrackNumber: 4, Source: SourceFingerprint},
			{Path: "b.flac", DiscNumber: 2, TrackNumber: 7, Source: SourceFingerprint},
			{Path: "c.flac", DiscNumber: 3, TrackNumber: 2, Source: SourceFingerprint},
			{Path: "d.flac", DiscNumber: 4, TrackNumber: 9, Source: SourceFingerprint},
			{Path: "e.flac", DiscNumber: 5, TrackNumber: 1, Source: SourceFingerprint},
		},
		MatchedCount: 5,
	}
	got := contiguity(o)
	if got > 0.6 {
		t.Fatalf("one file per disc must not earn near-full contiguity credit, got %v", got)
	}
	// A contiguous single-disc run with the same coverage must still outrank it.
	inputs := []Input{{Path: "a.flac"}, {Path: "b.flac"}, {Path: "c.flac"}, {Path: "d.flac"}, {Path: "e.flac"}}
	album := opt("album", "Album", 1991, []int{1, 2, 3, 4, 5}, 0.8)
	o.MeanScore = 0.8
	opts := []*AlbumOption{o, album}
	rankOptions(opts, inputs)
	if opts[0].ReleaseMBID != "album" {
		t.Fatalf("expected the contiguous album to outrank the one-per-disc box set, got %v", mbids(opts))
	}
}

// Adding a scattered file on a fresh disc must not multiply the credit the same
// scatter earns on one disc: pre-fix the unweighted per-disc mean turned
// {1,5,9} (0.333) plus one stray into 0.667.
func TestRankContiguityIsMonotoneInScatter(t *testing.T) {
	scattered := &AlbumOption{
		ReleaseMBID: "one-disc",
		Assignments: []Assignment{
			{Path: "a.flac", DiscNumber: 1, TrackNumber: 1, Source: SourceFingerprint},
			{Path: "b.flac", DiscNumber: 1, TrackNumber: 5, Source: SourceFingerprint},
			{Path: "c.flac", DiscNumber: 1, TrackNumber: 9, Source: SourceFingerprint},
		},
	}
	plusStray := &AlbumOption{
		ReleaseMBID: "two-disc",
		Assignments: append(append([]Assignment{}, scattered.Assignments...),
			Assignment{Path: "d.flac", DiscNumber: 2, TrackNumber: 4, Source: SourceFingerprint}),
	}
	base, withStray := contiguity(scattered), contiguity(plusStray)
	if withStray > base*1.5 {
		t.Fatalf("a stray file on a second disc inflated contiguity from %v to %v", base, withStray)
	}
}

func slot(disc, track int, title string, dur float64, recMBID string) Slot {
	return Slot{
		DiscNumber: disc, TrackNumber: track, Title: title,
		DurationSeconds: dur, RecordingMBID: recMBID,
	}
}

func assignmentFor(o *AlbumOption, path string) *Assignment {
	for i := range o.Assignments {
		if o.Assignments[i].Path == path {
			return &o.Assignments[i]
		}
	}
	return nil
}

func TestFillGapsPlacesUnmatchedByDurationAndTitle(t *testing.T) {
	o := &AlbumOption{
		ReleaseMBID: "rel-A", Album: "Album A", Enriched: true, TrackCount: 3, DiscCount: 1,
		Tracks: []Slot{
			slot(1, 1, "One", 180, "rec-1"),
			slot(1, 2, "Two", 200, "rec-2"),
			slot(1, 3, "Three", 300, "rec-3"),
		},
		Assignments: []Assignment{{
			Path: "01.flac", Source: SourceFingerprint, Title: "One",
			RecordingMBID: "rec-1", DiscNumber: 1, TrackNumber: 1, Score: 0.9,
		}},
		MatchedCount: 1,
	}
	results := []fileResult{
		{input: Input{Path: "01.flac"}, duration: 180},
		// No fingerprint match, but its duration and title point at track 3.
		{input: Input{Path: "03.flac", CurrentTitle: "Three"}, duration: 299},
	}

	fillGaps(o, results)

	a := assignmentFor(o, "03.flac")
	if a == nil {
		t.Fatal("expected an assignment for 03.flac")
	}
	if a.Source != SourceInferred {
		t.Fatalf("expected an inferred source, got %q", a.Source)
	}
	if a.TrackNumber != 3 || a.DiscNumber != 1 || a.Title != "Three" || a.RecordingMBID != "rec-3" {
		t.Fatalf("unexpected inferred assignment: %+v", a)
	}
	// Coverage counts fingerprint matches only: an inference is not evidence.
	if o.MatchedCount != 1 {
		t.Fatalf("expected MatchedCount to stay 1, got %d", o.MatchedCount)
	}
}

func TestFillGapsNeverReusesASlot(t *testing.T) {
	o := &AlbumOption{
		ReleaseMBID: "rel-A", Enriched: true, TrackCount: 2, DiscCount: 1,
		Tracks: []Slot{
			slot(1, 1, "One", 200, "rec-1"),
			slot(1, 2, "Two", 220, "rec-2"),
		},
	}
	// File a has a very close duration match to slot 1 (delta 1s); file b has a
	// weaker match to slot 1 (delta 19s) but a perfect match to slot 2. The
	// best-first ordering must let a claim slot 1, forcing b to take slot 2.
	results := []fileResult{
		{input: Input{Path: "a.flac"}, duration: 201},
		{input: Input{Path: "b.flac"}, duration: 220},
	}

	fillGaps(o, results)

	a, b := assignmentFor(o, "a.flac"), assignmentFor(o, "b.flac")
	if a == nil || b == nil {
		t.Fatalf("expected both files assigned, got %+v", o.Assignments)
	}
	if a.TrackNumber != 1 || b.TrackNumber != 2 {
		t.Fatalf("expected best-first: a→1, b→2, got a→%d, b→%d", a.TrackNumber, b.TrackNumber)
	}
}

func TestFillGapsSkipsSlotsTakenByFingerprintMatches(t *testing.T) {
	o := &AlbumOption{
		ReleaseMBID: "rel-A", Enriched: true, TrackCount: 2, DiscCount: 1,
		Tracks: []Slot{
			slot(1, 1, "One", 200, "rec-1"),
			slot(1, 2, "Two", 205, "rec-2"),
		},
		Assignments: []Assignment{{
			Path: "01.flac", Source: SourceFingerprint, DiscNumber: 1, TrackNumber: 1,
		}},
		MatchedCount: 1,
	}
	// Duration fits slot 1 best, but the fingerprint already owns it, so the
	// file must fall through to the next plausible slot.
	results := []fileResult{
		{input: Input{Path: "01.flac"}, duration: 200},
		{input: Input{Path: "x.flac"}, duration: 200},
	}

	fillGaps(o, results)

	if a := assignmentFor(o, "x.flac"); a == nil || a.TrackNumber != 2 {
		t.Fatalf("expected the free slot 2, got %+v", a)
	}
}

func TestFillGapsRejectsBareTrackNumberAgreement(t *testing.T) {
	o := &AlbumOption{
		ReleaseMBID: "rel-A", Enriched: true, TrackCount: 2, DiscCount: 1,
		Tracks: []Slot{
			slot(1, 1, "Alpha", 200, "rec-1"),
			slot(1, 2, "Beta", 205, "rec-2"),
		},
	}
	results := []fileResult{
		// This file has a matching track number but no duration and no title:
		// a bare track-number agreement is too weak to justify placement.
		{input: Input{Path: "weak.flac", CurrentTrackNumber: 1}},
		// This file has a good duration match: it should be placed.
		{input: Input{Path: "strong.flac"}, duration: 204},
	}

	fillGaps(o, results)

	weak := assignmentFor(o, "weak.flac")
	if weak == nil || weak.Source != SourceNone {
		t.Fatalf("expected bare track-number file to be SourceNone, got %+v", weak)
	}
	strong := assignmentFor(o, "strong.flac")
	if strong == nil || strong.Source != SourceInferred || strong.TrackNumber != 2 {
		t.Fatalf("expected duration-matched file to be placed, got %+v", strong)
	}
}

func TestFillGapsUsesCurrentTrackNumberWhenNothingElseSeparates(t *testing.T) {
	o := &AlbumOption{
		ReleaseMBID: "rel-A", Enriched: true, TrackCount: 2, DiscCount: 1,
		// Identical durations and unrelated titles: two files compete, each with
		// a matching track number plus duration evidence.
		Tracks: []Slot{
			slot(1, 1, "Alpha", 200, "rec-1"),
			slot(1, 2, "Beta", 200, "rec-2"),
		},
	}
	results := []fileResult{
		{input: Input{Path: "a.flac", CurrentTrackNumber: 1}, duration: 200},
		{input: Input{Path: "b.flac", CurrentTrackNumber: 2}, duration: 200},
	}

	fillGaps(o, results)

	a := assignmentFor(o, "a.flac")
	if a == nil || a.TrackNumber != 1 {
		t.Fatalf("expected track 1 from the current tag, got %+v", a)
	}
	b := assignmentFor(o, "b.flac")
	if b == nil || b.TrackNumber != 2 {
		t.Fatalf("expected track 2 from the current tag, got %+v", b)
	}
}

// Pre-fix, gap-fill's track-number hint was disc-blind, placing disc-2 files
// on disc-1 positions with the same track number.
func TestFillGapsRequiresDiscAgreementForTrackNumberHint(t *testing.T) {
	o := &AlbumOption{
		ReleaseMBID: "rel-A", Enriched: true, TrackCount: 6, DiscCount: 2,
		Tracks: []Slot{
			slot(1, 1, "D1T1", 180, "rec-d1-1"),
			slot(1, 2, "D1T2", 185, "rec-d1-2"),
			slot(1, 3, "D1T3", 190, "rec-d1-3"),
			slot(2, 1, "D2T1", 195, "rec-d2-1"),
			slot(2, 2, "D2T2", 200, "rec-d2-2"),
			slot(2, 3, "D2T3", 205, "rec-d2-3"),
		},
	}
	// Two files: disc 1 track 3, and disc 2 track 3. Both durations within
	// tolerance of multiple slots, so the track-number+disc-number hint must
	// separate them.
	results := []fileResult{
		{input: Input{Path: "d1-t3.flac", CurrentTrackNumber: 3, CurrentDiscNumber: 1}, duration: 191},
		{input: Input{Path: "d2-t3.flac", CurrentTrackNumber: 3, CurrentDiscNumber: 2}, duration: 206},
	}

	fillGaps(o, results)

	d1 := assignmentFor(o, "d1-t3.flac")
	if d1 == nil || d1.DiscNumber != 1 || d1.TrackNumber != 3 {
		t.Fatalf("expected disc 1 track 3, got %+v", d1)
	}
	d2 := assignmentFor(o, "d2-t3.flac")
	if d2 == nil || d2.DiscNumber != 2 || d2.TrackNumber != 3 {
		t.Fatalf("expected disc 2 track 3, got %+v", d2)
	}
}

// Pre-fix, gap-fill normalised titles with the album normaliser, which deletes
// parenthesised content: "Song (Live)" and "Song (Remix)" both became "song" and
// scored a perfect 1.0, so the title term provided no separation at all.
func TestFillTitleSimilaritySeparatesParentheticalVariants(t *testing.T) {
	live := normalizeTitleText("Song (Live)")
	remix := normalizeTitleText("Song (Remix)")
	if live == remix {
		t.Fatalf("parenthetical variants must not normalise identically, both became %q", live)
	}
	if got := titleSimilarity(live, remix); got >= 1.0 {
		t.Fatalf("expected the variants to score below 1.0, got %v", got)
	}
	// A wholly parenthetical title must keep some text, or the title term's
	// weight silently drops out of the placement decision.
	if got := normalizeTitleText("(Untitled)"); got == "" {
		t.Fatalf("a wholly parenthetical title must not normalise to empty")
	}
}

// The same separation must survive the real placement path: a studio and a live
// take within the duration tolerance must be told apart by title.
func TestFillGapsPlacesTheRightParentheticalVariant(t *testing.T) {
	o := &AlbumOption{
		ReleaseMBID: "rel-A", Enriched: true, TrackCount: 2, DiscCount: 1,
		// Durations 3s apart, well inside the 12s tolerance, so only the title
		// can separate these two slots.
		Tracks: []Slot{
			slot(1, 1, "Song (Live)", 200, "rec-live"),
			slot(1, 2, "Song (Remix)", 203, "rec-remix"),
		},
	}
	results := []fileResult{
		{input: Input{Path: "remix.flac", CurrentTitle: "Song (Remix)"}, duration: 201},
	}

	fillGaps(o, results)

	a := assignmentFor(o, "remix.flac")
	if a == nil || a.RecordingMBID != "rec-remix" {
		t.Fatalf("expected the remix slot to win on title, got %+v", a)
	}
}

func TestFillGapsMarksHopelessFilesAsNone(t *testing.T) {
	o := &AlbumOption{
		ReleaseMBID: "rel-A", Enriched: true, TrackCount: 1, DiscCount: 1,
		Tracks:      []Slot{slot(1, 1, "One", 180, "rec-1")},
		Assignments: []Assignment{{
			Path: "01.flac", Source: SourceFingerprint, DiscNumber: 1, TrackNumber: 1,
		}},
		MatchedCount: 1,
	}
	// The only slot is taken, so this file has nowhere to go.
	results := []fileResult{
		{input: Input{Path: "01.flac"}, duration: 180},
		{input: Input{Path: "extra.flac"}, duration: 999},
	}

	fillGaps(o, results)

	a := assignmentFor(o, "extra.flac")
	if a == nil || a.Source != SourceNone {
		t.Fatalf("expected a none-source assignment, got %+v", a)
	}
	if a.TrackNumber != 0 || a.Title != "" {
		t.Fatalf("a none assignment must carry no position or title: %+v", a)
	}
}

func TestFillGapsCarriesFingerprintErrors(t *testing.T) {
	o := &AlbumOption{ReleaseMBID: "rel-A", Enriched: true, Tracks: []Slot{slot(1, 1, "One", 180, "rec-1")}}
	results := []fileResult{{input: Input{Path: "bad.flac"}, err: errFake}}

	fillGaps(o, results)

	a := assignmentFor(o, "bad.flac")
	if a == nil || a.Source != SourceNone || a.Error == "" {
		t.Fatalf("expected a none assignment carrying the error, got %+v", a)
	}
}

// An un-enriched option has no tracklist, so nothing can be inferred — but
// every file must still appear, or the dialog would silently drop rows.
func TestFillGapsWithoutTracklistMarksEverythingNone(t *testing.T) {
	o := &AlbumOption{ReleaseMBID: "rel-A"}
	results := []fileResult{
		{input: Input{Path: "a.flac"}, duration: 180},
		{input: Input{Path: "b.flac"}, duration: 200},
	}

	fillGaps(o, results)

	if len(o.Assignments) != 2 {
		t.Fatalf("expected 2 assignments, got %+v", o.Assignments)
	}
	for _, a := range o.Assignments {
		if a.Source != SourceNone {
			t.Fatalf("expected all none, got %+v", a)
		}
	}
}

type fakeFileIdentifier struct {
	// byPath maps an absolute path to its fingerprint outcome.
	byPath map[string]fileResult
}

func (f fakeFileIdentifier) IdentifyFileWithDuration(
	_ context.Context, absPath string,
) ([]acoustid.Recording, float64, error) {
	res := f.byPath[absPath]
	return res.recordings, res.duration, res.err
}

type fakeReleaseLookup struct {
	mu      sync.Mutex
	calls   []string
	byMBID  map[string]artistimage.ReleaseDetail
	failFor map[string]bool
}

func (f *fakeReleaseLookup) Release(
	_ context.Context, mbid string,
) (artistimage.ReleaseDetail, error) {
	f.mu.Lock()
	f.calls = append(f.calls, mbid)
	f.mu.Unlock()
	if f.failFor[mbid] {
		return artistimage.ReleaseDetail{}, errFake
	}
	return f.byMBID[mbid], nil
}

func TestResolveEnrichesAndFillsTheBestOption(t *testing.T) {
	ident := fakeFileIdentifier{byPath: map[string]fileResult{
		"/lib/01.flac": {
			recordings: []acoustid.Recording{rec(0.95, "rec-1", "One", rel("rel-A", "Album A", 1991, 1, 1))},
			duration:   180,
		},
		// No fingerprint match; its duration matches track 2.
		"/lib/02.flac": {duration: 200},
	}}
	releases := &fakeReleaseLookup{byMBID: map[string]artistimage.ReleaseDetail{
		"rel-A": {
			ReleaseMBID: "rel-A", ReleaseGroupMBID: "rg-A", Title: "Album A", Date: "1991-09-24",
			Artists:    []artistimage.ReleaseArtistCredit{{Name: "Artist", MBID: "art-1"}},
			TrackCount: 2, DiscCount: 1,
			Tracks: []artistimage.ReleaseTrack{
				{DiscNumber: 1, TrackNumber: 1, Title: "One", DurationSeconds: 180, RecordingMBID: "rec-1"},
				{DiscNumber: 1, TrackNumber: 2, Title: "Two", DurationSeconds: 200, RecordingMBID: "rec-2"},
			},
		},
	}}

	r := New(ident, releases)
	opts, _, err := r.Resolve(context.Background(), []Input{
		{Path: "01.flac", AbsPath: "/lib/01.flac"},
		{Path: "02.flac", AbsPath: "/lib/02.flac"},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(opts) != 1 {
		t.Fatalf("expected 1 option, got %d", len(opts))
	}
	o := opts[0]
	if !o.Enriched || o.TrackCount != 2 || o.DiscCount != 1 {
		t.Fatalf("expected an enriched option, got %+v", o)
	}
	if len(o.Artists) != 1 || o.Artists[0].MBID != "art-1" {
		t.Fatalf("expected release artists from MusicBrainz, got %+v", o.Artists)
	}
	if o.Year != 1991 {
		t.Fatalf("expected the year parsed from the MB date, got %d", o.Year)
	}
	if len(o.Assignments) != 2 {
		t.Fatalf("expected an assignment per input, got %+v", o.Assignments)
	}
	var fp, inferred int
	for _, a := range o.Assignments {
		switch a.Source {
		case SourceFingerprint:
			fp++
		case SourceInferred:
			inferred++
		}
	}
	if fp != 1 || inferred != 1 {
		t.Fatalf("expected 1 fingerprint + 1 inferred, got %d/%d", fp, inferred)
	}
}

func TestResolveCapsEnrichmentAtMaxEnrichedOptions(t *testing.T) {
	// 20 distinct releases, one per song, so the union has 20 options.
	byPath := map[string]fileResult{}
	inputs := make([]Input, 0, 20)
	byMBID := map[string]artistimage.ReleaseDetail{}
	for i := 0; i < 20; i++ {
		p := fmt.Sprintf("/lib/%02d.flac", i)
		mbid := fmt.Sprintf("rel-%02d", i)
		byPath[p] = fileResult{
			recordings: []acoustid.Recording{
				rec(0.9, fmt.Sprintf("rec-%02d", i), "Song", rel(mbid, "Album "+mbid, 1991, 1, i+1)),
			},
			duration: 180,
		}
		inputs = append(inputs, Input{Path: fmt.Sprintf("%02d.flac", i), AbsPath: p})
		byMBID[mbid] = artistimage.ReleaseDetail{
			ReleaseMBID: mbid, Title: "Album " + mbid, TrackCount: 1, DiscCount: 1,
		}
	}
	releases := &fakeReleaseLookup{byMBID: byMBID}

	r := New(fakeFileIdentifier{byPath: byPath}, releases)
	opts, _, err := r.Resolve(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(opts) != 20 {
		t.Fatalf("expected all 20 options returned, got %d", len(opts))
	}
	if len(releases.calls) != MaxEnrichedOptions {
		t.Fatalf("expected %d enrichment calls, got %d", MaxEnrichedOptions, len(releases.calls))
	}
	var enriched int
	for _, o := range opts {
		if o.Enriched {
			enriched++
		}
	}
	if enriched != MaxEnrichedOptions {
		t.Fatalf("expected %d enriched options, got %d", MaxEnrichedOptions, enriched)
	}
}

func TestResolveDegradesAFailedEnrichment(t *testing.T) {
	ident := fakeFileIdentifier{byPath: map[string]fileResult{
		"/lib/01.flac": {
			recordings: []acoustid.Recording{rec(0.95, "rec-1", "One", rel("rel-A", "Album A", 1991, 1, 1))},
			duration:   180,
		},
	}}
	releases := &fakeReleaseLookup{failFor: map[string]bool{"rel-A": true}}

	r := New(ident, releases)
	opts, _, err := r.Resolve(context.Background(), []Input{{Path: "01.flac", AbsPath: "/lib/01.flac"}})
	if err != nil {
		t.Fatalf("Resolve must not fail when MusicBrainz does: %v", err)
	}
	if len(opts) != 1 || opts[0].Enriched {
		t.Fatalf("expected one un-enriched option, got %+v", opts)
	}
	// The fingerprint data still stands on its own.
	if opts[0].Album != "Album A" || len(opts[0].Assignments) != 1 {
		t.Fatalf("unexpected degraded option: %+v", opts[0])
	}
}

func TestResolveReportsPerFileFailuresWithoutFailing(t *testing.T) {
	ident := fakeFileIdentifier{byPath: map[string]fileResult{
		"/lib/01.flac": {
			recordings: []acoustid.Recording{rec(0.95, "rec-1", "One", rel("rel-A", "Album A", 1991, 1, 1))},
			duration:   180,
		},
		"/lib/bad.flac": {err: errFake},
	}}
	releases := &fakeReleaseLookup{byMBID: map[string]artistimage.ReleaseDetail{
		"rel-A": {ReleaseMBID: "rel-A", Title: "Album A", TrackCount: 1, DiscCount: 1,
			Tracks: []artistimage.ReleaseTrack{{DiscNumber: 1, TrackNumber: 1, Title: "One",
				DurationSeconds: 180, RecordingMBID: "rec-1"}}},
	}}

	r := New(ident, releases)
	opts, _, err := r.Resolve(context.Background(), []Input{
		{Path: "01.flac", AbsPath: "/lib/01.flac"},
		{Path: "bad.flac", AbsPath: "/lib/bad.flac"},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	a := assignmentFor(&opts[0], "bad.flac")
	if a == nil || a.Source != SourceNone || a.Error == "" {
		t.Fatalf("expected the failure carried on the row, got %+v", a)
	}
}

func TestResolveWithNoMatchesReturnsNoOptions(t *testing.T) {
	ident := fakeFileIdentifier{byPath: map[string]fileResult{"/lib/01.flac": {duration: 180}}}
	r := New(ident, &fakeReleaseLookup{})
	opts, fileErrs, err := r.Resolve(context.Background(), []Input{{Path: "01.flac", AbsPath: "/lib/01.flac"}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(opts) != 0 {
		t.Fatalf("expected no options, got %+v", opts)
	}
	if len(fileErrs) != 0 {
		t.Fatalf("expected no file errors when the file just didn't match, got %+v", fileErrs)
	}
}

// Pre-fix, when every file failed to fingerprint, Resolve returned (nil, nil)
// and the dialog told the user "None matched" — a hard failure was misdiagnosed
// as a lookup miss.
func TestResolveReturnsFileErrorsWhenAllFilesFail(t *testing.T) {
	ident := fakeFileIdentifier{byPath: map[string]fileResult{
		"/lib/a.flac": {err: errFake},
		"/lib/b.flac": {err: errFake},
	}}
	r := New(ident, &fakeReleaseLookup{})
	opts, fileErrs, err := r.Resolve(context.Background(), []Input{
		{Path: "a.flac", AbsPath: "/lib/a.flac"},
		{Path: "b.flac", AbsPath: "/lib/b.flac"},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(opts) != 0 {
		t.Fatalf("expected no options when all files failed, got %+v", opts)
	}
	if len(fileErrs) != 2 {
		t.Fatalf("expected 2 file errors, got %d", len(fileErrs))
	}
	for _, fe := range fileErrs {
		if fe.Error == "" {
			t.Fatalf("file error must carry the error message, got %+v", fe)
		}
	}
}

// upstreamErr builds the failure an AcoustID outage produces, wrapped the way
// internal/identify wraps it, so the discriminator under test is the real one.
func upstreamErr(kind upstream.Kind, status int) error {
	return fmt.Errorf("acoustid: %w",
		upstream.WrapError("AcoustID", kind, status, errors.New("boom")))
}

// Pre-fix, Resolve had no failure path at all: a rate-limited or unreachable
// AcoustID was collected into fileErrors and answered 200, reading to the user
// as "these files could not be identified" instead of "try again in a moment".
func TestResolveFailsWhenEveryFileHitsAnUpstreamOutage(t *testing.T) {
	for _, tc := range []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"rate limited", upstreamErr(upstream.KindRateLimited, 429), 429},
		{"unreachable", upstreamErr(upstream.KindUnreachable, 0), 502},
		{"timeout", upstreamErr(upstream.KindTimeout, 0), 504},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ident := fakeFileIdentifier{byPath: map[string]fileResult{
				"/lib/a.flac": {err: tc.err},
				"/lib/b.flac": {err: tc.err},
			}}
			r := New(ident, &fakeReleaseLookup{})
			_, _, err := r.Resolve(context.Background(), []Input{
				{Path: "a.flac", AbsPath: "/lib/a.flac"},
				{Path: "b.flac", AbsPath: "/lib/b.flac"},
			})
			if err == nil {
				t.Fatal("expected Resolve to fail on a total upstream outage")
			}
			var uerr *upstream.Error
			if !errors.As(err, &uerr) {
				t.Fatalf("expected an *upstream.Error the handler can classify, got %v", err)
			}
			if got := upstream.HTTPStatus(err); got != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, got)
			}
		})
	}
}

// A total failure for FILE-specific reasons (missing fpcalc, unsupported codec,
// unreadable file) is not a request failure: those reports belong on the rows.
func TestResolveDoesNotFailWhenEveryFileFailsForItsOwnReasons(t *testing.T) {
	ident := fakeFileIdentifier{byPath: map[string]fileResult{
		"/lib/a.flac": {err: errors.New("fingerprint: fpcalc exec: executable file not found in $PATH")},
		"/lib/b.flac": {err: errors.New("fingerprint: fpcalc json: unexpected end of JSON input")},
	}}
	r := New(ident, &fakeReleaseLookup{})
	_, fileErrs, err := r.Resolve(context.Background(), []Input{
		{Path: "a.flac", AbsPath: "/lib/a.flac"},
		{Path: "b.flac", AbsPath: "/lib/b.flac"},
	})
	if err != nil {
		t.Fatalf("per-file failures must not fail the request: %v", err)
	}
	if len(fileErrs) != 2 {
		t.Fatalf("expected both failures reported per file, got %+v", fileErrs)
	}
}

// One bad file among good ones is never a request failure, even when the bad
// one failed upstream-shaped: the answer for the rest is still useful.
func TestResolveSucceedsWhenOnlySomeFilesFailUpstream(t *testing.T) {
	ident := fakeFileIdentifier{byPath: map[string]fileResult{
		"/lib/01.flac": {
			recordings: []acoustid.Recording{rec(0.95, "rec-1", "One", rel("rel-A", "Album A", 1991, 1, 1))},
			duration:   180,
		},
		"/lib/bad.flac": {err: upstreamErr(upstream.KindRateLimited, 429)},
	}}
	r := New(ident, &fakeReleaseLookup{})
	opts, fileErrs, err := r.Resolve(context.Background(), []Input{
		{Path: "01.flac", AbsPath: "/lib/01.flac"},
		{Path: "bad.flac", AbsPath: "/lib/bad.flac"},
	})
	if err != nil {
		t.Fatalf("a partial failure must not fail the request: %v", err)
	}
	if len(opts) != 1 {
		t.Fatalf("expected the good file's option, got %+v", opts)
	}
	if len(fileErrs) != 1 || fileErrs[0].Path != "bad.flac" {
		t.Fatalf("expected the one failure reported per file, got %+v", fileErrs)
	}
}

// A total failure mixing an outage with a file-specific problem is still an
// outage: nothing was identified and the retryable cause explains why.
func TestResolveFailsWhenATotalFailureIncludesAnOutage(t *testing.T) {
	ident := fakeFileIdentifier{byPath: map[string]fileResult{
		"/lib/a.flac": {err: errors.New("fingerprint: fpcalc: empty fingerprint in output")},
		"/lib/b.flac": {err: upstreamErr(upstream.KindRateLimited, 429)},
	}}
	r := New(ident, &fakeReleaseLookup{})
	if _, _, err := r.Resolve(context.Background(), []Input{
		{Path: "a.flac", AbsPath: "/lib/a.flac"},
		{Path: "b.flac", AbsPath: "/lib/b.flac"},
	}); err == nil || upstream.HTTPStatus(err) != 429 {
		t.Fatalf("expected a classified rate-limit failure, got %v", err)
	}
}

// A provider REFUSAL (bad API key, malformed fingerprint) is not retryable and
// not per-file either; it must still reach the handler as a request failure
// rather than be reported as eleven unidentifiable files.
func TestResolveFailsOnATotalProviderRejection(t *testing.T) {
	ident := fakeFileIdentifier{byPath: map[string]fileResult{
		"/lib/a.flac": {err: upstreamErr(upstream.KindRejected, 400)},
		"/lib/b.flac": {err: upstreamErr(upstream.KindRejected, 400)},
	}}
	r := New(ident, &fakeReleaseLookup{})
	if _, _, err := r.Resolve(context.Background(), []Input{
		{Path: "a.flac", AbsPath: "/lib/a.flac"},
		{Path: "b.flac", AbsPath: "/lib/b.flac"},
	}); err == nil {
		t.Fatal("expected a total provider rejection to fail the request")
	}
}

func TestEnrichIsIdempotent(t *testing.T) {
	releases := &fakeReleaseLookup{byMBID: map[string]artistimage.ReleaseDetail{
		"rel-A": {
			ReleaseMBID: "rel-A", Title: "Album A",
			Artists:    []artistimage.ReleaseArtistCredit{{Name: "Artist", MBID: "art-1"}},
			TrackCount: 2, DiscCount: 1,
			Tracks: []artistimage.ReleaseTrack{
				{DiscNumber: 1, TrackNumber: 1, Title: "One", DurationSeconds: 180, RecordingMBID: "rec-1"},
				{DiscNumber: 1, TrackNumber: 2, Title: "Two", DurationSeconds: 200, RecordingMBID: "rec-2"},
			},
		},
	}}
	r := New(nil, releases)
	o := &AlbumOption{ReleaseMBID: "rel-A"}

	r.enrich(context.Background(), o)
	artistsAfterOne := len(o.Artists)
	tracksAfterOne := len(o.Tracks)

	r.enrich(context.Background(), o)
	if len(o.Artists) != artistsAfterOne {
		t.Fatalf("enrich is not idempotent: artists grew from %d to %d", artistsAfterOne, len(o.Artists))
	}
	if len(o.Tracks) != tracksAfterOne {
		t.Fatalf("enrich is not idempotent: tracks grew from %d to %d", tracksAfterOne, len(o.Tracks))
	}
}
