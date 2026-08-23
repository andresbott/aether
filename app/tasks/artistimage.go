package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/go-bumbu/tempo"
)

const FetchArtistImagesTaskName = "fetch-artist-images"

var FetchArtistImagesTaskDef = TaskDef{
	ID:          FetchArtistImagesTaskName,
	Name:        "Fetch Artist Images",
	Description: "Download artist images from external providers using MusicBrainz IDs",
}

// Fetcher fetches an artist image by MusicBrainz MBID. Satisfied by
// *artistimage.Chain.
type Fetcher interface {
	Fetch(ctx context.Context, mbid string) ([]byte, string, error)
}

// FetchAndStoreArtistImage attempts to fetch a's image via f, storing it in
// as on success, and always stamps a.LastImageFetchAt afterwards. It reports
// whether an image was stored. Callers decide their own skip/backoff policy
// before calling this — it always attempts the fetch.
func FetchAndStoreArtistImage(ctx context.Context, s *store.Store, as *assetstore.Store, f Fetcher, a model.Artist) (bool, error) {
	data, ext, ferr := f.Fetch(ctx, a.MBArtistID)
	now := time.Now()
	if ferr != nil {
		if uerr := s.SetArtistImageFetchedAt(a.ID, now); uerr != nil {
			return false, fmt.Errorf("stamp fetch time: %w", uerr)
		}
		return false, ferr
	}
	if len(data) == 0 {
		if uerr := s.SetArtistImageFetchedAt(a.ID, now); uerr != nil {
			return false, fmt.Errorf("stamp fetch time: %w", uerr)
		}
		return false, nil
	}
	storeErr := as.PutAuto(assetstore.KindArtist, assetkey.Artist(a.MBArtistID, a.NameNorm), ext, data)
	if uerr := s.SetArtistImageFetchedAt(a.ID, now); uerr != nil {
		return false, fmt.Errorf("stamp fetch time: %w", uerr)
	}
	if storeErr != nil {
		return false, fmt.Errorf("store image: %w", storeErr)
	}
	return true, nil
}

// NewFetchArtistImagesTaskFn iterates artists with a MusicBrainz ID and
// fetches a missing image for each, applying skip/backoff before attempting.
func NewFetchArtistImagesTaskFn(s *store.Store, as *assetstore.Store, f Fetcher, logger *slog.Logger, backoff time.Duration) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// No provider configured: report a clear, actionable message. The
		// runner writes a returned error to the task's log at ERROR level, so
		// this shows up in the execution log instead of failing silently.
		if f == nil {
			return fmt.Errorf("artist image fetching is not configured: set ArtistImages.FanartApiKey and/or ArtistImages.TheAudioDBApiKey in your config")
		}
		artists, err := s.ArtistsWithMBID()
		if err != nil {
			return fmt.Errorf("list artists with mbid: %w", err)
		}
		var stored, skipped, failed int
		for _, a := range artists {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if _, ok := as.Get(assetstore.KindArtist, assetkey.Artist(a.MBArtistID, a.NameNorm)); ok {
				skipped++
				continue
			}
			if a.LastImageFetchAt != nil && time.Since(*a.LastImageFetchAt) < backoff {
				skipped++
				continue
			}
			ok, ferr := FetchAndStoreArtistImage(ctx, s, as, f, a)
			if ferr != nil {
				// Surface fetch failures in the task log instead of swallowing
				// them — otherwise the task looks like it did nothing.
				tempo.Error(ctx, fmt.Sprintf("fetch image for %q (%s): %v", a.Name, a.MBArtistID, ferr))
				failed++
			} else if ok {
				stored++
			}
		}
		_ = logger
		tempo.Info(ctx, fmt.Sprintf("artist images: %d stored, %d failed, %d skipped", stored, failed, skipped))
		return nil
	}
}
