package artistimage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestFanartTVList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v3/music/mbid-1", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"artistthumb":[
			{"url":"https://assets.fanart.tv/fanart/music/mbid-1/artistthumb/a.jpg"},
			{"url":"https://assets.fanart.tv/fanart/music/mbid-1/artistthumb/b.png"},
			{"url":""}
		]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := NewFanartTV("key")
	p.BaseURL = srv.URL
	p.Doer.Client = srv.Client()
	p.Doer.Limiter = rate.NewLimiter(rate.Inf, 1)
	p.Doer.Wait = func(context.Context, time.Duration) error { return nil }

	got, err := p.List(context.Background(), "mbid-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 candidates (empty URL skipped), got %d: %+v", len(got), got)
	}
	if got[0].FullURL != "https://assets.fanart.tv/fanart/music/mbid-1/artistthumb/a.jpg" {
		t.Fatalf("FullURL[0]=%q", got[0].FullURL)
	}
	if got[0].ThumbURL != "https://assets.fanart.tv/preview/music/mbid-1/artistthumb/a.jpg" {
		t.Fatalf("ThumbURL[0]=%q (preview derivation wrong)", got[0].ThumbURL)
	}
	if got[0].Provider != "fanart.tv" {
		t.Fatalf("Provider[0]=%q", got[0].Provider)
	}
}

// FanartTV.List must return nil, not a non-nil empty slice, when artistthumb
// is empty — matching TheAudioDB.List and the Provider.List doc comment.
func TestFanartTVListNoCandidatesIsNil(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v3/music/mbid-1", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"artistthumb":[]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := NewFanartTV("key")
	p.BaseURL = srv.URL
	p.Doer.Client = srv.Client()
	p.Doer.Limiter = rate.NewLimiter(rate.Inf, 1)
	p.Doer.Wait = func(context.Context, time.Duration) error { return nil }

	got, err := p.List(context.Background(), "mbid-1")
	if err != nil || got != nil {
		t.Fatalf("expected nil candidates when artistthumb is empty, got %+v err=%v", got, err)
	}
}

func TestEmptyKeySkips(t *testing.T) {
	p := NewFanartTV("")
	cands, err := p.List(context.Background(), "x")
	if err != nil || cands != nil {
		t.Fatalf("empty key should skip, got cands=%v err=%v", cands, err)
	}
}

func TestChainListAggregatesInOrder(t *testing.T) {
	a := stubProvider{name: "A", cands: []ImageCandidate{{FullURL: "a1", Provider: "A"}}}
	b := stubProvider{name: "B", cands: []ImageCandidate{{FullURL: "b1", Provider: "B"}}}
	c := NewChain(a, b)
	got, err := c.List(context.Background(), "m")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 || got[0].FullURL != "a1" || got[1].FullURL != "b1" {
		t.Fatalf("expected [a1 b1] in provider order, got %+v", got)
	}
}

func TestChainListErrorOnlyWhenEmpty(t *testing.T) {
	// One provider errors, the other returns a candidate: the candidate wins.
	failing := stubProvider{name: "fail", err: errors.New("boom")}
	hit := stubProvider{name: "hit", cands: []ImageCandidate{{FullURL: "u", Provider: "hit"}}}
	if got, err := NewChain(failing, hit).List(context.Background(), "m"); err != nil || len(got) != 1 {
		t.Fatalf("partial success: got=%+v err=%v", got, err)
	}
	// Every provider errors and none has candidates: surface the error.
	if _, err := NewChain(failing).List(context.Background(), "m"); err == nil {
		t.Fatal("expected the provider error to surface when the aggregate is empty")
	}
}

func TestChainDownloadRoutesByProvider(t *testing.T) {
	a := stubProvider{name: "A", data: []byte("AAA"), ext: "jpg"}
	b := stubProvider{name: "B", data: []byte("BBB"), ext: "png"}
	data, ext, err := NewChain(a, b).Download(context.Background(), "B", "b1")
	if err != nil || string(data) != "BBB" || ext != "png" {
		t.Fatalf("routing: data=%q ext=%q err=%v", data, ext, err)
	}
}

// A provider name that matches nothing in the chain (e.g. a stale/forged name
// from a client) must error rather than silently return no bytes — the caller
// (setImageFromSearch) needs to tell "download failed" from "nothing to do".
func TestChainDownloadUnknownProviderErrors(t *testing.T) {
	a := stubProvider{name: "A", data: []byte("AAA"), ext: "jpg"}
	data, ext, err := NewChain(a).Download(context.Background(), "does-not-exist", "u")
	if err == nil {
		t.Fatal("expected an error when no provider matches the given name")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Fatalf("expected the error to name the unmatched provider, got %q", err.Error())
	}
	if data != nil || ext != "" {
		t.Fatalf("expected no data/ext alongside the error, got data=%q ext=%q", data, ext)
	}
}

