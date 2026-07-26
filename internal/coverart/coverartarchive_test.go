package coverart

import (
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

// newTestClient points a client at a test server with throttling and retry
// backoff disabled, so the logic under test runs at full speed.
func newTestClient(baseURL string) *Client {
	c := New("Aether/test")
	c.BaseURL = baseURL
	c.Doer.Limiter = rate.NewLimiter(rate.Inf, 1)
	c.Doer.Wait = func(context.Context, time.Duration) error { return nil }
	return c
}

func TestListRelease(t *testing.T) {
	const releaseJSON = `{
		"images": [
			{"id": "111", "image": "http://img/front.jpg", "front": true, "types": ["Front"], "comment": "digipak",
			 "thumbnails": {"250": "http://img/front-250.jpg", "500": "http://img/front-500.jpg"}},
			{"id": "222", "image": "http://img/back.png", "front": false, "types": ["Back", "Booklet"], "thumbnails": {}}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/release/rel-mbid" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(releaseJSON))
	}))
	defer srv.Close()

	imgs, err := newTestClient(srv.URL).List(context.Background(), "rel-mbid", "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(imgs) != 2 {
		t.Fatalf("want 2 images, got %d", len(imgs))
	}
	if imgs[0].ID != "111" || !imgs[0].IsFront {
		t.Errorf("unexpected front image: %+v", imgs[0])
	}
	if imgs[0].ThumbURL != "http://img/front-250.jpg" {
		t.Errorf("want 250 thumbnail, got %q", imgs[0].ThumbURL)
	}
	if len(imgs[0].Types) != 1 || imgs[0].Types[0] != "Front" || imgs[0].Comment != "digipak" {
		t.Errorf("unexpected types/comment on front image: %+v", imgs[0])
	}
	if len(imgs[1].Types) != 2 || imgs[1].Types[1] != "Booklet" {
		t.Errorf("unexpected types on back image: %+v", imgs[1])
	}
	// no thumbnails -> falls back to the full image
	if imgs[1].ThumbURL != "http://img/back.png" {
		t.Errorf("want full image fallback, got %q", imgs[1].ThumbURL)
	}
}

func TestListReleaseGroupFallback(t *testing.T) {
	var releaseHits, groupHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/release/rel-mbid":
			releaseHits++
			http.NotFound(w, r) // no cover for the release
		case "/release-group/grp-mbid":
			groupHits++
			_, _ = w.Write([]byte(`{"images":[{"id":"9","image":"http://img/g.jpg","front":true}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	imgs, err := newTestClient(srv.URL).List(context.Background(), "rel-mbid", "grp-mbid")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if releaseHits != 1 || groupHits != 1 {
		t.Fatalf("want release+group each hit once, got release=%d group=%d", releaseHits, groupHits)
	}
	if len(imgs) != 1 || imgs[0].ImageURL != "http://img/g.jpg" {
		t.Fatalf("unexpected fallback images: %+v", imgs)
	}
}

func TestListEmptyMBIDs(t *testing.T) {
	imgs, err := newTestClient("http://unused").List(context.Background(), "", "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if imgs != nil {
		t.Fatalf("want nil, got %+v", imgs)
	}
}

// A release lookup that stays broken must not sink the whole request when a
// release-group MBID is also known — the group is a valid answer for the album.
func TestListFallsBackToGroupWhenReleaseFails(t *testing.T) {
	var groupHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/release/rel-mbid":
			w.WriteHeader(http.StatusInternalServerError)
		case "/release-group/grp-mbid":
			atomic.AddInt32(&groupHits, 1)
			_, _ = w.Write([]byte(`{"images":[{"id":"9","image":"http://img/g.jpg","front":true}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	imgs, err := newTestClient(srv.URL).List(context.Background(), "rel-mbid", "grp-mbid")
	if err != nil {
		t.Fatalf("List should recover via the release group: %v", err)
	}
	if len(imgs) != 1 || imgs[0].ImageURL != "http://img/g.jpg" {
		t.Fatalf("unexpected images: %+v", imgs)
	}
	if n := atomic.LoadInt32(&groupHits); n != 1 {
		t.Fatalf("release-group lookup ran %d times, want 1", n)
	}
}

// With no group to fall back to, the release failure surfaces — as a typed
// upstream error carrying a human-readable message.
func TestListReleaseFailureIsTypedUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).List(context.Background(), "rel-mbid", "")
	var uerr *upstream.Error
	if !errors.As(err, &uerr) {
		t.Fatalf("want *upstream.Error, got %T: %v", err, err)
	}
	if uerr.Service != "Cover Art Archive" {
		t.Fatalf("service = %q", uerr.Service)
	}
	if msg := uerr.UserMessage(); !strings.Contains(msg, "Cover Art Archive") {
		t.Fatalf("unhelpful message: %q", msg)
	}
}

// When both lookups fail the group error is what the caller sees; it must stay
// typed rather than degrading to a bare string.
func TestListBothFailuresReturnTypedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).List(context.Background(), "rel-mbid", "grp-mbid")
	var uerr *upstream.Error
	if !errors.As(err, &uerr) {
		t.Fatalf("want *upstream.Error, got %T: %v", err, err)
	}
	if uerr.Kind != upstream.KindUnavailable {
		t.Fatalf("kind = %v, want unavailable", uerr.Kind)
	}
}

func TestDownloadImageFailureIsTypedUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, _, err := newTestClient(srv.URL).DownloadImage(context.Background(), srv.URL+"/cover.jpg")
	var uerr *upstream.Error
	if !errors.As(err, &uerr) {
		t.Fatalf("want *upstream.Error, got %T: %v", err, err)
	}
}

func TestDownloadImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("PNGDATA"))
	}))
	defer srv.Close()

	data, ext, err := newTestClient(srv.URL).DownloadImage(context.Background(), srv.URL+"/cover.png")
	if err != nil {
		t.Fatalf("DownloadImage: %v", err)
	}
	if string(data) != "PNGDATA" {
		t.Errorf("unexpected data %q", data)
	}
	if ext != "png" {
		t.Errorf("want png ext, got %q", ext)
	}
}
