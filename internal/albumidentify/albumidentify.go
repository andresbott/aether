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

	"github.com/andresbott/aether/internal/artistimage"
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
// today — the current album name and track number are ranking and gap-fill
// signals, not values to preserve.
type Input struct {
	Path               string
	AbsPath            string
	CurrentAlbum       string
	CurrentTitle       string
	CurrentTrackNumber int
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
