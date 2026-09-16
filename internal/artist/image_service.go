// Package artist holds artist-domain application services that compose the
// lower-level lookup and persistence packages. ImageService acquires an
// artist's portrait from external providers (via a Fetcher, satisfied by
// *artistimage.Chain) and stores it in the asset store, keeping the pure
// internal/artistimage lookup library free of any storage dependency.
package artist

import (
	"context"
	"fmt"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
)

// Fetcher fetches an artist image by MusicBrainz MBID. Satisfied by
// *artistimage.Chain.
type Fetcher interface {
	Fetch(ctx context.Context, mbid string) ([]byte, string, error)
}

// ImageService fetches an artist's image from external providers and stores it
// in the asset store. It is the on-demand fetch used by the setMBID handler; it
// holds no batch or pacing state.
type ImageService struct {
	assets *assetstore.Store
	fetch  Fetcher
}

// NewImageService wires the service. fetch is nil when no provider API key is
// configured; callers guard on a nil *ImageService before using it.
func NewImageService(as *assetstore.Store, fetch Fetcher) *ImageService {
	return &ImageService{assets: as, fetch: fetch}
}

// FetchAndStore attempts to fetch a's image and, on success, stores it in the
// asset store. It reports whether an image was stored.
func (svc *ImageService) FetchAndStore(ctx context.Context, a model.Artist) (bool, error) {
	data, ext, err := svc.fetch.Fetch(ctx, a.MBArtistID)
	if err != nil {
		return false, err
	}
	if len(data) == 0 {
		return false, nil
	}
	if err := svc.assets.PutAuto(assetstore.KindArtist, assetkey.Artist(a.MBArtistID, a.NameNorm), ext, data); err != nil {
		return false, fmt.Errorf("store image: %w", err)
	}
	return true, nil
}
