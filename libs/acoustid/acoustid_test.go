package acoustid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/time/rate"
)

func newTestClient(srv *httptest.Server) *Client {
	c := New("test-key", "Aether/test (https://example.com)")
	c.BaseURL = srv.URL
	c.Client = srv.Client()
	c.limiter = rate.NewLimiter(rate.Inf, 1)
	return c
}

func TestLookupParsesResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.PostForm.Get("client"); got != "test-key" {
			t.Errorf("unexpected client key: %q", got)
		}
		if got := r.PostForm.Get("fingerprint"); got != "FINGERPRINT" {
			t.Errorf("unexpected fingerprint: %q", got)
		}
		if got := r.PostForm.Get("duration"); got != "123" {
			t.Errorf("unexpected duration: %q", got)
		}
		if got := r.PostForm.Get("meta"); !strings.Contains(got, "recordings") {
			t.Errorf("expected recordings in meta, got %q", got)
		}
		if got := r.PostForm.Get("meta"); !strings.Contains(got, "tracks") {
			t.Errorf("expected tracks in meta, got %q", got)
		}
		_, _ = w.Write([]byte(`{
			"status": "ok",
			"results": [
				{
					"id": "acoustid-uuid",
					"score": 0.97,
					"recordings": [
						{
							"id": "rec-uuid",
							"title": "Smells Like Teen Spirit",
							"artists": [
								{"id": "artist-uuid", "name": "Nirvana"}
							],
							"releasegroups": [
								{
									"id": "rg-uuid",
									"title": "Nevermind",
									"releases": [
										{"id": "rel-uuid", "date": {"year": 1991}, "mediums": [{"position": 1, "tracks": [{"position": 5}]}]}
									]
								}
							]
						}
					]
				}
			]
		}`))
	}))
	defer srv.Close()

	got, err := newTestClient(srv).Lookup(context.Background(), "FINGERPRINT", 123.4)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	want := []Recording{
		{
			Score: 0.97,
			MBID:  "rec-uuid",
			Title: "Smells Like Teen Spirit",
			Artists: []ArtistCredit{
				{MBID: "artist-uuid", Name: "Nirvana"},
			},
			Release: []Release{
				{MBID: "rel-uuid", ReleaseGroupMBID: "rg-uuid", Title: "Nevermind", Year: 1991, TrackNumber: 5, DiscNumber: 1},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestLookupSortsByScoreAndSkipsEmptyMBIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"status": "ok",
			"results": [
				{"score": 0.5, "recordings": [{"id": "low", "title": "Low"}]},
				{"score": 0.9, "recordings": [{"id": "high", "title": "High"}, {"id": "", "title": "NoID"}]}
			]
		}`))
	}))
	defer srv.Close()

	got, err := newTestClient(srv).Lookup(context.Background(), "FP", 100)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 recordings, got %d", len(got))
	}
	if got[0].MBID != "high" || got[1].MBID != "low" {
		t.Fatalf("expected score-descending order, got %q then %q", got[0].MBID, got[1].MBID)
	}
}

func TestLookupServiceError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status": "error", "error": {"message": "invalid API key"}}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).Lookup(context.Background(), "FP", 100)
	if err == nil || !strings.Contains(err.Error(), "invalid API key") {
		t.Fatalf("expected service error message, got %v", err)
	}
}
