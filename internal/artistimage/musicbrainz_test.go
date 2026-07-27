package artistimage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/upstream"
	"golang.org/x/time/rate"
)

// newTestSearch points a search client at srv with throttling and retry backoff
// disabled, so the logic under test runs at full speed.
func newTestSearch(t *testing.T, srv *httptest.Server) *MusicBrainzSearch {
	t.Helper()
	m := NewMusicBrainzSearch("Aether/test (https://example.com)")
	m.BaseURL = srv.URL
	m.Doer.Client = srv.Client()
	m.Doer.Limiter = rate.NewLimiter(rate.Inf, 1)
	m.Doer.Wait = func(context.Context, time.Duration) error { return nil }
	return m
}

func TestMusicBrainzSearchParsesResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "Aether/test (https://example.com)" {
			t.Errorf("unexpected User-Agent: %q", got)
		}
		if !strings.Contains(r.URL.RawQuery, "query=Nirvana") {
			t.Errorf("expected query in request, got %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{
			"artists": [
				{
					"id": "5b11f4ce-a62d-471e-81fc-a69a8278c7da",
					"name": "Nirvana",
					"type": "Group",
					"disambiguation": "90s Seattle grunge band",
					"score": 100,
					"life-span": {"begin": "1987", "end": "1994"}
				}
			]
		}`))
	}))
	defer srv.Close()

	m := newTestSearch(t, srv)

	results, err := m.Search(context.Background(), "Nirvana", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	got := results[0]
	want := Candidate{
		MBID:           "5b11f4ce-a62d-471e-81fc-a69a8278c7da",
		Name:           "Nirvana",
		Type:           "Group",
		Disambiguation: "90s Seattle grunge band",
		LifeSpanBegin:  "1987",
		LifeSpanEnd:    "1994",
		Score:          100,
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestMusicBrainzReleaseGroupGenres(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/ws/2/release-group/rg-uuid") {
			t.Errorf("expected release-group endpoint, got %q", r.URL.Path)
		}
		if !strings.Contains(r.URL.RawQuery, "inc=genres") {
			t.Errorf("expected inc=genres in request, got %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{
			"genres": [
				{"name": "art rock", "count": 5},
				{"name": "alternative rock", "count": 12},
				{"name": "electronic", "count": 2}
			]
		}`))
	}))
	defer srv.Close()

	m := newTestSearch(t, srv)

	got, err := m.ReleaseGroupGenres(context.Background(), "rg-uuid")
	if err != nil {
		t.Fatalf("ReleaseGroupGenres: %v", err)
	}
	want := []string{"alternative rock", "art rock", "electronic"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMusicBrainzReleaseGroupGenresEmptyMBID(t *testing.T) {
	m := NewMusicBrainzSearch("Aether/test (https://example.com)")
	got, err := m.ReleaseGroupGenres(context.Background(), "")
	if err != nil || got != nil {
		t.Fatalf("expected nil/nil on empty mbid, got %v, %v", got, err)
	}
}

func TestMusicBrainzReleaseGroupGenresUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	m := newTestSearch(t, srv)

	if _, err := m.ReleaseGroupGenres(context.Background(), "rg-uuid"); err == nil {
		t.Fatal("expected error on upstream failure")
	}
}

func TestMusicBrainzSearchReleaseParsesResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/ws/2/release") {
			t.Errorf("expected release endpoint, got %q", r.URL.Path)
		}
		if !strings.Contains(r.URL.RawQuery, "query=OK+Computer") {
			t.Errorf("expected query in request, got %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{
			"releases": [
				{
					"id": "rel-uuid",
					"title": "OK Computer",
					"disambiguation": "remaster",
					"date": "1997-05-21",
					"country": "GB",
					"track-count": 12,
					"score": 100,
					"artist-credit": [
						{"name": "Radiohead", "joinphrase": " feat. ", "artist": {"id": "radiohead-uuid"}},
						{"name": "Someone", "joinphrase": "", "artist": {"id": "someone-uuid"}}
					],
					"release-group": {"id": "rg-uuid"}
				}
			]
		}`))
	}))
	defer srv.Close()

	m := newTestSearch(t, srv)

	results, err := m.SearchRelease(context.Background(), "OK Computer", 10)
	if err != nil {
		t.Fatalf("SearchRelease: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	got := results[0]
	want := ReleaseCandidate{
		ReleaseMBID:      "rel-uuid",
		ReleaseGroupMBID: "rg-uuid",
		Title:            "OK Computer",
		Artist:           "Radiohead feat. Someone",
		Artists: []ReleaseArtistCredit{
			{Name: "Radiohead", MBID: "radiohead-uuid"},
			{Name: "Someone", MBID: "someone-uuid"},
		},
		Date:           "1997-05-21",
		Country:        "GB",
		TrackCount:     12,
		Disambiguation: "remaster",
		Score:          100,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestMusicBrainzSearchReleaseEmptyQuery(t *testing.T) {
	m := NewMusicBrainzSearch("Aether/test")
	results, err := m.SearchRelease(context.Background(), "", 10)
	if err != nil || results != nil {
		t.Fatalf("expected nil, nil for empty query, got %v, %v", results, err)
	}
}

func TestMusicBrainzSearchReleaseUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	m := newTestSearch(t, srv)

	if _, err := m.SearchRelease(context.Background(), "OK Computer", 10); err == nil {
		t.Fatal("expected an error for a non-200 upstream response")
	}
}

func TestMusicBrainzSearchEmptyQuery(t *testing.T) {
	m := NewMusicBrainzSearch("Aether/test")
	results, err := m.Search(context.Background(), "", 10)
	if err != nil || results != nil {
		t.Fatalf("expected nil, nil for empty query, got %v, %v", results, err)
	}
}

func TestMusicBrainzSearchUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	m := newTestSearch(t, srv)

	if _, err := m.Search(context.Background(), "Nirvana", 10); err == nil {
		t.Fatal("expected an error for a non-200 upstream response")
	}
}

// MusicBrainz throttles aggressively; a 503 that clears on the next attempt
// must not reach the user at all.
func TestMusicBrainzSearchRetriesTransientFailure(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"artists":[{"id":"a-1","name":"Nirvana"}]}`))
	}))
	defer srv.Close()

	m := newTestSearch(t, srv)
	got, err := m.Search(context.Background(), "Nirvana", 10)
	if err != nil {
		t.Fatalf("Search should have recovered on retry: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Nirvana" {
		t.Fatalf("unexpected results: %+v", got)
	}
	if n := atomic.LoadInt32(&hits); n != 2 {
		t.Fatalf("want 2 attempts, got %d", n)
	}
}

// A persistent failure surfaces as a typed error naming MusicBrainz, so the UI
// can show a sentence instead of "status 503".
func TestMusicBrainzSearchFailureIsTypedUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	m := newTestSearch(t, srv)
	_, err := m.Search(context.Background(), "Nirvana", 10)
	var uerr *upstream.Error
	if !errors.As(err, &uerr) {
		t.Fatalf("want *upstream.Error, got %T: %v", err, err)
	}
	if uerr.Service != "MusicBrainz" {
		t.Fatalf("service = %q, want MusicBrainz", uerr.Service)
	}
	if msg := uerr.UserMessage(); !strings.Contains(msg, "MusicBrainz") {
		t.Fatalf("unhelpful message: %q", msg)
	}
}

