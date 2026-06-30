// Package artistimage fetches artist images from external providers, keyed by
// the MusicBrainz artist MBID, behind a small Provider interface.
package artistimage

import (
	"context"
	"net/url"
	"path"
	"strings"
)

type Provider interface {
	// Fetch returns image bytes and a file extension ("jpg"/"png"), or
	// (nil, "", nil) when the provider has no image for this MBID.
	Fetch(ctx context.Context, mbid string) ([]byte, string, error)
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
