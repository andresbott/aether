package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/andresbott/aether/internal/assetstore"
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
			if _, ok := as.Get(assetstore.KindArtist, a.MBArtistID); ok {
				skipped++
				continue
			}
			if a.LastImageFetchAt != nil && time.Since(*a.LastImageFetchAt) < backoff {
				skipped++
				continue
			}
			data, ext, ferr := f.Fetch(ctx, a.MBArtistID)
			if ferr != nil {
				// Surface fetch failures in the task log instead of swallowing
				// them — otherwise the task looks like it did nothing.
				tempo.Error(ctx, fmt.Sprintf("fetch image for %q (%s): %v", a.Name, a.MBArtistID, ferr))
				failed++
			} else if len(data) > 0 {
				if perr := as.PutAuto(assetstore.KindArtist, a.MBArtistID, ext, data); perr != nil {
					tempo.Error(ctx, fmt.Sprintf("store image for %q: %v", a.Name, perr))
					failed++
				} else {
					stored++
				}
			}
			now := time.Now()
			if uerr := s.SetArtistImageFetchedAt(a.ID, now); uerr != nil {
				tempo.Error(ctx, fmt.Sprintf("stamp fetch time for %q: %v", a.Name, uerr))
			}
		}
		_ = logger
		tempo.Info(ctx, fmt.Sprintf("artist images: %d stored, %d failed, %d skipped", stored, failed, skipped))
		return nil
	}
}