func TestMusicBrainzReleaseSearchRetriesTransientFailure(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"releases":[{"id":"rel-1","title":"OK Computer"}]}`))
	}))
	defer srv.Close()

	m := newTestSearch(t, srv)
	got, err := m.SearchRelease(context.Background(), "OK Computer", 10)
	if err != nil {
		t.Fatalf("SearchRelease should have recovered on retry: %v", err)
	}
	if len(got) != 1 || got[0].Title != "OK Computer" {
		t.Fatalf("unexpected results: %+v", got)
	}
}

func TestMusicBrainzGenresRetriesTransientFailure(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"genres":[{"name":"rock","count":3}]}`))
	}))
	defer srv.Close()

	m := newTestSearch(t, srv)
	got, err := m.ReleaseGroupGenres(context.Background(), "rg-uuid")
	if err != nil {
		t.Fatalf("ReleaseGroupGenres should have recovered on retry: %v", err)
	}
	if len(got) != 1 || got[0] != "rock" {
		t.Fatalf("unexpected genres: %v", got)
	}
}

func TestMusicBrainzReleaseParsesTracklist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/ws/2/release/rel-1") {
			t.Errorf("unexpected path: %q", r.URL.Path)
		}
		if !strings.Contains(r.URL.RawQuery, "inc=recordings") {
			t.Errorf("expected inc=recordings..., got %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{
			"id": "rel-1",
			"title": "Nevermind",
			"date": "1991-09-24",
			"release-group": {"id": "rg-1"},
			"artist-credit": [{"name": "Nirvana", "joinphrase": "", "artist": {"id": "art-1"}}],
			"media": [
				{
					"position": 1,
					"track-count": 2,
					"tracks": [
						{"position": 1, "title": "Smells Like Teen Spirit", "length": 301000,
						 "recording": {"id": "rec-1"}},
						{"position": 2, "title": "In Bloom", "length": 254000,
						 "recording": {"id": "rec-2"}}
					]
				}
			]
		}`))
	}))
	defer srv.Close()

	m := newTestSearch(t, srv)
	got, err := m.Release(context.Background(), "rel-1")
	if err != nil {
		t.Fatalf("Release: %v", err)
	}
	if got.ReleaseMBID != "rel-1" || got.ReleaseGroupMBID != "rg-1" || got.Title != "Nevermind" {
		t.Fatalf("unexpected release: %+v", got)
	}
	if got.Artist != "Nirvana" || len(got.Artists) != 1 || got.Artists[0].MBID != "art-1" {
		t.Fatalf("unexpected artists: %+v", got.Artists)
	}
	if got.TrackCount != 2 || got.DiscCount != 1 {
		t.Fatalf("expected 2 tracks on 1 disc, got %d/%d", got.TrackCount, got.DiscCount)
	}
	want := ReleaseTrack{
		DiscNumber: 1, TrackNumber: 2, Title: "In Bloom",
		DurationSeconds: 254, RecordingMBID: "rec-2",
	}
	if !reflect.DeepEqual(got.Tracks[1], want) {
		t.Fatalf("unexpected track: got %+v want %+v", got.Tracks[1], want)
	}
}

func TestMusicBrainzReleaseEmptyMBIDSkipsRequest(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer srv.Close()

	m := newTestSearch(t, srv)
	got, err := m.Release(context.Background(), "")
	if err != nil {
		t.Fatalf("Release: %v", err)
	}
	if got.ReleaseMBID != "" || calls.Load() != 0 {
		t.Fatalf("expected no request and a zero release, got %+v after %d calls", got, calls.Load())
	}
}

func TestMusicBrainzReleaseUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	m := newTestSearch(t, srv)
	if _, err := m.Release(context.Background(), "rel-1"); err == nil {
		t.Fatal("expected an error for a 500 response")
	}
}
