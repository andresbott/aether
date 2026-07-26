package artists_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	artistsHandler "github.com/andresbott/aether/app/router/handlers/artists"
	"github.com/andresbott/aether/app/tasks"
	"github.com/andresbott/aether/internal/artistimage"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/upstream"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type fakeSearcher struct {
	results        []artistimage.Candidate
	releaseResults []artistimage.ReleaseCandidate
	genres         []string
	err            error
}

func (f *fakeSearcher) Search(_ context.Context, _ string, _ int) ([]artistimage.Candidate, error) {
	return f.results, f.err
}

func (f *fakeSearcher) SearchRelease(_ context.Context, _ string, _ int) ([]artistimage.ReleaseCandidate, error) {
	return f.releaseResults, f.err
}

func (f *fakeSearcher) ReleaseGroupGenres(_ context.Context, _ string) ([]string, error) {
	return f.genres, f.err
}

type fakeFetcher struct {
	calls int
	data  []byte
	ext   string
	err   error
}

func (f *fakeFetcher) Fetch(_ context.Context, _ string) ([]byte, string, error) {
	f.calls++
	return f.data, f.ext, f.err
}

func newTestHandler(t *testing.T, search artistsHandler.Searcher, fetcher tasks.Fetcher) (*store.Store, *mux.Router) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := store.New(db)
	as := assetstore.New(t.TempDir())
	h := &artistsHandler.Handler{Store: s, Assets: as, Fetcher: fetcher, Search: search}
	r := mux.NewRouter()
	h.Routes(r)
	return s, r
}

