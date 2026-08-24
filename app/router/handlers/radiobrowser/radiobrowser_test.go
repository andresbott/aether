package radiobrowser_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andresbott/aether/app/router/handlers/httperr"
	rbHandler "github.com/andresbott/aether/app/router/handlers/radiobrowser"
	"github.com/andresbott/aether/internal/radiobrowser"
	"github.com/andresbott/aether/internal/upstream"
	"github.com/gorilla/mux"
)

type fakeSearcher struct {
	results     []radiobrowser.Station
	searchErr   error
	gotLimit    int
	faviconData []byte
	faviconType string
	faviconErr  error
}

func (f *fakeSearcher) Search(_ context.Context, _ string, limit int) ([]radiobrowser.Station, error) {
	f.gotLimit = limit
	return f.results, f.searchErr
}

func (f *fakeSearcher) FetchFavicon(_ context.Context, _ string) ([]byte, string, error) {
	return f.faviconData, f.faviconType, f.faviconErr
}

func newRouter(search rbHandler.Searcher) *mux.Router {
	h := &rbHandler.Handler{Client: search}
	r := mux.NewRouter()
	h.Routes(r)
	return r
}

func TestSearch_ReturnsResults(t *testing.T) {
	f := &fakeSearcher{results: []radiobrowser.Station{{Name: "BBC Radio 1", StreamURL: "http://bbc/stream"}}}
	r := newRouter(f)

	req := httptest.NewRequest(http.MethodGet, "/radiobrowser/search?q=BBC", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got []radiobrowser.Station
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "BBC Radio 1" {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
	if f.gotLimit != 10 {
		t.Fatalf("expected default limit 10, got %d", f.gotLimit)
	}
}

func TestSearch_BlankQuery(t *testing.T) {
	r := newRouter(&fakeSearcher{})
	req := httptest.NewRequest(http.MethodGet, "/radiobrowser/search?q=", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSearch_InvalidLimit(t *testing.T) {
	r := newRouter(&fakeSearcher{})
	req := httptest.NewRequest(http.MethodGet, "/radiobrowser/search?q=BBC&limit=nope", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSearch_LimitCapped(t *testing.T) {
	f := &fakeSearcher{}
	r := newRouter(f)
	req := httptest.NewRequest(http.MethodGet, "/radiobrowser/search?q=BBC&limit=500", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if f.gotLimit != 25 {
		t.Fatalf("expected limit capped to 25, got %d", f.gotLimit)
	}
}

func TestSearch_UpstreamError(t *testing.T) {
	r := newRouter(&fakeSearcher{searchErr: errors.New("boom")})
	req := httptest.NewRequest(http.MethodGet, "/radiobrowser/search?q=BBC", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
}

// The station directory's outages get the same human treatment.
func TestSearch_UpstreamErrorIsHumanReadable(t *testing.T) {
	r := newRouter(&fakeSearcher{searchErr: &upstream.Error{
		Service: "radio-browser.info",
		Kind:    upstream.KindUnavailable,
		Status:  http.StatusServiceUnavailable,
	}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/radiobrowser/search?q=BBC", nil))

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	var body httperr.Problem
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if got := httperr.Slug(body.Type); got != "upstream_error" {
		t.Errorf("code = %q, want upstream_error", got)
	}
	if !strings.Contains(body.Detail, "radio-browser.info") {
		t.Errorf("error does not name the service: %q", body.Detail)
	}
	if strings.Contains(body.Detail, "search failed") {
		t.Errorf("error leaks internal wording: %q", body.Detail)
	}
}

func TestFavicon_ReturnsBytes(t *testing.T) {
	f := &fakeSearcher{faviconData: []byte{0x89, 'P', 'N', 'G'}, faviconType: "image/png"}
	r := newRouter(f)

	req := httptest.NewRequest(http.MethodGet, "/radiobrowser/favicon?url=https://x/f.png", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("expected image/png, got %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc == "" {
		t.Fatalf("expected a Cache-Control header")
	}
	if w.Body.Len() != 4 {
		t.Fatalf("expected 4 bytes, got %d", w.Body.Len())
	}
}

func TestFavicon_MissingURL(t *testing.T) {
	r := newRouter(&fakeSearcher{})
	req := httptest.NewRequest(http.MethodGet, "/radiobrowser/favicon", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestFavicon_UpstreamError(t *testing.T) {
	r := newRouter(&fakeSearcher{faviconErr: errors.New("not an image")})
	req := httptest.NewRequest(http.MethodGet, "/radiobrowser/favicon?url=https://x/f.ico", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
}
