package subsonic

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
)

// These tests pin the "easy 90%" of the image-cache eviction bug: every handler
// that drops an entity's stored cover source must also drop that entity's cached
// derivatives (<cacheDir>/<kind>/<key>/), which are keyed identically. Without
// the mirror the derivatives leak on disk forever. See imagecache.Cache.Delete.

// cacheEntryDirExists reports whether the imagecache directory for one entity
// (<cacheDir>/<kind>/<key>) is present.
func cacheEntryDirExists(t *testing.T, cacheDir, kind, key string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(cacheDir, kind, key))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("stat cache entry %s/%s: %v", kind, key, err)
	}
	return err == nil
}

// seedCacheDir plants a derivative-like file under <cacheDir>/<kind>/<key> so a
// test can assert the handler removes an entry getCoverArt would not rebuild on
// its own — e.g. the artist name-hash slot that sits behind a winning MBID cover.
func seedCacheDir(t *testing.T, cacheDir, kind, key string) {
	t.Helper()
	dir := filepath.Join(cacheDir, kind, key)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir cache entry %s/%s: %v", kind, key, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "front.deadbeef.200.jpg"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed cache file %s/%s: %v", kind, key, err)
	}
}

// requireSeededDerivative fails the test unless a derivative is cached for the
// entity — a guard so an eviction assertion can't pass vacuously against an
// empty cache tree.
func requireSeededDerivative(t *testing.T, cacheDir, kind, key string) {
	t.Helper()
	if names := cachedDerivativeNames(t, cacheDir, kind, key); len(names) == 0 {
		t.Fatalf("precondition: expected a cached derivative under %s/%s", kind, key)
	}
}

