package artistimage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestSearchReleaseGroupsParsesResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws/2/release-group/" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("query"); got != `(Nevermind Nirvana)` {
			t.Errorf("unexpected query %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "5" {
			t.Errorf("unexpected limit %q", got)
		}
		_, _ = w.Write([]byte(`{"release-groups": [{
			"id": "1b022e01-4da6-387b-8658-8678046e4cef",
			"title": "Nevermind",
			"primary-type": "Album",
			"secondary-types": ["Compilation"],
			"first-release-date": "1991-09-24",
			"disambiguation": "",
			"score": 100,
			"artist-credit": [{"name": "Nirvana", "joinphrase": " & "}, {"name": "Friends"}]
		}]}`))
	}))
	defer srv.Close()

	got, err := newTestSearch(t, srv).SearchReleaseGroups(context.Background(), SearchQuery("Nevermind Nirvana"), 5)
	if err != nil {
		t.Fatal(err)
	}
	want := []ReleaseGroupCandidate{{
		MBID:             "1b022e01-4da6-387b-8658-8678046e4cef",
		Title:            "Nevermind",
		Artist:           "Nirvana & Friends",
		PrimaryType:      "Album",
		SecondaryTypes:   []string{"Compilation"},
		FirstReleaseDate: "1991-09-24",
		Score:            100,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestSearchRecordingsPrefersStudioAlbum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws/2/recording/" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"recordings": [
			{
				"id": "rec-1", "title": "Lithium", "length": 257000, "score": 98,
				"first-release-date": "1991-09-24",
				"artist-credit": [{"name": "Nirvana"}],
				"releases": [
					{"title": "Greatest Hits", "release-group": {"primary-type": "Album", "secondary-types": ["Compilation"]}},
					{"title": "Nevermind", "release-group": {"primary-type": "Album"}}
				]
			},
			{
				"id": "rec-2", "title": "Lithium", "score": 80,
				"artist-credit": [{"name": "Nirvana"}],
				"releases": [{"title": "Lithium", "release-group": {"primary-type": "Single"}}]
			},
			{"id": "rec-3", "title": "Lithium (demo)", "score": 50}
		]}`))
	}))
	defer srv.Close()

	got, err := newTestSearch(t, srv).SearchRecordings(context.Background(), SearchQuery("Lithium"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 results, got %d", len(got))
	}
	if got[0].Release != "Nevermind" || got[0].DurationSeconds != 257 || got[0].Artist != "Nirvana" {
		t.Errorf("first result: %+v", got[0])
	}
	if got[1].Release != "Lithium" {
		t.Errorf("without an album the first release is used: %+v", got[1])
	}
	if got[2].Release != "" {
		t.Errorf("no releases means no release title: %+v", got[2])
	}
}

func TestSearchEmptyQueryMakesNoRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s", r.URL)
	}))
	defer srv.Close()
	m := newTestSearch(t, srv)
	if got, err := m.SearchReleaseGroups(context.Background(), "", 5); got != nil || err != nil {
		t.Fatalf("got %v %v", got, err)
	}
	if got, err := m.SearchRecordings(context.Background(), "", 5); got != nil || err != nil {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestSearchQueryEscapesSyntax(t *testing.T) {
	cases := []struct{ text, want string }{
		{"  ", ""},
		{"  Nevermind ", "(Nevermind)"},
		{"AC/DC: Live!", `(AC\/DC\: Live\!)`},
		{`a "b" (x)`, `(a \"b\" \(x\))`},
	}
	for _, c := range cases {
		if got := SearchQuery(c.text); got != c.want {
			t.Errorf("SearchQuery(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}
