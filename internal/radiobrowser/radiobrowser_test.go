package radiobrowser

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/upstream"
	"golang.org/x/time/rate"
)

// testClient points a Client at a test server with the rate limiter and retry
// backoff disabled.
func testClient(baseURL string, hc *http.Client) *Client {
	c := New("Aether/test (https://example.com)")
	c.BaseURL = baseURL
	c.Doer.Client = hc
	c.Doer.Limiter = rate.NewLimiter(rate.Inf, 1)
	c.Doer.Wait = func(context.Context, time.Duration) error { return nil }
	return c
}

// The radio-browser mirror pool sheds load under pressure; a 503 that clears on
// retry must not surface as a failed station search.
func TestSearchRetriesTransientFailure(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`[{"name":"BBC Radio 1","url":"http://s/1"}]`))
	}))
	defer srv.Close()

	got, err := testClient(srv.URL, srv.Client()).Search(context.Background(), "BBC", 10)
	if err != nil {
		t.Fatalf("Search should have recovered on retry: %v", err)
	}
	if len(got) != 1 || got[0].Name != "BBC Radio 1" {
		t.Fatalf("unexpected results: %+v", got)
	}
}

// A persistent failure is typed, so the handler can render a sentence naming
// the directory rather than "status 503".
func TestSearchFailureIsTypedUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, err := testClient(srv.URL, srv.Client()).Search(context.Background(), "BBC", 10)
	var uerr *upstream.Error
	if !errors.As(err, &uerr) {
		t.Fatalf("want *upstream.Error, got %T: %v", err, err)
	}
	if msg := uerr.UserMessage(); !strings.Contains(msg, "radio-browser.info") {
		t.Fatalf("message does not name the service: %q", msg)
	}
}

func TestSearchParsesResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "Aether/test (https://example.com)" {
			t.Errorf("unexpected User-Agent: %q", got)
		}
		if !strings.Contains(r.URL.Path, "/json/stations/search") {
			t.Errorf("expected search endpoint, got %q", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("name") != "BBC" {
			t.Errorf("expected name=BBC, got %q", q.Get("name"))
		}
		if q.Get("hidebroken") != "true" || q.Get("order") != "votes" || q.Get("reverse") != "true" {
			t.Errorf("unexpected query params: %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[
			{
				"name": "BBC Radio 1",
				"url": "http://bbc/stream",
				"url_resolved": "http://bbc/stream/resolved",
				"homepage": "https://bbc.co.uk",
				"favicon": "https://bbc.co.uk/favicon.png",
				"tags": "pop,uk",
				"country": "United Kingdom",
				"countrycode": "GB",
				"language": "english",
				"codec": "MP3",
				"bitrate": 128,
				"votes": 4200,
				"stationuuid": "uuid-1"
			},
			{
				"name": "No Resolved",
				"url": "http://plain/stream",
				"url_resolved": "",
				"stationuuid": "uuid-2"
			}
		]`))
	}))
	defer srv.Close()

	c := testClient(srv.URL, srv.Client())
	results, err := c.Search(context.Background(), "BBC", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	want := Station{
		Name:        "BBC Radio 1",
		StreamURL:   "http://bbc/stream/resolved",
		Homepage:    "https://bbc.co.uk",
		Favicon:     "https://bbc.co.uk/favicon.png",
		Tags:        "pop,uk",
		Country:     "United Kingdom",
		CountryCode: "GB",
		Language:    "english",
		Codec:       "MP3",
		Bitrate:     128,
		Votes:       4200,
		UUID:        "uuid-1",
	}
	if results[0] != want {
		t.Fatalf("got %+v, want %+v", results[0], want)
	}
	// url_resolved empty falls back to url.
	if results[1].StreamURL != "http://plain/stream" {
		t.Fatalf("expected fallback to url, got %q", results[1].StreamURL)
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	c := New("Aether/test")
	results, err := c.Search(context.Background(), "   ", 10)
	if err != nil || results != nil {
		t.Fatalf("expected nil, nil for empty query, got %v, %v", results, err)
	}
}

func TestSearchUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := testClient(srv.URL, srv.Client())
	if _, err := c.Search(context.Background(), "BBC", 10); err == nil {
		t.Fatal("expected an error for a non-200 upstream response")
	}
}

// pngHeader is the PNG magic signature http.DetectContentType keys on.
var pngHeader = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

func TestFetchFaviconPNG(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Lie about the content type on purpose; the client must sniff bytes.
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(append(append([]byte{}, pngHeader...), []byte("padding")...))
	}))
	defer srv.Close()

	c := testClient(srv.URL, srv.Client())
	data, ct, err := c.FetchFavicon(context.Background(), srv.URL+"/favicon.png")
	if err != nil {
		t.Fatalf("FetchFavicon: %v", err)
	}
	if ct != "image/png" {
		t.Fatalf("expected image/png, got %q", ct)
	}
	if !bytes.HasPrefix(data, pngHeader) {
		t.Fatalf("returned bytes do not start with PNG header")
	}
}

func TestFetchFaviconRejectsNonImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("this is definitely not an image"))
	}))
	defer srv.Close()

	c := testClient(srv.URL, srv.Client())
	if _, _, err := c.FetchFavicon(context.Background(), srv.URL+"/favicon.ico"); err == nil {
		t.Fatal("expected an error for a non-image favicon")
	}
}

func TestFetchFaviconRejectsBadScheme(t *testing.T) {
	c := New("Aether/test")
	if _, _, err := c.FetchFavicon(context.Background(), "ftp://example.com/favicon.png"); err == nil {
		t.Fatal("expected an error for a non-http(s) scheme")
	}
	if _, _, err := c.FetchFavicon(context.Background(), ""); err == nil {
		t.Fatal("expected an error for an empty url")
	}
}

func TestFetchFaviconUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := testClient(srv.URL, srv.Client())
	if _, _, err := c.FetchFavicon(context.Background(), srv.URL+"/missing.png"); err == nil {
		t.Fatal("expected an error for a 404 favicon")
	}
}

func TestFetchFaviconRejectsOversized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := append(append([]byte{}, pngHeader...), bytes.Repeat([]byte{0}, maxFaviconBytes+1)...)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := testClient(srv.URL, srv.Client())
	if _, _, err := c.FetchFavicon(context.Background(), srv.URL+"/big.png"); err == nil {
		t.Fatal("expected an error for an oversized favicon")
	}
}
