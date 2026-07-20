package artistimage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/time/rate"
)

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

	m := NewMusicBrainzSearch("Aether/test (https://example.com)")
	m.BaseURL = srv.URL
	m.Client = srv.Client()
	m.limiter = rate.NewLimiter(rate.Inf, 1)

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

	m := NewMusicBrainzSearch("Aether/test (https://example.com)")
	m.BaseURL = srv.URL
	m.Client = srv.Client()
	m.limiter = rate.NewLimiter(rate.Inf, 1)

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

	m := NewMusicBrainzSearch("Aether/test (https://example.com)")
	m.BaseURL = srv.URL
	m.Client = srv.Client()
	m.limiter = rate.NewLimiter(rate.Inf, 1)

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

	m := NewMusicBrainzSearch("Aether/test (https://example.com)")
	m.BaseURL = srv.URL
	m.Client = srv.Client()
	m.limiter = rate.NewLimiter(rate.Inf, 1)

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

	m := NewMusicBrainzSearch("Aether/test")
	m.BaseURL = srv.URL
	m.Client = srv.Client()
	m.limiter = rate.NewLimiter(rate.Inf, 1)

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

	m := NewMusicBrainzSearch("Aether/test")
	m.BaseURL = srv.URL
	m.Client = srv.Client()
	m.limiter = rate.NewLimiter(rate.Inf, 1)

	if _, err := m.Search(context.Background(), "Nirvana", 10); err == nil {
		t.Fatal("expected an error for a non-200 upstream response")
	}
}
