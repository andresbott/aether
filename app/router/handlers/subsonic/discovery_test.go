package subsonic

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/imagecache"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type discoveryEnvelope struct {
	SubsonicResponse struct {
		Status    string `json:"status"`
		Discovery struct {
			Album []struct {
				ID      string `json:"id"`
				Name    string `json:"name"`
				Rank    int    `json:"rank"`
				Reason  string `json:"reason"`
				Starred string `json:"starred"`
			} `json:"album"`
			Playlist []struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Rank   int    `json:"rank"`
				Reason string `json:"reason"`
			} `json:"playlist"`
		} `json:"discovery"`
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	} `json:"subsonic-response"`
}

func newDiscoveryStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return store.New(db)
}

func newDiscoveryServer(t *testing.T, s *store.Store) *httptest.Server {
	t.Helper()
	r := mux.NewRouter()
	Register(r, s, assetstore.New(t.TempDir()), imagecache.New(t.TempDir()), nil)
	return httptest.NewServer(r)
}

func getDiscoveryJSON(t *testing.T, srv *httptest.Server, query string) discoveryEnvelope {
	t.Helper()
	resp, err := http.Get(srv.URL + "/rest/getDiscovery" + query)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body discoveryEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

func seedDiscoveryAlbum(t *testing.T, s *store.Store, name string) model.Album {
	t.Helper()
	al := model.Album{Name: name, NameNorm: name, AlbumArtistNorm: "x"}
	if err := s.DB().Create(&al).Error; err != nil {
		t.Fatal(err)
	}
	return al
}

func TestGetDiscoveryReturnsOkEnvelope(t *testing.T) {
	s := newDiscoveryStore(t)
	seedDiscoveryAlbum(t, s, "A")
	srv := newDiscoveryServer(t, s)
	defer srv.Close()

	body := getDiscoveryJSON(t, srv, "")
	if body.SubsonicResponse.Status != "ok" {
		t.Fatalf("status = %q, want ok (error: %+v)",
			body.SubsonicResponse.Status, body.SubsonicResponse.Error)
	}
	if len(body.SubsonicResponse.Discovery.Album) != 1 {
		t.Fatalf("got %d albums, want 1", len(body.SubsonicResponse.Discovery.Album))
	}
}

func TestGetDiscoveryEveryItemCarriesRankAndReason(t *testing.T) {
	s := newDiscoveryStore(t)
	for i := 0; i < 3; i++ {
		seedDiscoveryAlbum(t, s, string(rune('A'+i)))
	}
	pl := model.Playlist{Name: "PL", Owner: "admin"}
	if err := s.DB().Create(&pl).Error; err != nil {
		t.Fatal(err)
	}
	srv := newDiscoveryServer(t, s)
	defer srv.Close()

	body := getDiscoveryJSON(t, srv, "")
	d := body.SubsonicResponse.Discovery
	if len(d.Album)+len(d.Playlist) != 4 {
		t.Fatalf("got %d items, want 4", len(d.Album)+len(d.Playlist))
	}
	ranks := map[int]bool{}
	for _, a := range d.Album {
		if a.Reason == "" {
			t.Fatalf("album %s has no reason", a.ID)
		}
		ranks[a.Rank] = true
	}
	for _, p := range d.Playlist {
		if p.Reason == "" {
			t.Fatalf("playlist %s has no reason", p.ID)
		}
		ranks[p.Rank] = true
	}
	// Four items must occupy four distinct ranks 0..3 across both arrays.
	for want := 0; want < 4; want++ {
		if !ranks[want] {
			t.Fatalf("rank %d missing; got %v", want, ranks)
		}
	}
}

func TestGetDiscoveryRanksContinueAcrossPages(t *testing.T) {
	s := newDiscoveryStore(t)
	for i := 0; i < 8; i++ {
		seedDiscoveryAlbum(t, s, string(rune('A'+i)))
	}
	srv := newDiscoveryServer(t, s)
	defer srv.Close()

	page1 := getDiscoveryJSON(t, srv, "?size=4&offset=0&seed=42")
	page2 := getDiscoveryJSON(t, srv, "?size=4&offset=4&seed=42")

	collect := func(e discoveryEnvelope) (ids []string, ranks []int) {
		for _, a := range e.SubsonicResponse.Discovery.Album {
			ids = append(ids, a.ID)
			ranks = append(ranks, a.Rank)
		}
		return ids, ranks
	}
	ids1, ranks1 := collect(page1)
	ids2, ranks2 := collect(page2)

	for _, r := range ranks1 {
		if r < 0 || r > 3 {
			t.Fatalf("page 1 rank %d outside 0..3", r)
		}
	}
	for _, r := range ranks2 {
		if r < 4 || r > 7 {
			t.Fatalf("page 2 rank %d outside 4..7", r)
		}
	}
	seen := map[string]bool{}
	for _, id := range append(ids1, ids2...) {
		if seen[id] {
			t.Fatalf("id %s appeared on both pages", id)
		}
		seen[id] = true
	}
}

func TestGetDiscoveryOmitsStarredWhenNotStarred(t *testing.T) {
	s := newDiscoveryStore(t)
	seedDiscoveryAlbum(t, s, "Plain")
	srv := newDiscoveryServer(t, s)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rest/getDiscovery")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	sr := raw["subsonic-response"].(map[string]any)
	d := sr["discovery"].(map[string]any)
	albums := d["album"].([]any)
	first := albums[0].(map[string]any)
	if _, present := first["starred"]; present {
		t.Fatal("starred key present on an unstarred album; it must be omitted entirely")
	}
}

func TestGetDiscoveryEmitsStarredWhenStarred(t *testing.T) {
	s := newDiscoveryStore(t)
	al := seedDiscoveryAlbum(t, s, "Fav")
	// Add a track and record a play so the album has play history and doesn't end
	// up in the rediscover pool (which would override the reason to "rediscover").
	tr := model.Track{AlbumID: al.ID, Filename: "01.mp3", FilePath: "/01.mp3", Title: "Song"}
	if err := s.DB().Create(&tr).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.RecordPlay(tr.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.Star("admin", "album", al.ID); err != nil {
		t.Fatal(err)
	}
	srv := newDiscoveryServer(t, s)
	defer srv.Close()

	body := getDiscoveryJSON(t, srv, "")
	albums := body.SubsonicResponse.Discovery.Album
	if len(albums) != 1 {
		t.Fatalf("got %d albums, want 1", len(albums))
	}
	if albums[0].Starred == "" {
		t.Fatal("starred album has no starred timestamp")
	}
	if albums[0].Reason != "favorite" {
		t.Fatalf("reason = %q, want favorite", albums[0].Reason)
	}
}

func TestGetDiscoveryCapsSize(t *testing.T) {
	s := newDiscoveryStore(t)
	// Seed enough albums that the cap matters.
	for i := 0; i < 250; i++ {
		seedDiscoveryAlbum(t, s, string(rune('A'+i%26))+string(rune('0'+(i/26)%10))+string(rune('a'+(i/260)%26)))
	}
	srv := newDiscoveryServer(t, s)
	defer srv.Close()

	// A size far above the cap must be clamped to the max, not error or return everything.
	body := getDiscoveryJSON(t, srv, "?size=100000")
	if body.SubsonicResponse.Status != "ok" {
		t.Fatalf("status = %q, want ok", body.SubsonicResponse.Status)
	}
	got := len(body.SubsonicResponse.Discovery.Album) + len(body.SubsonicResponse.Discovery.Playlist)
	if got != 200 {
		t.Fatalf("?size=100000 returned %d items, want 200 (the max cap)", got)
	}
}

func TestGetDiscoveryFallsBackOnMalformedSeed(t *testing.T) {
	s := newDiscoveryStore(t)
	seedDiscoveryAlbum(t, s, "A")
	srv := newDiscoveryServer(t, s)
	defer srv.Close()

	body := getDiscoveryJSON(t, srv, "?seed=not-a-number")
	if body.SubsonicResponse.Status != "ok" {
		t.Fatalf("status = %q, want ok — a bad seed must fall back, not error",
			body.SubsonicResponse.Status)
	}
}

func TestGetDiscoveryOnEmptyLibrary(t *testing.T) {
	s := newDiscoveryStore(t)
	srv := newDiscoveryServer(t, s)
	defer srv.Close()

	body := getDiscoveryJSON(t, srv, "")
	if body.SubsonicResponse.Status != "ok" {
		t.Fatalf("status = %q, want ok", body.SubsonicResponse.Status)
	}
	d := body.SubsonicResponse.Discovery
	if len(d.Album) != 0 || len(d.Playlist) != 0 {
		t.Fatalf("got %d albums and %d playlists from an empty library, want 0 and 0",
			len(d.Album), len(d.Playlist))
	}
}

func TestGetDiscoveryRegisteredAtDotViewToo(t *testing.T) {
	s := newDiscoveryStore(t)
	seedDiscoveryAlbum(t, s, "A")
	srv := newDiscoveryServer(t, s)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rest/getDiscovery.view")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestDiscoveryExtensionIsAdvertised(t *testing.T) {
	s := newDiscoveryStore(t)
	srv := newDiscoveryServer(t, s)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rest/getOpenSubsonicExtensions")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		SubsonicResponse struct {
			OpenSubsonicExtensions []struct {
				Name     string `json:"name"`
				Versions []int  `json:"versions"`
			} `json:"openSubsonicExtensions"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	for _, e := range body.SubsonicResponse.OpenSubsonicExtensions {
		if e.Name == "discovery" {
			return
		}
	}
	t.Fatal("the discovery extension is not advertised")
}

func TestGetDiscoveryDefaultSizeIs48(t *testing.T) {
	s := newDiscoveryStore(t)
	// Seed enough albums that we can verify the default size cap is applied.
	for i := 0; i < 60; i++ {
		seedDiscoveryAlbum(t, s, string(rune('A'+i%26))+string(rune('0'+i/26)))
	}
	srv := newDiscoveryServer(t, s)
	defer srv.Close()

	body := getDiscoveryJSON(t, srv, "")
	if body.SubsonicResponse.Status != "ok" {
		t.Fatalf("status = %q, want ok", body.SubsonicResponse.Status)
	}
	got := len(body.SubsonicResponse.Discovery.Album) + len(body.SubsonicResponse.Discovery.Playlist)
	if got != 48 {
		t.Fatalf("default size returned %d items, want 48", got)
	}
}
