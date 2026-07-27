// Package acoustid is a client for the AcoustID lookup web service
// (https://acoustid.org/webservice), which resolves Chromaprint acoustic
// fingerprints to MusicBrainz recordings.
package acoustid

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// The AcoustID service allows 3 requests per second per application.
const requestsPerSecond = 3

// ArtistCredit is one credited artist on a recording.
type ArtistCredit struct {
	MBID string `json:"mbid"`
	Name string `json:"name"`
}

// Release is one release (album) a recording appears on, with its parent
// release group. TrackNumber and DiscNumber locate the recording on the
// release (0 when the service reported no position).
type Release struct {
	MBID             string `json:"release_mbid"`
	ReleaseGroupMBID string `json:"release_group_mbid"`
	Title            string `json:"album"`
	Year             int    `json:"year"`
	TrackNumber      int    `json:"track_number"`
	DiscNumber       int    `json:"disc_number"`
}

// Recording is a MusicBrainz recording matched by fingerprint, with the
// AcoustID match score (0..1).
type Recording struct {
	Score   float64        `json:"score"`
	MBID    string         `json:"recording_mbid"`
	Title   string         `json:"title"`
	Artists []ArtistCredit `json:"artists"`
	Release []Release      `json:"releases"`
}

// LookupError is a failed AcoustID call, carrying enough structure for callers
// to tell a transport/throttling outage (retry later) from the service refusing
// the request itself (retrying is pointless).
//
// It is a distinct type rather than internal/upstream's *Error because libs/
// deliberately has no aether imports; internal/identify translates it.
type LookupError struct {
	// Status is the HTTP status, or 0 when the request never got a response.
	Status int
	// Transport is true when the request never completed (DNS, refused, reset,
	// timeout) — as opposed to a response the service refused with.
	Transport bool
	// Message is the service's own error text, when it sent one.
	Message string
	Err     error
}

func (e *LookupError) Error() string {
	switch {
	case e.Message != "":
		return fmt.Sprintf("acoustid lookup: %s", e.Message)
	case e.Status > 0:
		return fmt.Sprintf("acoustid lookup: status %d", e.Status)
	case e.Err != nil:
		return fmt.Sprintf("acoustid lookup: %v", e.Err)
	default:
		return "acoustid lookup: request failed"
	}
}

func (e *LookupError) Unwrap() error { return e.Err }

// Timeout reports whether the failure was a timeout, so callers can classify it
// without unwrapping to a net error themselves.
func (e *LookupError) Timeout() bool {
	var terr interface{ Timeout() bool }
	return errors.As(e.Err, &terr) && terr.Timeout()
}

// Client calls the AcoustID lookup API.
type Client struct {
	BaseURL   string
	UserAgent string
	Client    *http.Client
	apiKey    string
	limiter   *rate.Limiter
}

// New returns a Client using the given AcoustID application API key.
func New(apiKey, userAgent string) *Client {
	return &Client{
		BaseURL:   "https://api.acoustid.org",
		UserAgent: userAgent,
		Client:    &http.Client{Timeout: 20 * time.Second},
		apiKey:    apiKey,
		limiter:   rate.NewLimiter(requestsPerSecond, 1),
	}
}

// lookupResponse mirrors the relevant parts of the AcoustID lookup JSON.
type lookupResponse struct {
	Status string `json:"status"`
	Error  struct {
		Message string `json:"message"`
	} `json:"error"`
	Results []struct {
		Score      float64 `json:"score"`
		Recordings []struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Artists []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"artists"`
			ReleaseGroups []struct {
				ID       string `json:"id"`
				Title    string `json:"title"`
				Releases []struct {
					ID   string `json:"id"`
					Date struct {
						Year int `json:"year"`
					} `json:"date"`
					Mediums []struct {
						Position int `json:"position"`
						Tracks   []struct {
							Position int `json:"position"`
						} `json:"tracks"`
					} `json:"mediums"`
				} `json:"releases"`
			} `json:"releasegroups"`
		} `json:"recordings"`
	} `json:"results"`
}

// Lookup resolves a Chromaprint fingerprint to MusicBrainz recordings,
// ordered by score descending. Recordings without a MusicBrainz ID are
// dropped.
func (c *Client) Lookup(ctx context.Context, fingerprint string, duration float64) ([]Recording, error) {
	form := url.Values{
		"client":      {c.apiKey},
		"format":      {"json"},
		"fingerprint": {fingerprint},
		"duration":    {strconv.Itoa(int(duration))},
		"meta":        {"recordings releasegroups releases tracks artists"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.BaseURL+"/v2/lookup", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", c.UserAgent)
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		// A cancelled caller context is the caller's business, not a service
		// fault, so it is not relabelled as an upstream transport failure.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, &LookupError{Transport: true, Err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	var body lookupResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, &LookupError{Status: resp.StatusCode, Err: err}
	}
	if body.Status != "ok" {
		return nil, &LookupError{Status: resp.StatusCode, Message: body.Error.Message}
	}
	return flatten(body), nil
}

// flatten converts the nested AcoustID response (results > recordings >
// releasegroups > releases) into a flat recording list, carrying each
// result's score onto its recordings.
func flatten(body lookupResponse) []Recording {
	var out []Recording
	for _, res := range body.Results {
		for _, rec := range res.Recordings {
			if rec.ID == "" {
				continue
			}
			r := Recording{Score: res.Score, MBID: rec.ID, Title: rec.Title}
			for _, a := range rec.Artists {
				r.Artists = append(r.Artists, ArtistCredit{MBID: a.ID, Name: a.Name})
			}
			for _, rg := range rec.ReleaseGroups {
				for _, rel := range rg.Releases {
					out := Release{
						MBID:             rel.ID,
						ReleaseGroupMBID: rg.ID,
						Title:            rg.Title,
						Year:             rel.Date.Year,
					}
					// With meta=tracks the response carries only the medium
					// and track the recording sits on, so the first entry is
					// the recording's own position.
					if len(rel.Mediums) > 0 {
						m := rel.Mediums[0]
						out.DiscNumber = m.Position
						if len(m.Tracks) > 0 {
							out.TrackNumber = m.Tracks[0].Position
						}
					}
					r.Release = append(r.Release, out)
				}
			}
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}
