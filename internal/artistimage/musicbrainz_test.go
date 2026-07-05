package artistimage

import (
	"context"
	"net/http"
	"net/http/httptest"
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
