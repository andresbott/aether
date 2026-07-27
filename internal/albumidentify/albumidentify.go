// Package albumidentify maps a set of audio files onto a single MusicBrainz
// release. Per-file fingerprint identification (internal/identify) answers
// "what recording is this?"; this package answers the album question the
// metadata editor actually asks when tagging a rip: which one release best
// explains this whole selection, and where does each file sit on it.
//
// The pipeline is: fingerprint every file, union every release any candidate
// mentions, rank the union by how well it covers the selection, enrich the
// best options with their MusicBrainz tracklist, and place the files the
// fingerprint missed against that tracklist.
package albumidentify

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/andresbott/aether/internal/artistimage"
	"github.com/andresbott/aether/internal/upstream"
	"github.com/andresbott/aether/libs/acoustid"
)

// Assignment sources: how a file ended up on a track position.
const (
	// SourceFingerprint: AcoustID matched the file to a recording that the
	// release carries at this position.
	SourceFingerprint = "fingerprint"
	// SourceInferred: the fingerprint did not place the file on this release,
	// so it was matched against the tracklist by duration and title.
	SourceInferred = "inferred"
	// SourceNone: no position could be justified; only album-level fields
	// apply to this file.
	SourceNone = "none"
)

// FileIdentifier fingerprints one file and returns its candidate recordings
// plus the measured duration. Satisfied by *identify.Identifier.
type FileIdentifier interface {
	IdentifyFileWithDuration(ctx context.Context, absPath string) ([]acoustid.Recording, float64, error)
}

// ReleaseLookup fetches one release with its full tracklist. Satisfied by
// *artistimage.MusicBrainzSearch.
type ReleaseLookup interface {
	Release(ctx context.Context, mbid string) (artistimage.ReleaseDetail, error)
}

// Input is one file to place on the album, with the tag values it carries
// today — the current album name, track number, and disc number are ranking and
// gap-fill signals, not values to preserve.
type Input struct {
	Path               string
	AbsPath            string
	CurrentAlbum       string
	CurrentTitle       string
	CurrentTrackNumber int
	CurrentDiscNumber  int
}

// Artist is one credited artist: the credited-as name and its MusicBrainz ID.
type Artist struct {
	Name string `json:"name"`
	MBID string `json:"mbid"`
}

// Slot is one position on a release's tracklist.
type Slot struct {
	DiscNumber      int     `json:"disc_number"`
	TrackNumber     int     `json:"track_number"`
	Title           string  `json:"title"`
	RecordingMBID   string  `json:"recording_mbid"`
	DurationSeconds float64 `json:"duration_seconds"`
}

// Assignment places one input file on one release. Source says how strong the
// placement is; Error is set instead when the file could not be fingerprinted
// at all.
type Assignment struct {
	Path          string   `json:"path"`
	Source        string   `json:"source"`
	Title         string   `json:"title"`
	RecordingMBID string   `json:"recording_mbid"`
	Artists       []Artist `json:"artists"`
	DiscNumber    int      `json:"disc_number"`
	TrackNumber   int      `json:"track_number"`
	Score         float64  `json:"score"`
	Error         string   `json:"error,omitempty"`
}

// AlbumOption is one candidate release for the whole selection, with a
// per-input assignment. Enriched reports whether the MusicBrainz tracklist
// lookup succeeded: when false, TrackCount/DiscCount/Tracks are unknown and no
// gap-fill was possible.
type AlbumOption struct {
	ReleaseMBID      string       `json:"release_mbid"`
	ReleaseGroupMBID string       `json:"release_group_mbid"`
	Album            string       `json:"album"`
	Year             int          `json:"year"`
	Artists          []Artist     `json:"artists"`
	TrackCount       int          `json:"track_count"`
	DiscCount        int          `json:"disc_count"`
	Enriched         bool         `json:"enriched"`
	MatchedCount     int          `json:"matched_count"`
	MeanScore        float64      `json:"mean_score"`
	Assignments      []Assignment `json:"assignments"`
	Tracks           []Slot       `json:"tracks"`
}

// fileResult is one file's fingerprint outcome.
type fileResult struct {
	input      Input
	recordings []acoustid.Recording
	duration   float64
	err        error
}

