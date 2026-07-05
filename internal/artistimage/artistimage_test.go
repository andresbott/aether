package artistimage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"golang.org/x/time/rate"
)

func TestFanartTVFetch(t *testing.T) {
	mux := http.NewServeMux()
	var base string
	mux.HandleFunc("/v3/music/mbid-1", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"artistthumb":[{"url":"` + base + `/img.png","likes":"3"}]}`))
	})
	mux.HandleFunc("/img.png", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("PNGDATA"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	base = srv.URL
	p := NewFanartTV("key")
	p.BaseURL = srv.URL
	p.Client = srv.Client()
	p.limiter = rate.NewLimiter(rate.Inf, 1) // disable throttling for this logic test

	data, ext, err := p.Fetch(context.Background(), "mbid-1")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(data) != "PNGDATA" || ext != "png" {
		t.Fatalf("got data=%q ext=%q", data, ext)
	}
}

func TestEmptyKeySkips(t *testing.T) {
	p := NewFanartTV("")
	data, _, err := p.Fetch(context.Background(), "x")
	if err != nil || data != nil {
		t.Fatalf("empty key should skip, got data=%v err=%v", data, err)
	}
}

func TestChainFallsThrough(t *testing.T) {
	empty := stubProvider{}
	hit := stubProvider{data: []byte("X"), ext: "jpg"}
	c := NewChain(empty, hit)
	data, ext, err := c.Fetch(context.Background(), "m")
	if err != nil || string(data) != "X" || ext != "jpg" {
		t.Fatalf("chain: data=%q ext=%q err=%v", data, ext, err)
	}
}

func TestTheAudioDBFetch(t *testing.T) {
	mux := http.NewServeMux()
	var base string
	mux.HandleFunc("/api/v1/json/testkey/artist-mb.php", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"artists":[{"strArtistThumb":"` + base + `/img.jpg"}]}`))
	})
	mux.HandleFunc("/img.jpg", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("JPGDATA"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	base = srv.URL

	p := NewTheAudioDB("testkey")
	p.BaseURL = srv.URL
	p.Client = srv.Client()
	p.limiter = rate.NewLimiter(rate.Inf, 1) // disable throttling for this logic test

	data, ext, err := p.Fetch(context.Background(), "some-mbid")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(data) != "JPGDATA" || ext != "jpg" {
		t.Fatalf("got data=%q ext=%q", data, ext)
	}
}

func TestDownloadNon200(t *testing.T) {
	mux := http.NewServeMux()
	var base string
	// Metadata endpoint returns a thumb URL that 404s.
	mux.HandleFunc("/api/v1/json/testkey/artist-mb.php", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"artists":[{"strArtistThumb":"` + base + `/missing.jpg"}]}`))
	})
	mux.HandleFunc("/missing.jpg", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	base = srv.URL

	p := NewTheAudioDB("testkey")
	p.BaseURL = srv.URL
	p.Client = srv.Client()
	p.limiter = rate.NewLimiter(rate.Inf, 1) // disable throttling for this logic test

	_, _, err := p.Fetch(context.Background(), "some-mbid")
	if err == nil {
		t.Fatal("expected error when image URL returns 404, got nil")
	}
}

// TestProviderThrottleGatesRequest verifies the fair-use throttle is applied
// before the outbound request: with a limiter that never grants a token, Fetch
// returns an error and no HTTP request reaches the server.
func TestProviderThrottleGatesRequest(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	p := NewFanartTV("key")
	p.BaseURL = srv.URL
	p.Client = srv.Client()
	p.limiter = rate.NewLimiter(1, 0) // burst 0 -> Wait can never succeed

	_, _, err := p.Fetch(context.Background(), "mbid-1")
	if err == nil {
		t.Fatal("expected the throttle to block the request and return an error")
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Fatalf("request reached the server despite the throttle: %d hits", n)
	}
}

func TestChainErrorContinuation(t *testing.T) {
	failing := stubProvider{err: errors.New("provider error")}
	hit := stubProvider{data: []byte("X"), ext: "jpg"}
	c := NewChain(failing, hit)
	data, ext, err := c.Fetch(context.Background(), "m")
	if err != nil || string(data) != "X" || ext != "jpg" {
		t.Fatalf("chain should continue past error: data=%q ext=%q err=%v", data, ext, err)
	}
}

type stubProvider struct {
	data []byte
	ext  string
	err  error
}

func (s stubProvider) Name() string { return "stub" }
func (s stubProvider) Fetch(_ context.Context, _ string) ([]byte, string, error) {
	return s.data, s.ext, s.err
}
