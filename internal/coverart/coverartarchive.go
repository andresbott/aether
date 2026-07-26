// Package coverart fetches album cover images from the Cover Art Archive
// (https://coverartarchive.org), keyed by a MusicBrainz release or
// release-group MBID.
package coverart

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/andresbott/aether/internal/upstream"
	"golang.org/x/time/rate"
)

// requestsPerSecond is the fair-use rate limit applied to outbound Cover Art
// Archive API and image-download requests (burst 1). The archive is backed by
// archive.org, so we stay polite.
const requestsPerSecond rate.Limit = 1

// serviceName is what the user sees when the archive misbehaves.
const serviceName = "Cover Art Archive"

// CoverImage is a single cover candidate from the Cover Art Archive.
type CoverImage struct {
	ID       string   `json:"id"`
	ImageURL string   `json:"imageUrl"`
	ThumbURL string   `json:"thumbUrl"`
	IsFront  bool     `json:"isFront"`
	// Types is what the image depicts, e.g. ["Front"], ["Back"], ["Booklet"],
	// ["Medium"]. Comment is the uploader's free-text note, if any.
	Types   []string `json:"types"`
	Comment string   `json:"comment"`
}

// Client queries the Cover Art Archive JSON API and downloads cover images.
// Retries, throttling and error classification live in the shared Doer.
type Client struct {
	BaseURL string
	Doer    *upstream.Doer
}

func New(userAgent string) *Client {
	return &Client{
		BaseURL: "https://coverartarchive.org",
		Doer:    upstream.New(serviceName, userAgent, requestsPerSecond),
	}
}

// List returns cover candidates for the given release MBID, falling back to the
// release-group MBID when the release has no images (or no release MBID is
// known). An empty result is not an error.
//
// A failing release lookup is not fatal while a release-group MBID is still on
// offer: the group covers the same album, so we try it rather than failing the
// whole request over one bad archive response. The error only surfaces when
// there is nothing left to try.
func (c *Client) List(ctx context.Context, releaseMBID, releaseGroupMBID string) ([]CoverImage, error) {
	var releaseErr error
	if releaseMBID != "" {
		imgs, err := c.list(ctx, "release", releaseMBID)
		if err != nil {
			if releaseGroupMBID == "" {
				return nil, err
			}
			releaseErr = err
		}
		if len(imgs) > 0 {
			return imgs, nil
		}
	}
	if releaseGroupMBID != "" {
		imgs, err := c.list(ctx, "release-group", releaseGroupMBID)
		if err != nil {
			return nil, err
		}
		return imgs, nil
	}
	return nil, releaseErr
}

type caaResponse struct {
	Images []struct {
		ID         json.Number       `json:"id"`
		Image      string            `json:"image"`
		Front      bool              `json:"front"`
		Types      []string          `json:"types"`
		Comment    string            `json:"comment"`
		Thumbnails map[string]string `json:"thumbnails"`
	} `json:"images"`
}

func (c *Client) list(ctx context.Context, kind, mbid string) ([]CoverImage, error) {
	u := fmt.Sprintf("%s/%s/%s", c.BaseURL, kind, url.PathEscape(mbid))
	// 404 is the archive's "no cover art for this MBID" — a valid answer, not
	// a failure, so it is allowed through rather than retried.
	resp, err := c.Doer.Get(ctx, u, http.Header{"Accept": []string{"application/json"}}, http.StatusNotFound)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	var body caaResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, c.Doer.BadResponse(err)
	}
	out := make([]CoverImage, 0, len(body.Images))
	for _, img := range body.Images {
		if img.Image == "" {
			continue
		}
		out = append(out, CoverImage{
			ID:       img.ID.String(),
			ImageURL: img.Image,
			ThumbURL: pickThumb(img.Thumbnails, img.Image),
			IsFront:  img.Front,
			Types:    img.Types,
			Comment:  img.Comment,
		})
	}
	return out, nil
}

// pickThumb prefers a small thumbnail, falling back to the full image.
func pickThumb(thumbs map[string]string, full string) string {
	for _, key := range []string{"250", "small", "500", "large"} {
		if t := thumbs[key]; t != "" {
			return t
		}
	}
	return full
}

// DownloadImage fetches imageURL and returns its bytes and a normalized
// extension ("jpg"/"png").
func (c *Client) DownloadImage(ctx context.Context, imageURL string) ([]byte, string, error) {
	resp, err := c.Doer.Get(ctx, imageURL, nil)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, "", c.Doer.BadResponse(err)
	}
	return data, extFromURL(imageURL), nil
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
