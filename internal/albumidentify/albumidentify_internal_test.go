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
