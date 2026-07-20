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
	fp, err := i.Fp.Fingerprint(ctx, absPath)
	if err != nil {
		return nil, fmt.Errorf("fingerprint: %w", err)
	}
	recs, err := i.Acoust.Lookup(ctx, fp.Fingerprint, fp.Duration)
	if err != nil {
		return nil, fmt.Errorf("acoustid: %w", err)
	}
	return recs, nil
}
