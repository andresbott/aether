// internal/tags/tags.go
package tags

import (
	"errors"
	"time"
)

var ErrUnsupported = errors.New("filetype unsupported")

type Reader interface {
	CanRead(absPath string) bool
	Read(absPath string) (Metadata, error)
}

type Metadata struct {
	Title         string
	Artist        []string
	AlbumArtist   []string
	Album         string
	Genre         []string
	Year          int
	TrackNumber   int
	DiscNumber    int
	DiscSubtitle  string
	Duration      time.Duration
	Bitrate       int
	MBRecordingID string
	MBReleaseID   string
	Lyrics        string
	Compilation   bool
	ReleaseType   string
	HasCover      bool
	ReplayGain    ReplayGain
}

type ReplayGain struct {
	TrackGain *float64
	TrackPeak *float64
	AlbumGain *float64
	AlbumPeak *float64
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

func (r *FallbackReader) Read(absPath string) (Metadata, error) {
	if r.primary.CanRead(absPath) {
		m, err := r.primary.Read(absPath)
		if err == nil {
			return m, nil
		}
		if r.fallback.CanRead(absPath) {
			return r.fallback.Read(absPath)
		}
		return Metadata{}, err
	}
	if r.fallback.CanRead(absPath) {
		return r.fallback.Read(absPath)
	}
	return Metadata{}, ErrUnsupported
}
