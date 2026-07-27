// Package identify resolves audio files to MusicBrainz recordings by acoustic
// fingerprint: Chromaprint (fpcalc) computes the fingerprint, the AcoustID web
// service matches it. Both dependencies are optional at the application level;
// construct an Identifier only when they are available.
package identify

import (
	"context"
	"fmt"

	"github.com/andresbott/aether/libs/acoustid"
	"github.com/andresbott/aether/libs/fpcalc"
)

// Identifier fingerprints files and looks them up on AcoustID.
type Identifier struct {
	Fp     *fpcalc.Client
	Acoust *acoustid.Client
}

// New returns an Identifier from an fpcalc client and an AcoustID client.
func New(fp *fpcalc.Client, ac *acoustid.Client) *Identifier {
	return &Identifier{Fp: fp, Acoust: ac}
}

// IdentifyFile fingerprints the audio file at absPath and returns matching
// MusicBrainz recordings ordered by score descending. An empty slice means the
// fingerprint matched nothing.
func (i *Identifier) IdentifyFile(ctx context.Context, absPath string) ([]acoustid.Recording, error) {
	recs, _, err := i.IdentifyFileWithDuration(ctx, absPath)
	return recs, err
}

// IdentifyFileWithDuration is IdentifyFile plus the track duration fpcalc
// measured, in seconds. Callers that map several files onto one album use it to
// place a file the fingerprint did not match: its duration against a release's
// tracklist is the strongest remaining signal.
func (i *Identifier) IdentifyFileWithDuration(
	ctx context.Context, absPath string,
) ([]acoustid.Recording, float64, error) {
	fp, err := i.Fp.Fingerprint(ctx, absPath)
	if err != nil {
		return nil, 0, fmt.Errorf("fingerprint: %w", err)
	}
	recs, err := i.Acoust.Lookup(ctx, fp.Fingerprint, fp.Duration)
	if err != nil {
		return nil, fp.Duration, fmt.Errorf("acoustid: %w", err)
	}
	return recs, fp.Duration, nil
}