func TestChainFetchDownloadsFirstCandidate(t *testing.T) {
	empty := stubProvider{name: "empty"}
	hit := stubProvider{name: "hit", cands: []ImageCandidate{{FullURL: "u", Provider: "hit"}}, data: []byte("X"), ext: "jpg"}
	data, ext, err := NewChain(empty, hit).Fetch(context.Background(), "m")
	if err != nil || string(data) != "X" || ext != "jpg" {
		t.Fatalf("fetch: data=%q ext=%q err=%v", data, ext, err)
	}
}

// If the first candidate's image download fails — even though its provider
// listed successfully — Fetch must fall through to the next candidate,
// possibly from a different provider, instead of giving up for the run.
func TestChainFetchFallsThroughOnDownloadError(t *testing.T) {
	a := stubProvider{name: "A", cands: []ImageCandidate{{FullURL: "a", Provider: "A"}}, downloadErr: errors.New("download failed")}
	b := stubProvider{name: "B", cands: []ImageCandidate{{FullURL: "b", Provider: "B"}}, data: []byte("B"), ext: "jpg"}
	data, ext, err := NewChain(a, b).Fetch(context.Background(), "m")
	if err != nil || string(data) != "B" || ext != "jpg" {
		t.Fatalf("fetch: data=%q ext=%q err=%v", data, ext, err)
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
	p.Doer.Client = srv.Client()
	p.Doer.Limiter = rate.NewLimiter(rate.Inf, 1) // disable throttling for this logic test
	p.Doer.Wait = func(context.Context, time.Duration) error { return nil }

	cands, err := p.List(context.Background(), "some-mbid")
	if err != nil || len(cands) != 1 {
		t.Fatalf("List: cands=%+v err=%v", cands, err)
	}
	if _, _, err := p.Download(context.Background(), cands[0].FullURL); err == nil {
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
	p.Doer.Client = srv.Client()
	p.Doer.Limiter = rate.NewLimiter(1, 0) // burst 0 -> Wait can never succeed

	_, err := p.List(context.Background(), "mbid-1")
	if err == nil {
		t.Fatal("expected the throttle to block the request and return an error")
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Fatalf("request reached the server despite the throttle: %d hits", n)
	}
}

// An image host that fails once must not cost us the artist image: the
// provider retries before the chain writes it off as "no image here".
func TestProviderRetriesTransientFailure(t *testing.T) {
	var imgHits int32
	mux := http.NewServeMux()
	var base string
	mux.HandleFunc("/v3/music/mbid-1", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"artistthumb":[{"url":"` + base + `/img.png"}]}`))
	})
	mux.HandleFunc("/img.png", func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&imgHits, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("PNGDATA"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	base = srv.URL

	p := NewFanartTV("key")
	p.BaseURL = srv.URL
	p.Doer.Client = srv.Client()
	p.Doer.Limiter = rate.NewLimiter(rate.Inf, 1)
	p.Doer.Wait = func(context.Context, time.Duration) error { return nil }

	cands, err := p.List(context.Background(), "mbid-1")
	if err != nil || len(cands) != 1 {
		t.Fatalf("List: cands=%+v err=%v", cands, err)
	}
	data, ext, err := p.Download(context.Background(), cands[0].FullURL)
	if err != nil {
		t.Fatalf("Download should have recovered on retry: %v", err)
	}
	if string(data) != "PNGDATA" || ext != "png" {
		t.Fatalf("got data=%q ext=%q", data, ext)
	}
}

func TestProviderDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("PNGDATA"))
	}))
	defer srv.Close()

	p := NewFanartTV("key")
	p.Doer.Client = srv.Client()
	p.Doer.Limiter = rate.NewLimiter(rate.Inf, 1)
	p.Doer.Wait = func(context.Context, time.Duration) error { return nil }

	data, ext, err := p.Download(context.Background(), srv.URL+"/img.png")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if string(data) != "PNGDATA" || ext != "png" {
		t.Fatalf("got data=%q ext=%q", data, ext)
	}
}

type stubProvider struct {
	name        string
	cands       []ImageCandidate
	data        []byte
	ext         string
	err         error
	downloadErr error
}

func (s stubProvider) Name() string {
	if s.name != "" {
		return s.name
	}
	return "stub"
}
func (s stubProvider) List(_ context.Context, _ string) ([]ImageCandidate, error) {
	return s.cands, s.err
}
func (s stubProvider) Download(_ context.Context, _ string) ([]byte, string, error) {
	return s.data, s.ext, s.downloadErr
}