func TestSearchMusicBrainz_ReturnsResults(t *testing.T) {
	search := &fakeSearcher{results: []artistimage.Candidate{{MBID: "abc", Name: "Nirvana", Score: 100}}}
	_, r := newTestHandler(t, search, nil)

	req := httptest.NewRequest(http.MethodGet, "/musicbrainz/search?q=Nirvana", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got []artistimage.Candidate
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Nirvana" {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestSearchMusicBrainz_BlankQuery(t *testing.T) {
	_, r := newTestHandler(t, &fakeSearcher{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/musicbrainz/search?q=", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSearchMusicBrainz_UpstreamError(t *testing.T) {
	search := &fakeSearcher{err: errors.New("upstream down")}
	_, r := newTestHandler(t, search, nil)
	req := httptest.NewRequest(http.MethodGet, "/musicbrainz/search?q=Nirvana", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
}

// An upstream outage must reach the UI as a sentence naming MusicBrainz, with
// a rate limit distinguished from an outage so the UI can say "wait and retry".
func TestSearchMusicBrainz_UpstreamErrorIsHumanReadable(t *testing.T) {
	search := &fakeSearcher{err: &upstream.Error{
		Service: "MusicBrainz",
		Kind:    upstream.KindUnavailable,
		Status:  http.StatusServiceUnavailable,
	}}
	_, r := newTestHandler(t, search, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/musicbrainz/search?q=Nirvana", nil))

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
	var body struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Code != "upstream_error" {
		t.Errorf("code = %q, want upstream_error", body.Code)
	}
	if !strings.Contains(body.Error, "MusicBrainz") || !strings.Contains(body.Error, "unavailable") {
		t.Errorf("error is not a human sentence: %q", body.Error)
	}
	if strings.Contains(body.Error, "search failed") || strings.Contains(body.Error, "status") {
		t.Errorf("error leaks internal wording: %q", body.Error)
	}
}

func TestSearchMusicBrainz_RateLimitedReturns429(t *testing.T) {
	search := &fakeSearcher{err: &upstream.Error{
		Service: "MusicBrainz",
		Kind:    upstream.KindRateLimited,
		Status:  http.StatusTooManyRequests,
	}}
	_, r := newTestHandler(t, search, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/musicbrainz/search?q=Nirvana", nil))

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	var body struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Code != "upstream_rate_limited" {
		t.Errorf("code = %q, want upstream_rate_limited", body.Code)
	}
}

func TestReleaseGroupGenres_ReturnsGenres(t *testing.T) {
	search := &fakeSearcher{genres: []string{"alternative rock", "art rock"}}
	_, r := newTestHandler(t, search, nil)

	req := httptest.NewRequest(http.MethodGet,
		"/musicbrainz/release-groups/0b6b4884-f8f0-3f47-a992-3730c2a477c9/genres", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got []string
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "alternative rock" {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestReleaseGroupGenres_EmptyIsJSONArray(t *testing.T) {
	_, r := newTestHandler(t, &fakeSearcher{}, nil)
	req := httptest.NewRequest(http.MethodGet,
		"/musicbrainz/release-groups/0b6b4884-f8f0-3f47-a992-3730c2a477c9/genres", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if body := strings.TrimSpace(w.Body.String()); body != "[]" {
		t.Fatalf("expected [], got %q", body)
	}
}

func TestReleaseGroupGenres_InvalidMBID(t *testing.T) {
	_, r := newTestHandler(t, &fakeSearcher{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/musicbrainz/release-groups/not-a-uuid/genres", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestReleaseGroupGenres_UpstreamError(t *testing.T) {
	search := &fakeSearcher{err: errors.New("upstream down")}
	_, r := newTestHandler(t, search, nil)
	req := httptest.NewRequest(http.MethodGet,
		"/musicbrainz/release-groups/0b6b4884-f8f0-3f47-a992-3730c2a477c9/genres", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSearchMusicBrainzReleases_ReturnsResults(t *testing.T) {
	search := &fakeSearcher{releaseResults: []artistimage.ReleaseCandidate{
		{ReleaseMBID: "rel-1", ReleaseGroupMBID: "rg-1", Title: "OK Computer", Score: 100},
	}}
	_, r := newTestHandler(t, search, nil)

	req := httptest.NewRequest(http.MethodGet, "/musicbrainz/search/releases?q=OK+Computer", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got []artistimage.ReleaseCandidate
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "OK Computer" || got[0].ReleaseGroupMBID != "rg-1" {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestSearchMusicBrainzReleases_BlankQuery(t *testing.T) {
	_, r := newTestHandler(t, &fakeSearcher{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/musicbrainz/search/releases?q=", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSearchMusicBrainzReleases_UpstreamError(t *testing.T) {
	search := &fakeSearcher{err: errors.New("upstream down")}
	_, r := newTestHandler(t, search, nil)
	req := httptest.NewRequest(http.MethodGet, "/musicbrainz/search/releases?q=OK+Computer", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetMBID_SetsAndFetchesImage(t *testing.T) {
	s, r := newTestHandler(t, &fakeSearcher{}, &fakeFetcher{data: []byte("IMG"), ext: "jpg"})
	artists, err := s.FindOrCreateArtists([]string{"Nirvana"}, []string{""})
	if err != nil {
		t.Fatal(err)
	}
	id := artists[0].ID

	body, _ := json.Marshal(map[string]string{"mbid": "5b11f4ce-a62d-471e-81fc-a69a8278c7da"})
	req := httptest.NewRequest(http.MethodPut, "/artists/"+strconv.FormatUint(uint64(id), 10)+"/mbid", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got struct {
		MBArtistID   string  `json:"mbArtistId"`
		ImageFetched bool    `json:"imageFetched"`
		FetchError   *string `json:"fetchError"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.MBArtistID != "5b11f4ce-a62d-471e-81fc-a69a8278c7da" {
		t.Fatalf("unexpected mbArtistId: %+v", got)
	}
	if !got.ImageFetched {
		t.Fatalf("expected image to be fetched: %+v", got)
	}
	if got.FetchError != nil {
		t.Fatalf("expected no fetch error, got %q", *got.FetchError)
	}
}

func TestSetMBID_ClearSkipsFetch(t *testing.T) {
	fetcher := &fakeFetcher{data: []byte("IMG"), ext: "jpg"}
	s, r := newTestHandler(t, &fakeSearcher{}, fetcher)
	artists, err := s.FindOrCreateArtists([]string{"Nirvana"}, []string{"old-mbid"})
	if err != nil {
		t.Fatal(err)
	}
	id := artists[0].ID

	body, _ := json.Marshal(map[string]string{"mbid": ""})
	req := httptest.NewRequest(http.MethodPut, "/artists/"+strconv.FormatUint(uint64(id), 10)+"/mbid", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if fetcher.calls != 0 {
		t.Fatalf("expected no fetch attempt when clearing, got %d calls", fetcher.calls)
	}
}

func TestSetMBID_InvalidUUID(t *testing.T) {
	s, r := newTestHandler(t, &fakeSearcher{}, nil)
	artists, err := s.FindOrCreateArtists([]string{"Nirvana"}, []string{""})
	if err != nil {
		t.Fatal(err)
	}
	id := artists[0].ID

	body, _ := json.Marshal(map[string]string{"mbid": "not-a-uuid"})
	req := httptest.NewRequest(http.MethodPut, "/artists/"+strconv.FormatUint(uint64(id), 10)+"/mbid", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSetMBID_UnknownArtist404(t *testing.T) {
	_, r := newTestHandler(t, &fakeSearcher{}, nil)
	body, _ := json.Marshal(map[string]string{"mbid": ""})
	req := httptest.NewRequest(http.MethodPut, "/artists/999/mbid", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSetMBID_FetchFailureStillSavesMbid(t *testing.T) {
	fetcher := &fakeFetcher{err: errors.New("provider down")}
	s, r := newTestHandler(t, &fakeSearcher{}, fetcher)
	artists, err := s.FindOrCreateArtists([]string{"Nirvana"}, []string{""})
	if err != nil {
		t.Fatal(err)
	}
	id := artists[0].ID

	body, _ := json.Marshal(map[string]string{"mbid": "5b11f4ce-a62d-471e-81fc-a69a8278c7da"})
	req := httptest.NewRequest(http.MethodPut, "/artists/"+strconv.FormatUint(uint64(id), 10)+"/mbid", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (mbid save succeeds even if fetch fails), got %d: %s", w.Code, w.Body.String())
	}
	var got struct {
		MBArtistID string  `json:"mbArtistId"`
		FetchError *string `json:"fetchError"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.MBArtistID != "5b11f4ce-a62d-471e-81fc-a69a8278c7da" {
		t.Fatalf("expected mbid saved despite fetch failure, got %+v", got)
	}
	if got.FetchError == nil {
		t.Fatal("expected fetchError to be populated")
	}
}

func TestSetMBID_NoImageFoundSetsFetchError(t *testing.T) {
	fetcher := &fakeFetcher{}
	s, r := newTestHandler(t, &fakeSearcher{}, fetcher)
	artists, err := s.FindOrCreateArtists([]string{"Nirvana"}, []string{""})
	if err != nil {
		t.Fatal(err)
	}
	id := artists[0].ID

	body, _ := json.Marshal(map[string]string{"mbid": "5b11f4ce-a62d-471e-81fc-a69a8278c7da"})
	req := httptest.NewRequest(http.MethodPut, "/artists/"+strconv.FormatUint(uint64(id), 10)+"/mbid", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got struct {
		MBArtistID   string  `json:"mbArtistId"`
		ImageFetched bool    `json:"imageFetched"`
		FetchError   *string `json:"fetchError"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ImageFetched {
		t.Fatalf("expected image not fetched: %+v", got)
	}
	if got.FetchError == nil || !strings.Contains(*got.FetchError, "no image found") {
		t.Fatalf("expected fetchError mentioning 'no image found', got %+v", got)
	}
}

func TestGetMBID_ReturnsCurrentValue(t *testing.T) {
	s, r := newTestHandler(t, &fakeSearcher{}, nil)
	artists, err := s.FindOrCreateArtists([]string{"Nirvana"}, []string{"existing-mbid"})
	if err != nil {
		t.Fatal(err)
	}
	id := artists[0].ID

	req := httptest.NewRequest(http.MethodGet, "/artists/"+strconv.FormatUint(uint64(id), 10)+"/mbid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var got struct {
		MBArtistID string `json:"mbArtistId"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.MBArtistID != "existing-mbid" {
		t.Fatalf("expected existing-mbid, got %q", got.MBArtistID)
	}
}

func TestGetArtistImageSource_FolderImage(t *testing.T) {
	s, r := newTestHandler(t, &fakeSearcher{}, nil)
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "artist.jpg")
	if err := os.WriteFile(imgPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	artists, err := s.FindOrCreateArtists([]string{"Pink Floyd"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := artists[0].ID
	if err := s.SetArtistImagePath(id, imgPath); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/artists/"+strconv.FormatUint(uint64(id), 10)+"/image-source", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got struct {
		Source string `json:"source"`
		Path   string `json:"path"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "folder" {
		t.Errorf("source = %q, want %q", got.Source, "folder")
	}
	if got.Path != imgPath {
		t.Errorf("path = %q, want %q", got.Path, imgPath)
	}
}

// A stored (uploaded or fetched) image outranks the folder image, so the note
// must not claim the image comes from disk.
func TestGetArtistImageSource_StoredImageWins(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := store.New(db)
	as := assetstore.New(t.TempDir())
	h := &artistsHandler.Handler{Store: s, Assets: as, Search: &fakeSearcher{}}
	r := mux.NewRouter()
	h.Routes(r)

	dir := t.TempDir()
	imgPath := filepath.Join(dir, "artist.jpg")
	if err := os.WriteFile(imgPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	artists, err := s.FindOrCreateArtists([]string{"Pink Floyd"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := artists[0].ID
	if err := s.SetArtistImagePath(id, imgPath); err != nil {
		t.Fatal(err)
	}
	if err := as.PutManual(assetstore.KindArtist, strconv.FormatUint(uint64(id), 10), "png", []byte("x")); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/artists/"+strconv.FormatUint(uint64(id), 10)+"/image-source", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var got struct {
		Source string `json:"source"`
		Path   string `json:"path"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "upload" {
		t.Errorf("source = %q, want %q", got.Source, "upload")
	}
	if got.Path != "" {
		t.Errorf("path = %q, want empty for a stored image", got.Path)
	}
}

// An artist with neither a stored nor a folder image is served the generated
// avatar; the UI shows no note for that.
func TestGetArtistImageSource_None(t *testing.T) {
	s, r := newTestHandler(t, &fakeSearcher{}, nil)
	artists, err := s.FindOrCreateArtists([]string{"Pink Floyd"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/artists/"+strconv.FormatUint(uint64(artists[0].ID), 10)+"/image-source", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var got struct {
		Source string `json:"source"`
		Path   string `json:"path"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "none" {
		t.Errorf("source = %q, want %q", got.Source, "none")
	}
}

// A recorded path whose file has gone away must not be reported as the source:
// getCoverArt would fall through to the generated avatar.
func TestGetArtistImageSource_MissingFolderImage(t *testing.T) {
	s, r := newTestHandler(t, &fakeSearcher{}, nil)
	artists, err := s.FindOrCreateArtists([]string{"Pink Floyd"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := artists[0].ID
	if err := s.SetArtistImagePath(id, filepath.Join(t.TempDir(), "gone", "artist.jpg")); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/artists/"+strconv.FormatUint(uint64(id), 10)+"/image-source", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var got struct {
		Source string `json:"source"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "none" {
		t.Errorf("source = %q, want %q", got.Source, "none")
	}
}

func TestGetArtistImageSource_NotFound(t *testing.T) {
	_, r := newTestHandler(t, &fakeSearcher{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/artists/999/image-source", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// A manual upload and an auto-fetched image both live in the asset store, but the
// editor must tell them apart: only an upload is something the user put there
// (and can meaningfully remove).
func TestGetArtistImageSource_DistinguishesUploadFromFetched(t *testing.T) {
	tests := []struct {
		name       string
		put        func(as *assetstore.Store, key string) error
		wantSource string
	}{
		{
			name: "manual upload",
			put: func(as *assetstore.Store, key string) error {
				return as.PutManual(assetstore.KindArtist, key, "png", []byte("x"))
			},
			wantSource: "upload",
		},
		{
			name: "auto-fetched",
			put: func(as *assetstore.Store, key string) error {
				return as.PutAuto(assetstore.KindArtist, key, "png", []byte("x"))
			},
			wantSource: "fetched",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err := model.Migrate(db); err != nil {
				t.Fatal(err)
			}
			s := store.New(db)
			as := assetstore.New(t.TempDir())
			h := &artistsHandler.Handler{Store: s, Assets: as, Search: &fakeSearcher{}}
			r := mux.NewRouter()
			h.Routes(r)

			artists, err := s.FindOrCreateArtists([]string{"Pink Floyd"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			id := strconv.FormatUint(uint64(artists[0].ID), 10)
			if err := tt.put(as, id); err != nil {
				t.Fatal(err)
			}

			req := httptest.NewRequest(http.MethodGet, "/artists/"+id+"/image-source", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			var got struct {
				Source   string `json:"source"`
				Path     string `json:"path"`
				Filename string `json:"filename"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Source != tt.wantSource {
				t.Errorf("source = %q, want %q", got.Source, tt.wantSource)
			}
			if got.Filename == "" {
				t.Error("filename should name the stored image file")
			}
			if got.Path != "" {
				t.Errorf("path = %q, want empty for a stored image", got.Path)
			}
		})
	}
}

// The folder case names the file on disk so the editor can show it.
func TestGetArtistImageSource_FolderReportsFilename(t *testing.T) {
	s, r := newTestHandler(t, &fakeSearcher{}, nil)
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "artist.jpg")
	if err := os.WriteFile(imgPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	artists, err := s.FindOrCreateArtists([]string{"Pink Floyd"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := artists[0].ID
	if err := s.SetArtistImagePath(id, imgPath); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/artists/"+strconv.FormatUint(uint64(id), 10)+"/image-source", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var got struct {
		Source   string `json:"source"`
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "folder" {
		t.Errorf("source = %q, want folder", got.Source)
	}
	if got.Filename != "artist.jpg" {
		t.Errorf("filename = %q, want artist.jpg", got.Filename)
	}
}
