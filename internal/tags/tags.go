// internal/tags/tags.go
package tags

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrUnsupported = errors.New("filetype unsupported")

type Reader interface {
	// CanRead takes no context: it only matches the file extension and never
	// touches the file.
	CanRead(absPath string) bool
	// Read extracts the file's metadata. A tag read can shell out to another
	// process (see FFProbeReader), so ctx carries the caller's cancellation —
	// abandoning a scan or a request stops the read instead of leaving it to
	// run to completion.
	Read(ctx context.Context, absPath string) (Metadata, error)
}

type Metadata struct {
	Title           string
	Artist          []string
	AlbumArtist     []string
	Album           string
	Genre           []string
	Year            int
	TrackNumber     int
	DiscNumber      int
	DiscSubtitle    string
	Duration        time.Duration
	Bitrate         int
	MBRecordingID    string
	MBReleaseID      string
	MBReleaseGroupID string
	MBArtistID       []string
	MBAlbumArtistID  []string
	Lyrics          string
	Compilation     bool
	ReleaseType     string
	HasCover        bool
	ReplayGain      ReplayGain
}

type ReplayGain struct {
	TrackGain *float64
	TrackPeak *float64
	AlbumGain *float64
	AlbumPeak *float64
}

// allOrSingle returns the values for the first present key, split on the
// MusicBrainz multi-value separators ("\x00" and ";"). Returns nil if absent.
func allOrSingle(tags map[string][]string, keys ...string) []string {
	for _, k := range keys {
		if vs, ok := tags[k]; ok && len(vs) > 0 {
			raw := strings.Join(vs, "\x00")
			parts := strings.FieldsFunc(raw, func(r rune) bool { return r == 0 || r == ';' })
			out := make([]string, 0, len(parts))
			for _, p := range parts {
				if t := strings.TrimSpace(p); t != "" {
					out = append(out, t)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

type FallbackReader struct {
	primary  Reader
	fallback Reader
}

func NewFallbackReader(primary, fallback Reader) *FallbackReader {
	return &FallbackReader{primary: primary, fallback: fallback}
}

func (r *FallbackReader) CanRead(absPath string) bool {
	return r.primary.CanRead(absPath) || r.fallback.CanRead(absPath)
}

func (r *FallbackReader) Read(ctx context.Context, absPath string) (Metadata, error) {
	if r.primary.CanRead(absPath) {
		m, err := r.primary.Read(ctx, absPath)
		if err == nil {
			return m, nil
		}
		// A canceled or expired context is the caller's decision, not a defect in
		// the file, so don't spend the fallback reader on it — that would double
		// the work already known to be unwanted.
		if ctx.Err() != nil {
			return Metadata{}, err
		}
		if r.fallback.CanRead(absPath) {
			return r.fallback.Read(ctx, absPath)
		}
		return Metadata{}, err
	}
	if r.fallback.CanRead(absPath) {
		return r.fallback.Read(ctx, absPath)
	}
	return Metadata{}, ErrUnsupported
}