// MaxEnrichedOptions caps how many of the ranked options get a MusicBrainz
// tracklist lookup. A dozen-song selection routinely unions dozens of releases
// (every reissue and compilation a track ever appeared on) and MusicBrainz is
// throttled to a few requests per second, so enriching all of them would make
// the dialog take tens of seconds. The options below the cap still appear —
// with an unknown track count and no gap-fill — and the user can pick one.
const MaxEnrichedOptions = 8

// Per-file failure reasons. These are the sentences a person reads next to the
// file name, so they say what happened and nothing about how: a raw Go error
// here would put server filesystem paths ("/usr/bin/fpcalc") and fpcalc's own
// stderr into a user-facing response body, which docs/agents/architecture.md
// forbids.
//
// Only three, because only three distinctions change what the user can DO about
// it: fix/replace the file, wait and retry, or fix the selection.
const (
	// ReasonNotFingerprinted: the local fingerprint step failed — fpcalc is
	// missing, the file is not decodable audio, or it produced no fingerprint.
	// Nothing about retrying will change it.
	ReasonNotFingerprinted = "could not be fingerprinted"
	// ReasonLookupFailed: the file fingerprinted fine but the AcoustID lookup
	// failed for it. Worth retrying, unlike the above. (When EVERY file fails
	// this way the whole request fails instead — see upstreamFailure.)
	ReasonLookupFailed = "could not be looked up — the identification service failed"
	// ReasonOutsideLibrary: the path never reached identification at all because
	// it resolved outside the library root. Set by the API layer, which is where
	// paths are resolved.
	ReasonOutsideLibrary = "is outside the library"
)

// FileError is one file that produced no identification, with the short reason
// a person reads. The technical error is deliberately absent: it is only useful
// server-side and this struct is serialised straight to the client.
type FileError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// failureReason maps one file's fingerprint/lookup error to its user-facing
// reason. An upstream-typed error means the AcoustID call itself failed (see
// identify.asUpstream); anything else came from the local fingerprint step.
//
// It classifies on the error TYPE, never on error text, so a reworded message
// upstream cannot silently reclassify a failure.
func failureReason(err error) string {
	if err == nil {
		return ""
	}
	var uerr *upstream.Error
	if errors.As(err, &uerr) {
		return ReasonLookupFailed
	}
	return ReasonNotFingerprinted
}

// Resolver maps file selections onto single releases. Releases may be nil: the
// resolver then skips enrichment entirely and returns fingerprint-only options.
type Resolver struct {
	Identifier FileIdentifier
	Releases   ReleaseLookup
}

// New returns a Resolver from a file identifier and a release lookup.
func New(id FileIdentifier, rel ReleaseLookup) *Resolver {
	return &Resolver{Identifier: id, Releases: rel}
}

// Resolve fingerprints every input and returns the candidate releases for the
// selection (best first, each carrying one assignment per input) plus per-file
// fingerprint errors.
//
// A partial failure is never a request failure: a file that will not fingerprint
// and a release lookup that errors are both reported in the result — the first
// in the FileError slice, the second as an un-enriched option — because a partial
// answer is still useful to the user, and one bad file must not sink a
// twelve-file album.
//
// It fails only when NOTHING could be identified and the reason was the AcoustID
// service rather than the files: an outage answered as a 200 list of per-file
// errors reads as "these files could not be identified", when the truth is "try
// again in a moment". See upstreamFailure.
func (r *Resolver) Resolve(ctx context.Context, inputs []Input) ([]AlbumOption, []FileError, error) {
	results := make([]fileResult, 0, len(inputs))
	var fileErrors []FileError
	for _, in := range inputs {
		// Sequential on purpose: fpcalc is CPU-bound and the AcoustID client is
		// rate-limited, so concurrency buys nothing here.
		recs, dur, err := r.Identifier.IdentifyFileWithDuration(ctx, in.AbsPath)
		results = append(results, fileResult{input: in, recordings: recs, duration: dur, err: err})
		if err != nil {
			fileErrors = append(fileErrors, FileError{Path: in.Path, Error: failureReason(err)})
		}
	}
	if err := upstreamFailure(results); err != nil {
		return nil, fileErrors, err
	}

	options := unionReleases(results)
	if len(options) == 0 {
		return nil, fileErrors, nil
	}
	rankOptions(options, inputs)

	for i, o := range options {
		if i < MaxEnrichedOptions && r.Releases != nil {
			r.enrich(ctx, o)
		}
		fillGaps(o, results)
	}
	// Enrichment changed the signals (track count, disc count), so rank again.
	rankOptions(options, inputs)

	out := make([]AlbumOption, 0, len(options))
	for _, o := range options {
		out = append(out, *o)
	}
	return out, fileErrors, nil
}

