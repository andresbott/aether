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
	"time"

	"golang.org/x/time/rate"
)

// requestsPerSecond is the fair-use rate limit applied to outbound Cover Art
// Archive API and image-download requests (burst 1). The archive is backed by
// archive.org, so we stay polite.
const requestsPerSecond rate.Limit = 1

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
type Client struct {
	BaseURL   string
	UserAgent string
	Client    *http.Client
	limiter   *rate.Limiter
}

func New(userAgent string) *Client {
	return &Client{
		BaseURL:   "https://coverartarchive.org",
		UserAgent: userAgent,
		Client:    &http.Client{Timeout: 20 * time.Second},
		limiter:   rate.NewLimiter(requestsPerSecond, 1),
	}
}

// List returns cover candidates for the given release MBID, falling back to the
// release-group MBID when the release has no images (or no release MBID is
// known). An empty result is not an error.
func (c *Client) List(ctx context.Context, releaseMBID, releaseGroupMBID string) ([]CoverImage, error) {
	if releaseMBID != "" {
		imgs, err := c.list(ctx, "release", releaseMBID)
		if err != nil {
			return nil, err
		}
		if len(imgs) > 0 {
			return imgs, nil
		}
	}
	if releaseGroupMBID != "" {
		return c.list(ctx, "release-group", releaseGroupMBID)
	}
	return nil, nil
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // no cover art for this MBID
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cover art archive %s: status %d", kind, resp.StatusCode)
	}
	var body caaResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, "", err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download %s: status %d", imageURL, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, "", err
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
