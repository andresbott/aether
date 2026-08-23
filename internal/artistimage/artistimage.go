// Package artistimage fetches artist images from external providers, keyed by
// the MusicBrainz artist MBID, behind a small Provider interface.
package artistimage

import (
	"context"
	"fmt"
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

func (c *Chain) List(ctx context.Context, mbid string) ([]ImageCandidate, error) {
	var all []ImageCandidate
	var lastErr error
	for _, p := range c.providers {
		cs, err := p.List(ctx, mbid)
		if err != nil {
			lastErr = err // a provider that errors is skipped, not fatal…
			continue
		}
		all = append(all, cs...)
	}
	if len(all) == 0 && lastErr != nil {
		return nil, lastErr // …unless nobody produced anything, then surface it
	}
	return all, nil
}

func (c *Chain) Download(ctx context.Context, providerName, url string) ([]byte, string, error) {
	for _, p := range c.providers {
		if p.Name() == providerName {
			return p.Download(ctx, url)
		}
	}
	return nil, "", fmt.Errorf("artistimage: no provider named %q", providerName)
}

// Fetch keeps the one-shot contract the auto-fetch job and setMBID rely on:
// list candidates, download the first. fanart.tv lists first, so the first
// candidate is still its top thumb.
func (c *Chain) Fetch(ctx context.Context, mbid string) ([]byte, string, error) {
	cands, err := c.List(ctx, mbid)
	if err != nil {
		return nil, "", err
	}
	if len(cands) == 0 {
		return nil, "", nil
	}
	return c.Download(ctx, cands[0].Provider, cands[0].FullURL)
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
