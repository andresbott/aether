// Package identify resolves audio files to MusicBrainz recordings by acoustic
// fingerprint: Chromaprint (fpcalc) computes the fingerprint, the AcoustID web
// service matches it. Both dependencies are optional at the application level;
// construct an Identifier only when they are available.
package identify

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/andresbott/aether/internal/upstream"
	"github.com/andresbott/aether/libs/acoustid"
	"github.com/andresbott/aether/libs/fpcalc"
)

// acoustIDServiceName is the provider name users see in an upstream error
// sentence.
const acoustIDServiceName = "AcoustID"

// asUpstream translates a typed AcoustID failure into internal/upstream's
// *Error, so a handler can classify an AcoustID outage exactly as it classifies
// MusicBrainz or Cover Art Archive: 429 → rate-limited, timeout → 504, transport
// → 502. libs/acoustid cannot produce *upstream.Error itself because libs/ has
// no aether imports by design, so this package is the translation seam.
//
// Errors that are not an *acoustid.LookupError (a cancelled context, say) are
// returned untouched: they are not the provider's fault and must not be reported
// as an outage.
func asUpstream(err error) error {
	var lerr *acoustid.LookupError
	if !errors.As(err, &lerr) {
		return err
	}
	kind := upstream.KindUnavailable
	switch {
	case lerr.Status == http.StatusTooManyRequests:
		kind = upstream.KindRateLimited
	case lerr.Timeout():
		kind = upstream.KindTimeout
	case lerr.Transport:
		kind = upstream.KindUnreachable
	case lerr.Status >= 400 && lerr.Status < 500:
		// A 4xx other than 429 is the service refusing this request (bad API
		// key, malformed fingerprint); retrying will not help.
		kind = upstream.KindRejected
	}
	return upstream.WrapError(acoustIDServiceName, kind, lerr.Status, err)
}

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
		// Wrapped with %w through asUpstream so callers can errors.As their way
		// to *upstream.Error and tell an AcoustID outage (a request-level
		// failure) from a per-file problem like an undecodable track.
		return nil, fp.Duration, fmt.Errorf("acoustid: %w", asUpstream(err))
	}
	return recs, fp.Duration, nil
}
