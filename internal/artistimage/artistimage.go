// Package artistimage fetches artist images from external providers, keyed by
// the MusicBrainz artist MBID, behind a small Provider interface.
package artistimage

import (
	"context"
	"net/url"
	"path"
	"strings"

	"golang.org/x/time/rate"
)

// requestsPerSecond is the fair-use rate limit applied per provider to its
// outbound API and image-download requests (burst 1).
const requestsPerSecond rate.Limit = 1

// ImageCandidate is one portrait a provider offers for an artist. FullURL is the
// image stored on commit; ThumbURL is a lighter preview variant the grid loads.
type ImageCandidate struct {
	FullURL  string
	ThumbURL string
	Provider string // producing Provider.Name(); routes Chain.Download
}

type Provider interface {
	// Fetch returns image bytes and a file extension ("jpg"/"png"), or
	// (nil, "", nil) when the provider has no image for this MBID.
	Fetch(ctx context.Context, mbid string) ([]byte, string, error)
	// List returns the provider's portrait candidates for the MBID, in the
	// order the provider returns them, or nil when it has none.
	List(ctx context.Context, mbid string) ([]ImageCandidate, error)
	// Download fetches the bytes of a URL this provider listed.
	Download(ctx context.Context, url string) ([]byte, string, error)
	Name() string
}

type Chain struct {
	providers []Provider
}

func NewChain(ps ...Provider) *Chain { return &Chain{providers: ps} }

func (c *Chain) Fetch(ctx context.Context, mbid string) ([]byte, string, error) {
	for _, p := range c.providers {
		data, ext, err := p.Fetch(ctx, mbid)
		if err != nil {
			continue // treat provider error as "no image from this provider"
		}
		if len(data) > 0 {
			return data, ext, nil
		}
	}
	return nil, "", nil
}

// extFromURL derives a normalized image extension from a URL, defaulting to jpg.
func extFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "jpg"
	}
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(u.Path), "."))
	if ext == "jpeg" {
		ext = "jpg"
	}
	if ext != "jpg" && ext != "png" {
		ext = "jpg"
	}
	return ext
}
