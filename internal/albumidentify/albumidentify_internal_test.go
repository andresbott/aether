package albumidentify

import (
	"errors"
	"testing"

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