// upstreamFailure returns a request-level error when EVERY input failed and at
// least one failure came from the AcoustID service itself; nil otherwise.
//
// Both halves of that condition matter. "Every input failed" is what separates
// an outage from one unreadable file among eleven good ones — a partial answer
// is still an answer, and the bad file's report belongs on its row. "Came from
// the service" is what separates a retryable outage from a per-file data problem
// (fpcalc missing, unsupported codec, truncated file): those are genuinely about
// the files, so 200 with per-file errors is the honest answer for them.
//
// The discriminator is *upstream.Error via errors.As, never error text.
// internal/identify wraps every AcoustID failure into that type (see
// identify.asUpstream), so a rate limit, a timeout, an unreachable host and a
// provider refusal all arrive typed and the handler's writeUpstreamErr maps them
// to 429/504/502 with a human sentence. A refusal is included even though
// retrying will not help: nothing was identified, and reporting a bad API key as
// "these files could not be fingerprinted" would send the user hunting the wrong
// problem.
//
// The returned error is the first upstream failure seen. Reporting one cause is
// enough — during an outage every file fails the same way, and the user needs the
// condition, not eleven copies of it.
func upstreamFailure(results []fileResult) error {
	if len(results) == 0 {
		return nil
	}
	var firstUpstream error
	for _, res := range results {
		if res.err == nil {
			// Something worked, so the service is reachable: not an outage.
			return nil
		}
		var uerr *upstream.Error
		if firstUpstream == nil && errors.As(res.err, &uerr) {
			firstUpstream = res.err
		}
	}
	return firstUpstream
}

// enrich fills an option's tracklist and album-level fields from MusicBrainz.
// A failure is silent by design: the option stays usable with only the data
// AcoustID gave us, and Enriched stays false so the caller can say so.
func (r *Resolver) enrich(ctx context.Context, o *AlbumOption) {
	detail, err := r.Releases.Release(ctx, o.ReleaseMBID)
	if err != nil || detail.ReleaseMBID == "" {
		return
	}
	o.Enriched = true
	if detail.Title != "" {
		o.Album = detail.Title
	}
	if detail.ReleaseGroupMBID != "" {
		o.ReleaseGroupMBID = detail.ReleaseGroupMBID
	}
	if y := yearOf(detail.Date); y > 0 {
		o.Year = y
	}
	o.TrackCount = detail.TrackCount
	o.DiscCount = detail.DiscCount
	// Assign fresh slices rather than appending so calling enrich twice yields
	// the same result as calling it once. The tracklist must not accumulate
	// across calls because gap-fill treats each slot as claimable exactly once.
	o.Artists = make([]Artist, 0, len(detail.Artists))
	for _, a := range detail.Artists {
		o.Artists = append(o.Artists, Artist{Name: a.Name, MBID: a.MBID})
	}
	o.Tracks = make([]Slot, 0, len(detail.Tracks))
	for _, t := range detail.Tracks {
		o.Tracks = append(o.Tracks, Slot{
			DiscNumber:      t.DiscNumber,
			TrackNumber:     t.TrackNumber,
			Title:           t.Title,
			RecordingMBID:   t.RecordingMBID,
			DurationSeconds: t.DurationSeconds,
		})
	}
}

// yearOf reads the year from a MusicBrainz date ("1991", "1991-09-24"); 0 when
// there is none.
func yearOf(date string) int {
	parts := strings.SplitN(strings.TrimSpace(date), "-", 2)
	y, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0
	}
	return y
}
