package metadata_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	metaHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	"github.com/andresbott/aether/internal/albumidentify"
	"github.com/andresbott/aether/internal/artistimage"
	"github.com/andresbott/aether/internal/identify"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/libs/acoustid"
	"github.com/andresbott/aether/libs/fpcalc"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// This file covers the seam the two endpoints share: both /metadata/identify and
// /metadata/identify-album resolve files through ONE *identify.Identifier, so a
// cache on it means the second endpoint reuses the first one's fingerprint pass
// instead of paying another fpcalc run plus a rate-limited AcoustID call per
// file. Everything here therefore uses the real Identifier, not a fake.

// fakeFpcalcBin writes an executable stand-in for the fpcalc binary. Each run
// appends a byte to a marker file next to it, which is how fpcalcRuns counts the
// fingerprint passes an endpoint actually performed.
func fakeFpcalcBin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "fpcalc")
	script := "#!/bin/sh\nprintf x >> " + filepath.Join(dir, "runs") + "\n" +
		`echo '{"duration": 180.0, "fingerprint": "ABC123"}'` + "\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// fpcalcRuns counts the fingerprint passes recorded so far by reading the marker
// the fake binary appends to.
func fpcalcRuns(t *testing.T, bin string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(filepath.Dir(bin), "runs"))
	if err != nil {
		// No marker yet means the binary never ran.
		return 0
	}
	return len(data)
}

// countingReleaseLookup stands in for MusicBrainz and counts the tracklist
// fetches. The real client is throttled to ONE request per second and a run
// enriches up to MaxEnrichedOptions releases, so these calls — not the
// fingerprinting — are what a repeated album identify waits on.
type countingReleaseLookup struct {
	calls atomic.Int32
}

func (c *countingReleaseLookup) Release(
	_ context.Context, mbid string,
) (artistimage.ReleaseDetail, error) {
	c.calls.Add(1)
	return artistimage.ReleaseDetail{
		ReleaseMBID: mbid,
		Title:       "Album A",
		TrackCount:  2,
		DiscCount:   1,
		Tracks: []artistimage.ReleaseTrack{
			{DiscNumber: 1, TrackNumber: 1, Title: "One", RecordingMBID: "rec-1"},
			{DiscNumber: 1, TrackNumber: 2, Title: "Two", RecordingMBID: "rec-2"},
		},
	}, nil
}

// newSharedIdentifyHandler wires both identify endpoints onto one real
// Identifier, exactly as app/router/api_v1.go does.
func newSharedIdentifyHandler(
	t *testing.T, libRoot string, ident *identify.Identifier,
) (*mux.Router, *model.Library) {
	return newSharedIdentifyHandlerWithReleases(t, libRoot, ident, &countingReleaseLookup{}, 0)
}

// newSharedIdentifyHandlerWithReleases is newSharedIdentifyHandler with an
// explicit tracklist lookup and release-cache size, so a test can count the
// MusicBrainz fetches a run performs.
func newSharedIdentifyHandlerWithReleases(
	t *testing.T,
	libRoot string,
	ident *identify.Identifier,
	releases albumidentify.ReleaseLookup,
	releaseCacheSize int,
) (*mux.Router, *model.Library) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := store.New(db)
	lib := &model.Library{Name: "Main", Path: libRoot, FollowSymlinks: true}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	h := &metaHandler.Handler{
		Store:      s,
		Reader:     nullReader{},
		Identifier: ident,
		// The same instance behind both endpoints — this is what makes the cache
		// shared rather than per-flow.
		AlbumIdentifier: albumidentify.New(
			ident,
			albumidentify.NewCachingReleaseLookup(releases, releaseCacheSize),
		),
	}
	r := mux.NewRouter()
	h.Routes(r)
	return r, lib
}