func TestDeleteInternetRadioStationEvictsThumbnails(t *testing.T) {
	s := testStore(t)
	as := assetstore.New(t.TempDir())
	cacheDir := t.TempDir()
	srv := serveCovers(t, s, as, cacheDir)

	const streamURL = "http://evict-radio-delete"
	body, ct := buildMultipart(t, map[string]string{"name": "R1", "streamUrl": streamURL}, realPNG(t, 64, 64), "c.png")
	resp, err := http.Post(srv.URL+"/rest/createInternetRadioStation.view", ct, body)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	var st model.InternetRadioStation
	s.DB().First(&st)

	key := assetkey.Radio(streamURL)
	if r := getCover(t, fmt.Sprintf("%s/rest/getCoverArt.view?id=rs-%d", srv.URL, st.ID), ""); r.StatusCode != http.StatusOK {
		t.Fatalf("seed getCoverArt status = %d, want 200", r.StatusCode)
	}
	requireSeededDerivative(t, cacheDir, assetstore.KindRadio, key)

	env := getJSON(t, srv.URL, fmt.Sprintf("/rest/deleteInternetRadioStation.view?id=rs-%d", st.ID))
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("delete status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}

	if cacheEntryDirExists(t, cacheDir, assetstore.KindRadio, key) {
		t.Fatal("radio thumbnails not evicted after station delete")
	}
}

func TestUpdateInternetRadioStationCoverClearEvictsThumbnails(t *testing.T) {
	s := testStore(t)
	as := assetstore.New(t.TempDir())
	cacheDir := t.TempDir()
	srv := serveCovers(t, s, as, cacheDir)

	const streamURL = "http://evict-radio-clear"
	body, ct := buildMultipart(t, map[string]string{"name": "R1", "streamUrl": streamURL}, realPNG(t, 64, 64), "c.png")
	resp, err := http.Post(srv.URL+"/rest/createInternetRadioStation.view", ct, body)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	var st model.InternetRadioStation
	s.DB().First(&st)

	key := assetkey.Radio(streamURL)
	if r := getCover(t, fmt.Sprintf("%s/rest/getCoverArt.view?id=rs-%d", srv.URL, st.ID), ""); r.StatusCode != http.StatusOK {
		t.Fatalf("seed getCoverArt status = %d, want 200", r.StatusCode)
	}
	requireSeededDerivative(t, cacheDir, assetstore.KindRadio, key)

	clear, cct := buildMultipart(t, map[string]string{
		"id":         fmt.Sprintf("rs-%d", st.ID),
		"name":       "R1",
		"streamUrl":  streamURL,
		"coverClear": "true",
	}, nil, "")
	resp2, err := http.Post(srv.URL+"/rest/updateInternetRadioStation.view", cct, clear)
	if err != nil {
		t.Fatal(err)
	}
	env := decodeRadio(t, resp2)
	_ = resp2.Body.Close()
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("clear status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}

	if cacheEntryDirExists(t, cacheDir, assetstore.KindRadio, key) {
		t.Fatal("radio thumbnails not evicted after cover clear")
	}
}

func TestUpdateInternetRadioStationURLChangeEvictsOldKeyThumbnails(t *testing.T) {
	s := testStore(t)
	as := assetstore.New(t.TempDir())
	cacheDir := t.TempDir()
	srv := serveCovers(t, s, as, cacheDir)

	const oldURL = "http://evict-old"
	const newURL = "http://evict-new"
	body, ct := buildMultipart(t, map[string]string{"name": "ReKey FM", "streamUrl": oldURL}, realPNG(t, 64, 64), "c.png")
	resp, err := http.Post(srv.URL+"/rest/createInternetRadioStation.view", ct, body)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	var st model.InternetRadioStation
	s.DB().First(&st)

	oldKey := assetkey.Radio(oldURL)
	if r := getCover(t, fmt.Sprintf("%s/rest/getCoverArt.view?id=rs-%d", srv.URL, st.ID), ""); r.StatusCode != http.StatusOK {
		t.Fatalf("seed getCoverArt status = %d, want 200", r.StatusCode)
	}
	requireSeededDerivative(t, cacheDir, assetstore.KindRadio, oldKey)

	// URL change with no new cover: the asset store re-keys, but the image cache
	// has no re-key, so the old key's derivatives must be evicted.
	upd, uct := buildMultipart(t, map[string]string{
		"id":        fmt.Sprintf("rs-%d", st.ID),
		"name":      "ReKey FM",
		"streamUrl": newURL,
	}, nil, "")
	resp2, err := http.Post(srv.URL+"/rest/updateInternetRadioStation.view", uct, upd)
	if err != nil {
		t.Fatal(err)
	}
	env := decodeRadio(t, resp2)
	_ = resp2.Body.Close()
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("update status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}

	if cacheEntryDirExists(t, cacheDir, assetstore.KindRadio, oldKey) {
		t.Fatal("old-key radio thumbnails not evicted after URL change")
	}
}

func TestDeletePlaylistEvictsThumbnails(t *testing.T) {
	s := testStore(t)
	pl, _ := s.CreatePlaylist("Temp", "admin", false, nil)
	as := assetstore.New(t.TempDir())
	cacheDir := t.TempDir()
	srv := serveCovers(t, s, as, cacheDir)

	key := assetkey.PlaylistOf(pl)
	if err := as.PutManual(assetstore.KindPlaylist, key, "png", realPNG(t, 64, 64)); err != nil {
		t.Fatal(err)
	}
	if r := getCover(t, srv.URL+"/rest/getCoverArt.view?id="+encodePlaylistID(pl.ID), ""); r.StatusCode != http.StatusOK {
		t.Fatalf("seed getCoverArt status = %d, want 200", r.StatusCode)
	}
	requireSeededDerivative(t, cacheDir, assetstore.KindPlaylist, key)

	env := getJSON(t, srv.URL, "/rest/deletePlaylist.view?id="+encodePlaylistID(pl.ID))
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("delete status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}

	if cacheEntryDirExists(t, cacheDir, assetstore.KindPlaylist, key) {
		t.Fatal("playlist thumbnails not evicted after playlist delete")
	}
}

func TestUpdatePlaylistCoverClearEvictsThumbnails(t *testing.T) {
	s := testStore(t)
	pl, _ := s.CreatePlaylist("Mix", "admin", false, nil)
	as := assetstore.New(t.TempDir())
	cacheDir := t.TempDir()
	srv := serveCovers(t, s, as, cacheDir)

	key := assetkey.PlaylistOf(pl)
	if err := as.PutManual(assetstore.KindPlaylist, key, "png", realPNG(t, 64, 64)); err != nil {
		t.Fatal(err)
	}
	if r := getCover(t, srv.URL+"/rest/getCoverArt.view?id="+encodePlaylistID(pl.ID), ""); r.StatusCode != http.StatusOK {
		t.Fatalf("seed getCoverArt status = %d, want 200", r.StatusCode)
	}
	requireSeededDerivative(t, cacheDir, assetstore.KindPlaylist, key)

	body, ct := buildMultipart(t, map[string]string{
		"playlistId": encodePlaylistID(pl.ID),
		"coverClear": "true",
	}, nil, "")
	resp, err := http.Post(srv.URL+"/rest/updatePlaylist.view", ct, body)
	if err != nil {
		t.Fatal(err)
	}
	env := decodePlaylist(t, resp)
	_ = resp.Body.Close()
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("clear status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}

	if cacheEntryDirExists(t, cacheDir, assetstore.KindPlaylist, key) {
		t.Fatal("playlist thumbnails not evicted after cover clear")
	}
}

func TestUpdateArtistCoverClearEvictsBothSlotThumbnails(t *testing.T) {
	s := testStore(t)
	artist := model.Artist{Name: "Evict", NameNorm: "evict", MBArtistID: "mbid-evict"}
	if err := s.DB().Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}
	as := assetstore.New(t.TempDir())
	cacheDir := t.TempDir()
	srv := serveCovers(t, s, as, cacheDir)

	primaryKey := assetkey.ArtistOf(&artist)
	nameHashKey := assetkey.Artist("", artist.NameNorm)
	if primaryKey == nameHashKey {
		t.Fatal("test needs distinct primary and name-hash keys")
	}

	if err := as.PutManual(assetstore.KindArtist, primaryKey, "png", realPNG(t, 64, 64)); err != nil {
		t.Fatal(err)
	}
	if r := getCover(t, srv.URL+"/rest/getCoverArt.view?id="+encodeArtistID(artist.ID), ""); r.StatusCode != http.StatusOK {
		t.Fatalf("seed getCoverArt status = %d, want 200", r.StatusCode)
	}
	requireSeededDerivative(t, cacheDir, assetstore.KindArtist, primaryKey)
	// The name-hash slot loses to the MBID cover, so getCoverArt never builds it;
	// plant it directly to prove the clear handler evicts that slot too.
	seedCacheDir(t, cacheDir, assetstore.KindArtist, nameHashKey)

	body, ct := buildMultipart(t, map[string]string{
		"id":         encodeArtistID(artist.ID),
		"coverClear": "true",
	}, nil, "")
	if status, code := postArtist(t, srv.URL, body, ct); status != "ok" {
		t.Fatalf("clear status=%s code=%d", status, code)
	}

	if cacheEntryDirExists(t, cacheDir, assetstore.KindArtist, primaryKey) {
		t.Fatal("artist primary-key thumbnails not evicted after cover clear")
	}
	if cacheEntryDirExists(t, cacheDir, assetstore.KindArtist, nameHashKey) {
		t.Fatal("artist name-hash-slot thumbnails not evicted after cover clear")
	}
}

func TestUpdateAlbumCoverClearEvictsThumbnails(t *testing.T) {
	s := testStore(t)
	album := model.Album{Name: "Amnesiac", NameNorm: "amnesiac", AlbumArtistNorm: "radiohead"}
	if err := s.DB().Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	as := assetstore.New(t.TempDir())
	cacheDir := t.TempDir()
	srv := serveCovers(t, s, as, cacheDir)

	key := assetkey.AlbumOf(&album)
	if err := as.PutManual(assetstore.KindAlbum, key, "png", realPNG(t, 64, 64)); err != nil {
		t.Fatal(err)
	}
	if r := getCover(t, srv.URL+"/rest/getCoverArt.view?id="+encodeAlbumID(album.ID), ""); r.StatusCode != http.StatusOK {
		t.Fatalf("seed getCoverArt status = %d, want 200", r.StatusCode)
	}
	requireSeededDerivative(t, cacheDir, assetstore.KindAlbum, key)

	body, ct := buildMultipart(t, map[string]string{
		"id":         encodeAlbumID(album.ID),
		"coverClear": "true",
	}, nil, "")
	if status, code := postAlbum(t, srv.URL, body, ct); status != "ok" {
		t.Fatalf("clear status=%s code=%d", status, code)
	}

	if cacheEntryDirExists(t, cacheDir, assetstore.KindAlbum, key) {
		t.Fatal("album thumbnails not evicted after cover clear")
	}
}

func TestUpdateGenreCoverClearEvictsThumbnails(t *testing.T) {
	s := testStore(t)
	genre := model.Genre{Name: "Blues"}
	if err := s.DB().Create(&genre).Error; err != nil {
		t.Fatal(err)
	}
	as := assetstore.New(t.TempDir())
	cacheDir := t.TempDir()
	srv := serveCovers(t, s, as, cacheDir)

	key := assetkey.GenreOf(&genre)
	if err := as.PutManual(assetstore.KindGenre, key, "png", realPNG(t, 64, 64)); err != nil {
		t.Fatal(err)
	}
	if r := getCover(t, srv.URL+"/rest/getCoverArt.view?id="+encodeGenreID(genre.ID), ""); r.StatusCode != http.StatusOK {
		t.Fatalf("seed getCoverArt status = %d, want 200", r.StatusCode)
	}
	requireSeededDerivative(t, cacheDir, assetstore.KindGenre, key)

	body, ct := buildMultipart(t, map[string]string{
		"id":         encodeGenreID(genre.ID),
		"coverClear": "true",
	}, nil, "")
	if status, code := postGenre(t, srv.URL, body, ct); status != "ok" {
		t.Fatalf("clear status=%s code=%d", status, code)
	}

	if cacheEntryDirExists(t, cacheDir, assetstore.KindGenre, key) {
		t.Fatal("genre thumbnails not evicted after cover clear")
	}
}
