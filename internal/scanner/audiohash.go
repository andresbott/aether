// internal/scanner/audiohash.go
package scanner

import (
	"errors"
	"log/slog"

	"github.com/andresbott/aether/libs/audiohash"
)

// audioHashOf returns the metadata-invariant hash of the file's audio payload —
// a value that survives a tag rewrite and changes only when the audio itself
// does. planTrackContinuity uses it to prove a move whose file was *also*
// retagged, which the size-and-title fingerprint cannot anchor because a tag
// edit changes both of its parts.
//
// It returns "" rather than an error, because there is nothing for a caller to
// do about a missing hash except carry on with its other identity signals. A
// hash is an optimisation and never a reason to fail or skip a file.
func audioHashOf(absPath string) string {
	h, err := audiohash.File(absPath)
	switch {
	case err == nil:
		return h
	case errors.Is(err, audiohash.ErrUnsupported):
		// audiohash covers FLAC, MP3, MP4, WAV, AIFF, Ogg Vorbis and Opus;
		// walk.go admits sixteen extensions. WMA, APE, WavPack, TTA, DSF,
		// Matroska/WebM and raw AAC simply keep the size-and-title proof — the
		// behaviour they had before this existed — so an uncovered format is a
		// quiet miss, not a warning. So is an Ogg file carrying a mapping other
		// than Vorbis or Opus; an Ogg file that is chained, truncated or carries a
		// trailer, which leaves its stream with no page ending at end of file and
		// so no length component to bind into the digest; and a file with no
		// locatable audio at all, such as a WAV declaring a "data" size of 0.
		// Each of those declines on purpose: a hash computed without the
		// discriminator would be shared by every other file in the same shape, and
		// a shared hash can move one track's history onto another.
		return ""
	default:
		// A malformed or unreadable payload still gets indexed; it just cannot
		// prove a retagged move.
		slog.Debug("audio hash failed", "path", absPath, "err", err)
		return ""
	}
}