func countingAcoustIDServer(t *testing.T, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{
			"status": "ok",
			"results": [{
				"score": 0.95,
				"recordings": [{
					"id": "rec-1",
					"title": "One",
					"releasegroups": [{"id": "rg-A", "title": "Album A",
						"releases": [{"id": "rel-A", "title": "Album A"}]}]
				}]
			}]
		}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newRealIdentifier(t *testing.T, bin string, srv *httptest.Server, cache *identify.Cache) *identify.Identifier {
	t.Helper()
	ac := acoustid.New("test-key", "test-agent")
	ac.BaseURL = srv.URL
	ac.Client = srv.Client()
	id := identify.New(fpcalc.New(bin), ac)
	id.Cache = cache
	return id
}

// The behaviour the user expects: identifying songs and then identifying the
// same songs as an album must not fingerprint or look them up twice.
func TestIdentifyAndIdentifyAlbumShareOneFingerprintPass(t *testing.T) {
	root := t.TempDir()
	paths := []string{"01.mp3", "02.mp3"}
	for _, n := range paths {
		if err := os.WriteFile(filepath.Join(root, n), []byte("audio-"+n), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var lookups atomic.Int32
	bin := fakeFpcalcBin(t)
	srv := countingAcoustIDServer(t, &lookups)
	ident := newRealIdentifier(t, bin, srv, identify.NewCache(100))
	r, lib := newSharedIdentifyHandler(t, root, ident)

	w := postIdentify(t, r, map[string]any{"library_id": lib.ID, "paths": paths})
	if w.Code != http.StatusOK {
		t.Fatalf("identify: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	afterTracks := fpcalcRuns(t, bin)
	if afterTracks != len(paths) {
		t.Fatalf("expected %d fingerprint runs for the per-track pass, got %d", len(paths), afterTracks)
	}
	if got := lookups.Load(); got != int32(len(paths)) {
		t.Fatalf("expected %d AcoustID lookups, got %d", len(paths), got)
	}

	w = postIdentifyAlbum(t, r, map[string]any{"library_id": lib.ID, "paths": paths})
	if w.Code != http.StatusOK {
		t.Fatalf("identify-album: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if got := fpcalcRuns(t, bin); got != afterTracks {
		t.Fatalf("album identify re-fingerprinted: expected %d runs, got %d", afterTracks, got)
	}
	if got := lookups.Load(); got != int32(len(paths)) {
		t.Fatalf("album identify re-queried AcoustID: expected %d lookups, got %d", len(paths), got)
	}
}

// And the other direction: album first, then per-track.
func TestIdentifyAlbumThenIdentifyReusesTheCache(t *testing.T) {
	root := t.TempDir()
	paths := []string{"01.mp3", "02.mp3"}
	for _, n := range paths {
		if err := os.WriteFile(filepath.Join(root, n), []byte("audio-"+n), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var lookups atomic.Int32
	bin := fakeFpcalcBin(t)
	srv := countingAcoustIDServer(t, &lookups)
	ident := newRealIdentifier(t, bin, srv, identify.NewCache(100))
	r, lib := newSharedIdentifyHandler(t, root, ident)

	w := postIdentifyAlbum(t, r, map[string]any{"library_id": lib.ID, "paths": paths})
	if w.Code != http.StatusOK {
		t.Fatalf("identify-album: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	afterAlbum := fpcalcRuns(t, bin)

	w = postIdentify(t, r, map[string]any{"library_id": lib.ID, "paths": paths})
	if w.Code != http.StatusOK {
		t.Fatalf("identify: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if got := fpcalcRuns(t, bin); got != afterAlbum {
		t.Fatalf("per-track identify re-fingerprinted: expected %d runs, got %d", afterAlbum, got)
	}
	if got := lookups.Load(); got != int32(len(paths)) {
		t.Fatalf("per-track identify re-queried AcoustID: expected %d lookups, got %d", len(paths), got)
	}
}

// Without a cache both endpoints pay in full — the guard that the tests above
// are really observing the cache and not some accidental single-pass behaviour.
func TestWithoutACacheBothFlowsFingerprintSeparately(t *testing.T) {
	root := t.TempDir()
	// Two files: the album endpoint needs at least two paths to have a set to map.
	paths := []string{"01.mp3", "02.mp3"}
	for _, n := range paths {
		if err := os.WriteFile(filepath.Join(root, n), []byte("audio-"+n), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var lookups atomic.Int32
	bin := fakeFpcalcBin(t)
	srv := countingAcoustIDServer(t, &lookups)
	ident := newRealIdentifier(t, bin, srv, nil)
	r, lib := newSharedIdentifyHandler(t, root, ident)

	if w := postIdentify(t, r, map[string]any{"library_id": lib.ID, "paths": paths}); w.Code != http.StatusOK {
		t.Fatalf("identify: got %d: %s", w.Code, w.Body.String())
	}
	if w := postIdentifyAlbum(t, r, map[string]any{"library_id": lib.ID, "paths": paths}); w.Code != http.StatusOK {
		t.Fatalf("identify-album: got %d: %s", w.Code, w.Body.String())
	}

	// Each endpoint fingerprints every file itself: 2 files x 2 endpoints.
	if got := fpcalcRuns(t, bin); got != 4 {
		t.Fatalf("expected 4 fingerprint runs without a cache, got %d", got)
	}
	if got := lookups.Load(); got != 4 {
		t.Fatalf("expected 4 AcoustID lookups without a cache, got %d", got)
	}
}

// The symptom that started this: with the fingerprint cache in place a repeated
// album identify was STILL slow, because every run re-fetched the tracklist of
// every enriched option through a client throttled to one request per second.
func TestRepeatedIdentifyAlbumRefetchesNoTracklists(t *testing.T) {
	root := t.TempDir()
	paths := []string{"01.mp3", "02.mp3"}
	for _, n := range paths {
		if err := os.WriteFile(filepath.Join(root, n), []byte("audio-"+n), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var lookups atomic.Int32
	bin := fakeFpcalcBin(t)
	srv := countingAcoustIDServer(t, &lookups)
	ident := newRealIdentifier(t, bin, srv, identify.NewCache(100))
	releases := &countingReleaseLookup{}
	r, lib := newSharedIdentifyHandlerWithReleases(t, root, ident, releases, 100)

	body := map[string]any{"library_id": lib.ID, "paths": paths}
	if w := postIdentifyAlbum(t, r, body); w.Code != http.StatusOK {
		t.Fatalf("first identify-album: got %d: %s", w.Code, w.Body.String())
	}
	afterFirst := releases.calls.Load()
	if afterFirst == 0 {
		t.Fatal("expected the first run to fetch at least one tracklist")
	}

	if w := postIdentifyAlbum(t, r, body); w.Code != http.StatusOK {
		t.Fatalf("second identify-album: got %d: %s", w.Code, w.Body.String())
	}

	if got := releases.calls.Load(); got != afterFirst {
		t.Fatalf("second run refetched tracklists: %d calls after the first, %d after the second",
			afterFirst, got)
	}
	if got := fpcalcRuns(t, bin); got != len(paths) {
		t.Fatalf("second run re-fingerprinted: expected %d runs, got %d", len(paths), got)
	}
}

// Without the release cache the tracklists are refetched every run — the guard
// that the test above observes the cache rather than some incidental behaviour.
func TestRepeatedIdentifyAlbumWithoutAReleaseCacheRefetches(t *testing.T) {
	root := t.TempDir()
	paths := []string{"01.mp3", "02.mp3"}
	for _, n := range paths {
		if err := os.WriteFile(filepath.Join(root, n), []byte("audio-"+n), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var lookups atomic.Int32
	bin := fakeFpcalcBin(t)
	srv := countingAcoustIDServer(t, &lookups)
	ident := newRealIdentifier(t, bin, srv, identify.NewCache(100))
	releases := &countingReleaseLookup{}
	// Size 0 disables the release cache.
	r, lib := newSharedIdentifyHandlerWithReleases(t, root, ident, releases, 0)

	body := map[string]any{"library_id": lib.ID, "paths": paths}
	if w := postIdentifyAlbum(t, r, body); w.Code != http.StatusOK {
		t.Fatalf("first: got %d", w.Code)
	}
	afterFirst := releases.calls.Load()
	if w := postIdentifyAlbum(t, r, body); w.Code != http.StatusOK {
		t.Fatalf("second: got %d", w.Code)
	}

	if got := releases.calls.Load(); got != afterFirst*2 {
		t.Fatalf("expected tracklists to be refetched without a cache: %d then %d",
			afterFirst, got)
	}
}

// The exact reported scenario: identify ONE song on its own, then run album
// identify over a selection that includes it. That one file must not be
// fingerprinted or looked up a second time, even though its neighbours are
// cold.
func TestIdentifyOneSongThenAlbumReusesThatSongsAnswer(t *testing.T) {
	root := t.TempDir()
	paths := []string{"01.mp3", "02.mp3", "03.mp3"}
	for _, n := range paths {
		if err := os.WriteFile(filepath.Join(root, n), []byte("audio-"+n), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var lookups atomic.Int32
	bin := fakeFpcalcBin(t)
	srv := countingAcoustIDServer(t, &lookups)
	ident := newRealIdentifier(t, bin, srv, identify.NewCache(100))
	r, lib := newSharedIdentifyHandlerWithReleases(t, root, ident, &countingReleaseLookup{}, 100)

	// One song, on its own.
	w := postIdentify(t, r, map[string]any{"library_id": lib.ID, "paths": []string{"01.mp3"}})
	if w.Code != http.StatusOK {
		t.Fatalf("single identify: got %d: %s", w.Code, w.Body.String())
	}
	if got := fpcalcRuns(t, bin); got != 1 {
		t.Fatalf("expected 1 fingerprint run for one song, got %d", got)
	}
	if got := lookups.Load(); got != 1 {
		t.Fatalf("expected 1 AcoustID lookup, got %d", got)
	}

	// Now the whole album, which includes that song.
	w = postIdentifyAlbum(t, r, map[string]any{"library_id": lib.ID, "paths": paths})
	if w.Code != http.StatusOK {
		t.Fatalf("album identify: got %d: %s", w.Code, w.Body.String())
	}

	// Only the two cold files may be fingerprinted: 1 (single) + 2 (new) = 3.
	if got := fpcalcRuns(t, bin); got != 3 {
		t.Fatalf("expected 3 total fingerprint runs (01 reused), got %d", got)
	}
	if got := lookups.Load(); got != 3 {
		t.Fatalf("expected 3 total AcoustID lookups (01 reused), got %d", got)
	}
}

// And the tradeoff that WILL look like a cache miss to a user: saving tags
// rewrites the file, so mtime moves and the fingerprint answer is dropped.
func TestSavingTagsBetweenRunsInvalidatesThatFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "01.mp3")
	if err := os.WriteFile(file, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	var lookups atomic.Int32
	bin := fakeFpcalcBin(t)
	srv := countingAcoustIDServer(t, &lookups)
	ident := newRealIdentifier(t, bin, srv, identify.NewCache(100))
	r, lib := newSharedIdentifyHandlerWithReleases(t, root, ident, &countingReleaseLookup{}, 100)

	body := map[string]any{"library_id": lib.ID, "paths": []string{"01.mp3"}}
	if w := postIdentify(t, r, body); w.Code != http.StatusOK {
		t.Fatalf("first: got %d", w.Code)
	}
	// Stand in for a tag write: the bytes change, so size and mtime move.
	if err := os.WriteFile(file, []byte("audio with new tags"), 0o600); err != nil {
		t.Fatal(err)
	}
	if w := postIdentify(t, r, body); w.Code != http.StatusOK {
		t.Fatalf("second: got %d", w.Code)
	}

	if got := lookups.Load(); got != 2 {
		t.Fatalf("expected the rewritten file to be looked up again, got %d", got)
	}
}
