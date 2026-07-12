// Package radiobrowser is a small client for the community-run
// radio-browser.info directory API. It searches internet radio stations by
// name and proxies station favicons server-side, so the admin UI can import
// stations without hitting third-party CORS restrictions or exposing the
// upstream API to the browser.
package radiobrowser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	// defaultBaseURL is the round-robin DNS pool that resolves to one of the
	// radio-browser.info mirrors.
	defaultBaseURL = "https://all.api.radio-browser.info"

	// requestsPerSecond is a conservative fair-use rate limit (burst 1).
	requestsPerSecond rate.Limit = 1

	// maxFaviconBytes caps a proxied favicon download. Matches the 5 MiB cover
	// limit enforced by the radio-station cover pipeline (subsonic.readCoverFile).
	maxFaviconBytes = 5 * 1024 * 1024
)

// Station is a simplified radio-browser.info search result, carrying only the
// fields the station-import UI needs.
type Station struct {
	Name        string `json:"name"`
	StreamURL   string `json:"streamUrl"`
	Homepage    string `json:"homepage"`
	Favicon     string `json:"favicon"`
	Tags        string `json:"tags"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
	Language    string `json:"language"`
	Codec       string `json:"codec"`
	Bitrate     int    `json:"bitrate"`
	Votes       int    `json:"votes"`
	UUID        string `json:"uuid"`
}

// Client queries the radio-browser.info API.
type Client struct {
	BaseURL    string
	UserAgent  string
	HTTPClient *http.Client
	limiter    *rate.Limiter
}

// New returns a Client pointed at the shared mirror pool with a conservative
// rate limit. userAgent should identify this application (radio-browser asks
// callers to send a descriptive User-Agent).
func New(userAgent string) *Client {
	return &Client{
		BaseURL:    defaultBaseURL,
		UserAgent:  userAgent,
		HTTPClient: &http.Client{Timeout: 20 * time.Second},
		limiter:    rate.NewLimiter(requestsPerSecond, 1),
	}
}

// rbStation mirrors the raw radio-browser.info station JSON. Only the fields we
// surface are decoded.
type rbStation struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	URLResolved string `json:"url_resolved"`
	Homepage    string `json:"homepage"`
	Favicon     string `json:"favicon"`
	Tags        string `json:"tags"`
	Country     string `json:"country"`
	CountryCode string `json:"countrycode"`
	Language    string `json:"language"`
	Codec       string `json:"codec"`
	Bitrate     int    `json:"bitrate"`
	Votes       int    `json:"votes"`
	UUID        string `json:"stationuuid"`
}

// Search looks up stations by name, returning up to limit results ordered by
// radio-browser's vote count (most popular first) with broken streams hidden.
// An empty query returns (nil, nil) without making a request.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Station, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	q := url.Values{}
	q.Set("name", query)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("hidebroken", "true")
	q.Set("order", "votes")
	q.Set("reverse", "true")
	u := fmt.Sprintf("%s/json/stations/search?%s", c.BaseURL, q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("radiobrowser search: status %d", resp.StatusCode)
	}
	var body []rbStation
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	out := make([]Station, 0, len(body))
	for _, s := range body {
		stream := s.URLResolved
		if stream == "" {
			stream = s.URL
		}
		out = append(out, Station{
			Name:        s.Name,
			StreamURL:   stream,
			Homepage:    s.Homepage,
			Favicon:     s.Favicon,
			Tags:        s.Tags,
			Country:     s.Country,
			CountryCode: s.CountryCode,
			Language:    s.Language,
			Codec:       s.Codec,
			Bitrate:     s.Bitrate,
			Votes:       s.Votes,
			UUID:        s.UUID,
		})
	}
	return out, nil
}

// FetchFavicon downloads a station favicon server-side (avoiding browser CORS
// limits) and returns the image bytes plus its content type.
//
// It only succeeds for PNG and JPEG images — the two formats the radio-station
// cover pipeline accepts (subsonic.readCoverFile). Any other format (.ico,
// .svg, .gif, …) is rejected so callers skip the cover rather than importing a
// file that would fail the create. It also rejects non-http(s) URLs and bodies
// larger than maxFaviconBytes.
func (c *Client) FetchFavicon(ctx context.Context, faviconURL string) (data []byte, contentType string, err error) {
	faviconURL = strings.TrimSpace(faviconURL)
	if faviconURL == "" {
		return nil, "", fmt.Errorf("favicon url is required")
	}
	parsed, perr := url.Parse(faviconURL)
	if perr != nil {
		return nil, "", fmt.Errorf("invalid favicon url: %w", perr)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, "", fmt.Errorf("favicon url must be http or https")
	}

	req, rerr := http.NewRequestWithContext(ctx, http.MethodGet, faviconURL, nil)
	if rerr != nil {
		return nil, "", rerr
	}
	req.Header.Set("User-Agent", c.UserAgent)
	if werr := c.limiter.Wait(ctx); werr != nil {
		return nil, "", werr
	}
	resp, derr := c.HTTPClient.Do(req)
	if derr != nil {
		return nil, "", derr
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("radiobrowser favicon: status %d", resp.StatusCode)
	}

	// Read one byte past the cap so we can detect an over-limit body.
	body, aerr := io.ReadAll(io.LimitReader(resp.Body, maxFaviconBytes+1))
	if aerr != nil {
		return nil, "", aerr
	}
	if len(body) > maxFaviconBytes {
		return nil, "", fmt.Errorf("favicon exceeds %d bytes", maxFaviconBytes)
	}

	// Sniff the actual bytes rather than trusting the Content-Type header, and
	// only accept the formats the cover store can persist.
	sniff := body
	if len(sniff) > 512 {
		sniff = sniff[:512]
	}
	switch ct := http.DetectContentType(sniff); ct {
	case "image/png", "image/jpeg":
		return body, ct, nil
	default:
		return nil, "", fmt.Errorf("favicon is not a PNG or JPEG image (%s)", ct)
	}
}
