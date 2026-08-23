package subsonic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/imagecache"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/gorilla/mux"
	"go.senan.xyz/taglib"
)

func TestGetCoverArtGeneratesWhenMissing(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	// Seed one album with no cover_path and no tracks with embedded cover.
	album := model.Album{
		Name:            "19",
		NameNorm:        "19",
		AlbumArtistNorm: "adele",
	}
	if err := db.Create(&album).Error; err != nil {
		t.Fatalf("create album: %v", err)
	}

	cacheDir := t.TempDir() + "/generated-covers"
	r := mux.NewRouter()
	Register(r, s, assetstore.New(t.TempDir()), imagecache.New(cacheDir), nil)
	srv := httptest.NewServer(r)
	defer srv.Close()

	url := fmt.Sprintf("%s/rest/getCoverArt.view?v=1.16.1&c=test&f=json&id=al-%d&size=200", srv.URL, album.ID)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "image/jpeg") {
		t.Errorf("Content-Type = %q, want image/jpeg* (this client sent no Accept)", ct)
	}
	cfg, format := decodeServedCover(t, resp)
	if format != "jpeg" {
		t.Errorf("format = %q, want jpeg", format)
	}
	// 200 quantizes up to the 256 bucket.
	if want := quantizeCoverSize(200); cfg.Width != want {
		t.Errorf("generated cover width = %d, want %d", cfg.Width, want)
	}

	// The derivative is cached under the album's identity, in the size bucket.
	names := cachedDerivativeNames(t, cacheDir, assetstore.KindAlbum, assetkey.AlbumOf(&album))
	if len(names) != 1 || !strings.HasSuffix(names[0], ".256.jpg") {
		t.Errorf("cached derivatives = %v, want one generated.<fingerprint>.256.jpg", names)
	}
}

func TestGetCoverArtRadioUploadedServed(t *testing.T) {
	s := testStore(t)
	st, _ := s.CreateInternetRadioStation("R1", "http://r1", "")

	// Store a PNG into the asset store keyed by assetkey.Radio(streamURL).
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	assetDir := t.TempDir()
	as := assetstore.New(assetDir)
	if err := as.PutManual(assetstore.KindRadio, assetkey.Radio(st.StreamURL), "png", buf.Bytes()); err != nil {
		t.Fatal(err)
	}

	r := mux.NewRouter()
	Register(r, s, as, imagecache.New(t.TempDir()), nil)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=rs-%d", srv.URL, st.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	// The uploaded 4x4 image is never upscaled, so its derivative keeps those
	// dimensions — proof the upload was served rather than the generated cover.
	cfg, _ := decodeServedCover(t, resp)
	if cfg.Width != 4 || cfg.Height != 4 {
		t.Errorf("served %dx%d, want 4x4 (a derivative of the uploaded image)", cfg.Width, cfg.Height)
	}
}

func TestGetCoverArtRadioFallbackGenerated(t *testing.T) {
	s := testStore(t)
	st, _ := s.CreateInternetRadioStation("Fallback FM", "http://f", "")

	cacheDir := t.TempDir()
	r := mux.NewRouter()
	Register(r, s, assetstore.New(t.TempDir()), imagecache.New(cacheDir), nil)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=rs-%d&size=256", srv.URL, st.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	cfg, _ := decodeServedCover(t, resp)
	if cfg.Width != 256 {
		t.Errorf("generated cover width = %d, want 256", cfg.Width)
	}
	names := cachedDerivativeNames(t, cacheDir, assetstore.KindRadio, assetkey.Radio(st.StreamURL))
	if len(names) != 1 || !strings.HasPrefix(names[0], "generated.") {
		t.Errorf("cached derivatives = %v, want one generated.<fingerprint>.256.<ext>", names)
	}
}

// TestGetCoverArtPlaylistFallbackGenerated verifies a playlist (which has no
// artwork of its own) falls through to the name-seeded generated cover, the
// same mechanism used for artists and radio stations without an image.
func TestGetCoverArtPlaylistFallbackGenerated(t *testing.T) {
	s := testStore(t)
	pl, err := s.CreatePlaylist("Road Trip Mix", "admin", false, nil)
	if err != nil {
		t.Fatalf("create playlist: %v", err)
	}

	cacheDir := t.TempDir()
	r := mux.NewRouter()
	Register(r, s, assetstore.New(t.TempDir()), imagecache.New(cacheDir), nil)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=pl-%d&size=256", srv.URL, pl.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	cfg, _ := decodeServedCover(t, resp)
	if cfg.Width != 256 {
		t.Errorf("generated cover width = %d, want 256", cfg.Width)
	}
	names := cachedDerivativeNames(t, cacheDir, assetstore.KindPlaylist, assetkey.PlaylistOf(pl))
	if len(names) != 1 || !strings.HasPrefix(names[0], "generated.") {
		t.Errorf("cached derivatives = %v, want one generated.<fingerprint>.256.<ext>", names)
	}
}

// TestGetCoverArtSetsNoCacheHeader guards against a real bug: getCoverArt
// responses had no cache-control header, so browsers could heuristically
// cache the generated-avatar fallback (served before an artist has a fetched
// image) and keep serving it from cache after the real image is later
// fetched and stored, since the URL doesn't change.
func TestGetCoverArtSetsNoCacheHeader(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y"}
	if err := db.Create(&album).Error; err != nil {
		t.Fatalf("create album: %v", err)
	}

	r := mux.NewRouter()
	Register(r, s, assetstore.New(t.TempDir()), imagecache.New(t.TempDir()), nil)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d", srv.URL, album.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-cache")
	}
}

// newGuardedTestServer registers /rest with the media path guard restricted to
// the given roots, standing in for the configured libraries.
func newGuardedTestServer(t *testing.T, s *store.Store, roots ...string) *httptest.Server {
	t.Helper()
	r := mux.NewRouter()
	Register(r, s, assetstore.New(t.TempDir()), imagecache.New(t.TempDir()), nil,
		WithMediaRoots(roots...))
	return httptest.NewServer(r)
}

// A track row whose file_path points outside every configured library must not be
// served. Nothing in the request supplies that path — it comes from the DB — so
// this is the enforcement of an assumption the //nolint:gosec on the file open
// previously only asserted: a stale row or a metadata-editor bug is enough to
// name any file the server process can read.
func TestStreamRefusesFileOutsideEveryLibraryRoot(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	base := t.TempDir()
	libRoot := filepath.Join(base, "music")
	if err := os.MkdirAll(libRoot, 0o750); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(base, "secret.env")
	if err := os.WriteFile(secret, []byte("DB_PASSWORD=hunter2"), 0o600); err != nil {
		t.Fatal(err)
	}

	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y"}
	if err := db.Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	track := model.Track{AlbumID: album.ID, Filename: "secret.env", FilePath: secret}
	if err := db.Create(&track).Error; err != nil {
		t.Fatal(err)
	}

	srv := newGuardedTestServer(t, s, libRoot)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/stream.view?id=tr-%d", srv.URL, track.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "hunter2") {
		t.Fatal("stream served a file outside every library root")
	}
}

// The guard must not break the normal case: a track inside a configured root
// streams as before.
func TestStreamServesFileInsideLibraryRoot(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	libRoot := t.TempDir()
	song := filepath.Join(libRoot, "a.mp3")
	if err := os.WriteFile(song, []byte("song-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y"}
	if err := db.Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	track := model.Track{AlbumID: album.ID, Filename: "a.mp3", FilePath: song}
	if err := db.Create(&track).Error; err != nil {
		t.Fatal(err)
	}

	srv := newGuardedTestServer(t, s, libRoot)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/stream.view?id=tr-%d", srv.URL, track.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "song-bytes" {
		t.Errorf("served %q, want the track's file", body)
	}
}

// An album cover_path outside every library root is the same defect on the cover
// path: the bytes of an arbitrary file would be re-encoded into a JPEG and
// served. The request must fall through to the generated cover instead.
func TestGetCoverArtRefusesCoverPathOutsideLibraryRoot(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	base := t.TempDir()
	libRoot := filepath.Join(base, "music")
	if err := os.MkdirAll(libRoot, 0o750); err != nil {
		t.Fatal(err)
	}
	// A real PNG living outside the library: if the guard is missing, its 4x4
	// dimensions come back instead of the 256px generated cover.
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(base, "outside.png")
	if err := os.WriteFile(outside, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y", CoverPath: outside}
	if err := db.Create(&album).Error; err != nil {
		t.Fatal(err)
	}

	srv := newGuardedTestServer(t, s, libRoot)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d&size=256", srv.URL, album.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (the generated cover)", resp.StatusCode)
	}
	cfg, _ := decodeServedCover(t, resp)
	if cfg.Width == 4 {
		t.Fatal("served a derivative of a file outside every library root")
	}
	if cfg.Width != 256 {
		t.Errorf("served width %d, want the 256px generated cover", cfg.Width)
	}
}

// A cover file inside the library must still be served — the guard is about
// provenance, not about disabling folder covers.
func TestGetCoverArtServesCoverPathInsideLibraryRoot(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	libRoot := t.TempDir()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	cover := filepath.Join(libRoot, "cover.png")
	if err := os.WriteFile(cover, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y", CoverPath: cover}
	if err := db.Create(&album).Error; err != nil {
		t.Fatal(err)
	}

	srv := newGuardedTestServer(t, s, libRoot)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d&size=256", srv.URL, album.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	// Never upscaled, so 4x4 proves the real cover was used, not the generated one.
	cfg, _ := decodeServedCover(t, resp)
	if cfg.Width != 4 {
		t.Errorf("served width %d, want 4 (a derivative of the in-library cover)", cfg.Width)
	}
}

// An album's embedded-cover source is an audio file path from the DB too, so it
// needs the same containment check as cover_path.
func TestGetCoverArtRefusesEmbeddedSourceOutsideLibraryRoot(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	base := t.TempDir()
	libRoot := filepath.Join(base, "music")
	if err := os.MkdirAll(libRoot, 0o750); err != nil {
		t.Fatal(err)
	}
	// A real audio file with a real embedded front cover, living outside the
	// library: its distinctive 2:1 shape is what proves whether it was read.
	outside := embeddedFixture(t, base, "outside.flac",
		embeddedPic{"Front Cover", realPNG(t, 300, 150)})

	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y", HasEmbeddedCover: true}
	if err := db.Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	track := model.Track{AlbumID: album.ID, Filename: "outside.flac", FilePath: outside, HasEmbeddedCover: true}
	if err := db.Create(&track).Error; err != nil {
		t.Fatal(err)
	}

	srv := newGuardedTestServer(t, s, libRoot)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d&size=256", srv.URL, album.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (the generated cover)", resp.StatusCode)
	}
	// Generated covers are square; the out-of-library fixture's art is 2:1, so
	// the aspect ratio says which source was actually read regardless of the
	// size bucket the derivative was scaled into.
	cfg, _ := decodeServedCover(t, resp)
	if cfg.Width != cfg.Height {
		t.Fatalf("served a %dx%d (non-square) cover: the embedded art of a file outside every library root was read", cfg.Width, cfg.Height)
	}
}

// A manually uploaded cover lives in aether's own asset store, under the data
// dir and therefore outside every library root by construction. The library
// guard is about paths that came out of the DB naming user media, so applying it
// to the asset store would silently fall every upload through to the generated
// cover. Covers all the entities whose covers can be uploaded through the UI.
func TestGetCoverArtServesUploadedCoverWithLibraryGuard(t *testing.T) {
	// seed stores a 4x4 upload for one entity kind and returns its cover-art id.
	tests := []struct {
		name string
		seed func(t *testing.T, s *store.Store, as *assetstore.Store) string
	}{
		{"playlist", func(t *testing.T, s *store.Store, as *assetstore.Store) string {
			pl, err := s.CreatePlaylist("Mix", "admin", false, nil)
			if err != nil {
				t.Fatal(err)
			}
			putUpload(t, as, assetstore.KindPlaylist, assetkey.PlaylistOf(pl))
			return encodePlaylistID(pl.ID)
		}},
		{"album", func(t *testing.T, s *store.Store, as *assetstore.Store) string {
			album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y"}
			if err := s.DB().Create(&album).Error; err != nil {
				t.Fatal(err)
			}
			putUpload(t, as, assetstore.KindAlbum, assetkey.AlbumOf(&album))
			return encodeAlbumID(album.ID)
		}},
		{"artist", func(t *testing.T, s *store.Store, as *assetstore.Store) string {
			artist := model.Artist{Name: "A", NameNorm: "a"}
			if err := s.DB().Create(&artist).Error; err != nil {
				t.Fatal(err)
			}
			putUpload(t, as, assetstore.KindArtist, assetkey.Artist("", artist.NameNorm))
			return encodeArtistID(artist.ID)
		}},
		{"genre", func(t *testing.T, s *store.Store, as *assetstore.Store) string {
			genre := model.Genre{Name: "Jazz"}
			if err := s.DB().Create(&genre).Error; err != nil {
				t.Fatal(err)
			}
			putUpload(t, as, assetstore.KindGenre, assetkey.GenreOf(&genre))
			return encodeGenreID(genre.ID)
		}},
		{"radio", func(t *testing.T, s *store.Store, as *assetstore.Store) string {
			st, err := s.CreateInternetRadioStation("R", "http://stream.test/x", "")
			if err != nil {
				t.Fatal(err)
			}
			putUpload(t, as, assetstore.KindRadio, assetkey.Radio(st.StreamURL))
			return encodeRadioID(st.ID)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testStore(t)
			if err := s.CreateLibrary(&model.Library{Name: "L", Path: t.TempDir()}); err != nil {
				t.Fatal(err)
			}
			as := assetstore.New(t.TempDir())
			id := tc.seed(t, s, as)

			r := mux.NewRouter()
			Register(r, s, as, imagecache.New(t.TempDir()), nil, WithLibraryRoots(s.LibraryRoots))
			srv := httptest.NewServer(r)
			defer srv.Close()

			resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=%s&size=256", srv.URL, id))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			// Sources are never upscaled, so 4x4 proves the upload was served
			// rather than the 256px generated fallback.
			cfg, _ := decodeServedCover(t, resp)
			if cfg.Width != uploadEdge {
				t.Errorf("served width %d, want %d: the uploaded cover was refused by the library guard", cfg.Width, uploadEdge)
			}
		})
	}
}

// uploadEdge is the edge length of the upload fixture putUpload stores — small
// enough that a derivative of it is never confused with a generated cover.
const uploadEdge = 4

func putUpload(t *testing.T, as *assetstore.Store, kind, key string) {
	t.Helper()
	if err := as.PutManual(kind, key, "png", realPNG(t, uploadEdge, uploadEdge)); err != nil {
		t.Fatal(err)
	}
}

// With no roots configured the guard is not installed at all — auth method
// "none" style single-user setups and every existing test construct the handler
// that way, and a server with no libraries yet must not 404 its own covers.
func TestMediaGuardAbsentWhenNoRootsConfigured(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	song := filepath.Join(t.TempDir(), "a.mp3")
	if err := os.WriteFile(song, []byte("song-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y"}
	if err := db.Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	track := model.Track{AlbumID: album.ID, Filename: "a.mp3", FilePath: song}
	if err := db.Create(&track).Error; err != nil {
		t.Fatal(err)
	}

	srv := newTestServer(t, s)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/stream.view?id=tr-%d", srv.URL, track.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 with no guard configured", resp.StatusCode)
	}
}

// Libraries are created at runtime through the settings UI, so the guard cannot
// be a snapshot taken at registration: a library added afterwards must have its
// files served without restarting the server.
func TestMediaGuardPicksUpLibrariesAddedAfterRegistration(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	firstRoot := t.TempDir()
	if err := s.CreateLibrary(&model.Library{Name: "First", Path: firstRoot}); err != nil {
		t.Fatal(err)
	}

	r := mux.NewRouter()
	Register(r, s, assetstore.New(t.TempDir()), imagecache.New(t.TempDir()), nil,
		WithLibraryRoots(s.LibraryRoots))
	srv := httptest.NewServer(r)
	defer srv.Close()

	// Now a second library appears, with a track in it.
	lateRoot := t.TempDir()
	if err := s.CreateLibrary(&model.Library{Name: "Late", Path: lateRoot}); err != nil {
		t.Fatal(err)
	}
	song := filepath.Join(lateRoot, "late.mp3")
	if err := os.WriteFile(song, []byte("late-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y"}
	if err := db.Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	track := model.Track{AlbumID: album.ID, Filename: "late.mp3", FilePath: song}
	if err := db.Create(&track).Error; err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(fmt.Sprintf("%s/rest/stream.view?id=tr-%d", srv.URL, track.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: a library added after registration must be served", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "late-bytes" {
		t.Errorf("served %q, want the newly added library's file", body)
	}
}

// The dynamic guard must still refuse: a path outside every library, however many
// times the root set is refreshed, stays refused.
func TestMediaGuardWithLibraryRootsStillRefusesOutsidePaths(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	base := t.TempDir()
	libRoot := filepath.Join(base, "music")
	if err := os.MkdirAll(libRoot, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateLibrary(&model.Library{Name: "L", Path: libRoot}); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(base, "secret.env")
	if err := os.WriteFile(secret, []byte("DB_PASSWORD=hunter2"), 0o600); err != nil {
		t.Fatal(err)
	}

	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y"}
	if err := db.Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	track := model.Track{AlbumID: album.ID, Filename: "secret.env", FilePath: secret}
	if err := db.Create(&track).Error; err != nil {
		t.Fatal(err)
	}

	r := mux.NewRouter()
	Register(r, s, assetstore.New(t.TempDir()), imagecache.New(t.TempDir()), nil,
		WithLibraryRoots(s.LibraryRoots))
	srv := httptest.NewServer(r)
	defer srv.Close()

	// Twice: the second request exercises the path where the root set has already
	// been refreshed once, which must not turn into an allow.
	for i := range 2 {
		resp, err := http.Get(fmt.Sprintf("%s/rest/stream.view?id=tr-%d", srv.URL, track.ID))
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "hunter2") {
			t.Fatalf("request %d served a file outside every library root", i+1)
		}
	}
}

// Track IDs are not stable across rescans: a dropped-and-rebuilt DB reassigns
// tr-N to a different song while the stream URL stays the same. http.ServeFile
// alone lets browsers heuristically cache the audio (no Cache-Control) and can
// answer 304 off Last-Modified when the reassigned file is older — either way
// the user hears the pre-rescan song. Stream responses must force revalidation
// and key their validator on which file is served, like covers do.
func TestStreamRevalidatesWhenTheServedFileChanges(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	dir := t.TempDir()
	newSong := filepath.Join(dir, "new.mp3")
	oldSong := filepath.Join(dir, "old.mp3")
	if err := os.WriteFile(newSong, []byte("new-song-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldSong, []byte("old-song-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	// The reassigned file is OLDER than the cached one — the case where a
	// Last-Modified check wrongly answers 304.
	old := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(oldSong, old, old); err != nil {
		t.Fatal(err)
	}

	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y"}
	if err := db.Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	track := model.Track{AlbumID: album.ID, Filename: "new.mp3", FilePath: newSong}
	if err := db.Create(&track).Error; err != nil {
		t.Fatal(err)
	}

	srv := newTestServer(t, s)
	defer srv.Close()

	url := fmt.Sprintf("%s/rest/stream.view?id=tr-%d", srv.URL, track.ID)

	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "new-song-bytes" {
		t.Fatalf("first fetch served %q, want the track's file", body)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q (heuristic caching replays stale audio)", got, "no-cache")
	}
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatal("no ETag on the stream response; conditional requests cannot be keyed on the served file")
	}

	// Simulate the rescan: the same track ID now points at a different, older file.
	if err := db.Model(&model.Track{}).Where("id = ?", track.ID).Update("file_path", oldSong).Error; err != nil {
		t.Fatal(err)
	}

	// A conditional re-fetch carrying the cached validators must get the new
	// song, not 304.
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("If-None-Match", etag)
	req.Header.Set("If-Modified-Since", time.Now().UTC().Format(http.TimeFormat))
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode == http.StatusNotModified {
		t.Fatal("got 304 after the file behind the ID changed; the browser keeps playing the old song")
	}
	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body2) != "old-song-bytes" {
		t.Errorf("conditional re-fetch served %q, want the reassigned file", body2)
	}
}

// Seeking relies on partial responses: the stream endpoint must keep honouring
// Range requests.
func TestStreamServesRangeRequests(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	songPath := filepath.Join(t.TempDir(), "a.mp3")
	if err := os.WriteFile(songPath, []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y"}
	if err := db.Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	track := model.Track{AlbumID: album.ID, Filename: "a.mp3", FilePath: songPath}
	if err := db.Create(&track).Error; err != nil {
		t.Fatal(err)
	}

	srv := newTestServer(t, s)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/rest/stream.view?id=tr-%d", srv.URL, track.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Range", "bytes=4-6")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "456" {
		t.Errorf("range body = %q, want %q", body, "456")
	}
}

func TestGetCoverArtRadioNotFound(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/rest/getCoverArt.view?id=rs-9999")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		SubsonicResponse struct {
			Error struct {
				Code int `json:"code"`
			} `json:"error"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.SubsonicResponse.Error.Code != 70 {
		t.Fatalf("expected code 70, got %d", body.SubsonicResponse.Error.Code)
	}
}

func TestGetCoverArtArtistServesStoredImage(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	// Seed an artist with a known MBArtistID.
	artist := model.Artist{
		Name:       "Test Artist",
		NameNorm:   "test-artist",
		MBArtistID: "mbid-art",
	}
	if err := db.Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}

	// Put an image into the asset store for this artist. Distinctive dimensions
	// identify it in the response: served covers are re-encoded derivatives, so
	// the source bytes never come back verbatim.
	assetDir := t.TempDir()
	as := assetstore.New(assetDir)
	if err := as.PutAuto(assetstore.KindArtist, "mbid-art", "png", realPNG(t, 300, 150)); err != nil {
		t.Fatalf("PutAuto: %v", err)
	}

	r := mux.NewRouter()
	Register(r, s, as, imagecache.New(t.TempDir()), nil)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=ar-%d", srv.URL, artist.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	// 300x150 is under the cap, so it is served at its own size and 2:1 shape —
	// a generated cover would be square and full-size.
	if cfg, _ := decodeServedCover(t, resp); cfg.Width != 300 || cfg.Height != 150 {
		t.Errorf("served %dx%d, want 300x150 (the stored artist image)", cfg.Width, cfg.Height)
	}
}

func TestGetCoverArtAlbumServesManagedStoreImage(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	album := model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead"}
	if err := db.Create(&album).Error; err != nil {
		t.Fatalf("create album: %v", err)
	}

	// A cover saved to aether's managed store for this album.
	assetDir := t.TempDir()
	as := assetstore.New(assetDir)
	if err := as.PutManual(assetstore.KindAlbum, assetkey.AlbumOf(&album), "png", realPNG(t, 300, 150)); err != nil {
		t.Fatalf("PutManual: %v", err)
	}

	r := mux.NewRouter()
	Register(r, s, as, imagecache.New(t.TempDir()), nil)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d", srv.URL, album.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if cfg, _ := decodeServedCover(t, resp); cfg.Width != 300 || cfg.Height != 150 {
		t.Errorf("served %dx%d, want 300x150 (the managed-store cover)", cfg.Width, cfg.Height)
	}
}

// embeddedPic is one attached picture to write into a test fixture.
type embeddedPic struct {
	typeID string
	data   []byte
}

// embeddedFixture copies the shared fixture into dir and embeds the given
// pictures, in order, as attached pictures of the named types.
func embeddedFixture(t *testing.T, dir, name string, pics ...embeddedPic) string {
	t.Helper()
	src := "../../../../internal/metadataedit/testdata/empty.flac"
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("no fixture at %s: %v", src, err)
	}
	dst := filepath.Join(dir, name)
	if err := os.WriteFile(dst, raw, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	for i, p := range pics {
		if err := taglib.WriteImageOptions(dst, p.data, i, p.typeID, "", "image/png"); err != nil {
			t.Fatalf("embed %s: %v", p.typeID, err)
		}
	}
	return dst
}

// The embedded front cover must be served even when a back cover sits ahead of
// it in the file. Reading attached picture index 0 blindly served the back scan.
func TestGetCoverArtAlbumServesEmbeddedFrontCoverNotBack(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	// Distinct shapes tell the two apart in the response: covers are served as
	// re-encoded derivatives, never as the embedded bytes verbatim.
	front := realPNG(t, 300, 150) // 2:1
	back := realPNG(t, 150, 300)  // 1:2
	trackPath := embeddedFixture(t, t.TempDir(), "01.flac",
		embeddedPic{"Back Cover", back},
		embeddedPic{"Front Cover", front},
	)

	album := model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead", HasEmbeddedCover: true}
	if err := db.Create(&album).Error; err != nil {
		t.Fatalf("create album: %v", err)
	}
	track := model.Track{AlbumID: album.ID, Filename: "01.flac", FilePath: trackPath, HasEmbeddedCover: true}
	if err := db.Create(&track).Error; err != nil {
		t.Fatalf("create track: %v", err)
	}

	r := mux.NewRouter()
	Register(r, s, assetstore.New(t.TempDir()), imagecache.New(t.TempDir()), nil)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d", srv.URL, album.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	cfg, _ := decodeServedCover(t, resp)
	if cfg.Width == 150 && cfg.Height == 300 {
		t.Fatal("served the embedded BACK cover; want the front cover")
	}
	if cfg.Width != 300 || cfg.Height != 150 {
		t.Errorf("served %dx%d, want 300x150 (the embedded front cover)", cfg.Width, cfg.Height)
	}
}

// A track carrying only a back cover has no cover art: the album must fall
// through to the generated cover rather than serving the back scan.
func TestGetCoverArtAlbumBackCoverOnlyFallsBackToGenerated(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	// 1:2, so a served derivative of it would be unmistakably non-square.
	back := realPNG(t, 150, 300)
	trackPath := embeddedFixture(t, t.TempDir(), "01.flac", embeddedPic{"Back Cover", back})

	album := model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead", HasEmbeddedCover: true}
	if err := db.Create(&album).Error; err != nil {
		t.Fatalf("create album: %v", err)
	}
	track := model.Track{AlbumID: album.ID, Filename: "01.flac", FilePath: trackPath, HasEmbeddedCover: true}
	if err := db.Create(&track).Error; err != nil {
		t.Fatalf("create track: %v", err)
	}

	r := mux.NewRouter()
	Register(r, s, assetstore.New(t.TempDir()), imagecache.New(t.TempDir()), nil)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d&size=128", srv.URL, album.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	cfg, _ := decodeServedCover(t, resp)
	if cfg.Width == 150 && cfg.Height == 300 {
		t.Fatal("served the embedded back cover; want the generated fallback cover")
	}
	// Generated covers are square, at the requested bucket.
	if want := quantizeCoverSize(128); cfg.Width != want || cfg.Height != want {
		t.Errorf("served %dx%d, want %dx%d (the generated fallback)", cfg.Width, cfg.Height, want, want)
	}
}

// An artist with no fetched or uploaded image falls back to the image found in
// the artist's folder on disk before the generated avatar.
func TestGetCoverArtArtistServesFolderImage(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	musicDir := t.TempDir()
	imgPath := filepath.Join(musicDir, "artist.jpg")
	// 2:1, so a derivative of it is unmistakably not the square generated avatar.
	if err := os.WriteFile(imgPath, realPNG(t, 300, 150), 0o600); err != nil {
		t.Fatal(err)
	}

	artist := model.Artist{Name: "Pink Floyd", NameNorm: "pink floyd", ImagePath: imgPath}
	if err := db.Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}

	r := mux.NewRouter()
	Register(r, s, assetstore.New(t.TempDir()), imagecache.New(t.TempDir()), nil)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=ar-%d", srv.URL, artist.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if cfg, _ := decodeServedCover(t, resp); cfg.Width != 300 || cfg.Height != 150 {
		t.Errorf("served %dx%d, want 300x150 (the artist-folder image)", cfg.Width, cfg.Height)
	}
}

// A stored (fetched or uploaded) image outranks the artist-folder image.
func TestGetCoverArtArtistPrefersStoredOverFolderImage(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	musicDir := t.TempDir()
	imgPath := filepath.Join(musicDir, "artist.jpg")
	// The two candidates get distinct shapes so the response says which won.
	if err := os.WriteFile(imgPath, realPNG(t, 300, 150), 0o600); err != nil {
		t.Fatal(err)
	}

	artist := model.Artist{Name: "Pink Floyd", NameNorm: "pink floyd", MBArtistID: "mbid-pf", ImagePath: imgPath}
	if err := db.Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}

	as := assetstore.New(t.TempDir())
	if err := as.PutAuto(assetstore.KindArtist, "mbid-pf", "png", realPNG(t, 150, 300)); err != nil {
		t.Fatalf("PutAuto: %v", err)
	}

	r := mux.NewRouter()
	Register(r, s, as, imagecache.New(t.TempDir()), nil)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=ar-%d", srv.URL, artist.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if cfg, _ := decodeServedCover(t, resp); cfg.Width != 150 || cfg.Height != 300 {
		t.Errorf("served %dx%d, want 150x300: the stored image should win over the folder image",
			cfg.Width, cfg.Height)
	}
}

// A recorded artist-folder image that has since disappeared must not break cover
// art: the generated avatar still gets served.
func TestGetCoverArtArtistMissingFolderImageFallsBackToGenerated(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	artist := model.Artist{
		Name:      "Pink Floyd",
		NameNorm:  "pink floyd",
		ImagePath: filepath.Join(t.TempDir(), "gone", "artist.jpg"),
	}
	if err := db.Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}

	r := mux.NewRouter()
	Register(r, s, assetstore.New(t.TempDir()), imagecache.New(t.TempDir()), nil)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=ar-%d&size=200", srv.URL, artist.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	// Generated avatars are square at the requested bucket.
	cfg, _ := decodeServedCover(t, resp)
	if want := quantizeCoverSize(200); cfg.Width != want || cfg.Height != want {
		t.Errorf("served %dx%d, want a %d square generated avatar", cfg.Width, cfg.Height, want)
	}
}

// Removing an uploaded image makes getCoverArt fall back to the (older) file in
// the music folder. http.ServeFile would answer 304 Not Modified for a browser
// holding the upload, because the fallback's mtime is older than the cached
// copy's — the user would keep seeing the deleted image until a hard refresh.
// The response must be keyed on which file is served, not on its age.
func TestGetCoverArtRevalidatesWhenTheServedFileChanges(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	musicDir := t.TempDir()
	folderImg := filepath.Join(musicDir, "artist.jpg")
	// Distinct shapes identify which file was served; covers come back as
	// re-encoded derivatives, not as the source bytes.
	if err := os.WriteFile(folderImg, realPNG(t, 300, 150), 0o600); err != nil {
		t.Fatal(err)
	}
	// Make the folder image clearly older than any uploaded one.
	old := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(folderImg, old, old); err != nil {
		t.Fatal(err)
	}

	artist := model.Artist{Name: "Pink Floyd", NameNorm: "pink floyd", ImagePath: folderImg}
	if err := db.Create(&artist).Error; err != nil {
		t.Fatal(err)
	}

	assetDir := t.TempDir()
	as := assetstore.New(assetDir)
	key := assetkey.Artist("", artist.NameNorm)
	if err := as.PutManual(assetstore.KindArtist, key, "png", realPNG(t, 150, 300)); err != nil {
		t.Fatal(err)
	}

	r := mux.NewRouter()
	Register(r, s, as, imagecache.New(t.TempDir()), nil)
	srv := httptest.NewServer(r)
	defer srv.Close()

	url := fmt.Sprintf("%s/rest/getCoverArt.view?id=ar-%d", srv.URL, artist.ID)

	// First fetch: the upload, with whatever validators the server offers.
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	if cfg, _ := decodeServedCover(t, resp); cfg.Width != 150 || cfg.Height != 300 {
		t.Fatalf("served %dx%d first, want 150x300 (the upload)", cfg.Width, cfg.Height)
	}
	_ = resp.Body.Close()
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatal("no ETag on the cover response; conditional requests cannot be keyed on the served file")
	}

	// Remove the upload — getCoverArt now falls back to the older folder file.
	if err := as.Delete(assetstore.KindArtist, key); err != nil {
		t.Fatal(err)
	}

	// A conditional re-fetch carrying the cached validators must not be told
	// "not modified": a different file is being served now.
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("If-None-Match", etag)
	req.Header.Set("If-Modified-Since", time.Now().UTC().Format(http.TimeFormat))
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode == http.StatusNotModified {
		t.Fatal("got 304 after the served file changed; the stale image stays until a hard refresh")
	}
	if cfg, _ := decodeServedCover(t, resp2); cfg.Width != 300 || cfg.Height != 150 {
		t.Errorf("served %dx%d after removal, want 300x150 (the folder image)", cfg.Width, cfg.Height)
	}
}

// getCoverArt must apply the visibility guard to playlists: the owner or public
// only, answering error 70 otherwise — no existence leak.
func TestGetCoverArtPlaylistVisibilityGuard(t *testing.T) {
	s := testStore(t)
	// Demo creates a PRIVATE playlist.
	priv, err := s.CreatePlaylist("DemoPrivate", "demo", false, nil)
	if err != nil {
		t.Fatalf("create private playlist: %v", err)
	}
	// Demo also creates a PUBLIC playlist.
	pub, err := s.CreatePlaylist("DemoPublic", "demo", true, nil)
	if err != nil {
		t.Fatalf("create public playlist: %v", err)
	}

	resolver := func(r *http.Request) (string, int) {
		if u := r.Header.Get("X-Test-User"); u != "" {
			return u, 0
		}
		return "", 40
	}
	r := mux.NewRouter()
	Register(r, s, assetstore.New(t.TempDir()), imagecache.New(t.TempDir()), resolver)
	srv := httptest.NewServer(r)
	defer srv.Close()

	// Admin requests the private playlist's cover → error 70 (no leak).
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/rest/getCoverArt.view?id=pl-%d", srv.URL, priv.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Test-User", "admin")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	// Subsonic errors are HTTP 200 with a JSON envelope, not HTTP error codes.
	// Check if we got a JSON error response or an image.
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Fatalf("admin got image response (Content-Type: %q) for demo's PRIVATE playlist cover; expected JSON error 70", contentType)
	}
	var privEnv errorEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&privEnv); err != nil {
		t.Fatal(err)
	}
	if privEnv.SubsonicResponse.Status != "failed" || privEnv.SubsonicResponse.Error == nil || privEnv.SubsonicResponse.Error.Code != 70 {
		t.Errorf("private playlist cover for foreign user → status=%q code=%v, want failed/70",
			privEnv.SubsonicResponse.Status, privEnv.SubsonicResponse.Error)
	}

	// Admin requests the public playlist's cover → ok (or 404 if no actual image exists, but NOT error 70).
	req2, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/rest/getCoverArt.view?id=pl-%d", srv.URL, pub.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	req2.Header.Set("X-Test-User", "admin")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()
	// Public playlist should return an image (generated cover) or 404, not error 70.
	contentType2 := resp2.Header.Get("Content-Type")
	if contentType2 == "application/json" {
		var env errorEnvelope
		if err := json.NewDecoder(resp2.Body).Decode(&env); err == nil {
			if env.SubsonicResponse.Status == "failed" && env.SubsonicResponse.Error != nil && env.SubsonicResponse.Error.Code == 70 {
				t.Errorf("public playlist cover returned error 70; visibility guard wrongly blocked it")
			}
		}
	}
}

func TestGetCoverArtHonoursLibraryCoverStyle(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	lib := model.Library{Name: "Styled", Path: "/styled", CoverStyle: "bauhaus"}
	if err := db.Create(&lib).Error; err != nil {
		t.Fatalf("create library: %v", err)
	}
	album := model.Album{Name: "19", NameNorm: "19", AlbumArtistNorm: "adele"}
	if err := db.Create(&album).Error; err != nil {
		t.Fatalf("create album: %v", err)
	}
	track := model.Track{AlbumID: album.ID, LibraryID: lib.ID, Filename: "a.mp3", FilePath: "/styled/a.mp3"}
	if err := db.Create(&track).Error; err != nil {
		t.Fatalf("create track: %v", err)
	}

	cacheDir := t.TempDir() + "/generated-covers"
	r := mux.NewRouter()
	Register(r, s, assetstore.New(t.TempDir()), imagecache.New(cacheDir), nil)
	srv := httptest.NewServer(r)
	defer srv.Close()

	fetch := func() []byte {
		t.Helper()
		resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d&size=128", srv.URL, album.ID))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}

	styled := fetch()

	// One generated derivative is cached for the album at this size bucket. The
	// style is part of the derivative's source fingerprint (not its filename),
	// so switching styles supersedes this file instead of serving it stale —
	// which the auto-vs-bauhaus comparison below proves.
	albumKey := assetkey.AlbumOf(&album)
	names := cachedDerivativeNames(t, cacheDir, assetstore.KindAlbum, albumKey)
	if len(names) != 1 || !strings.HasPrefix(names[0], "generated.") || !strings.Contains(names[0], ".160.") {
		t.Fatalf("cached derivatives = %v, want one generated.<fingerprint>.160.<ext>", names)
	}

	// Switching the library to auto must change the served bytes (unless the
	// auto pick for this seed happens to be bauhaus — it is "waves" here,
	// pinned by the seed hash, so the bytes must differ).
	if err := db.Model(&model.Library{}).Where("id = ?", lib.ID).Update("cover_style", "auto").Error; err != nil {
		t.Fatal(err)
	}
	auto := fetch()
	if bytes.Equal(styled, auto) {
		t.Fatal("bauhaus-styled and auto covers are byte-identical; style config had no effect")
	}
}

// TestCoverCacheKeyMatchesAssetKey verifies that the imagecache key for each
// entity kind derives from the same natural identity the asset store uses, not
// from the autoincrement database id. This prevents a DB rebuild from
// misattributing cached derivatives when ids reassign.
func TestCoverCacheKeyMatchesAssetKey(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	h := &Handler{store: s, assets: assetstore.New(t.TempDir())}

	// Album
	album := model.Album{
		Name:            "Test Album",
		NameNorm:        "test album",
		AlbumArtistNorm: "test artist",
		MBReleaseID:     "mb-release-123",
	}
	if err := db.Create(&album).Error; err != nil {
		t.Fatalf("create album: %v", err)
	}
	albumMeta := h.albumCoverMeta(&album)
	if got, want := albumMeta.cacheKey, assetkey.AlbumOf(&album); got != want {
		t.Errorf("album cacheKey = %q, want %q", got, want)
	}

	// Artist
	artist := model.Artist{
		Name:       "Test Artist",
		NameNorm:   "test artist",
		MBArtistID: "mb-artist-456",
	}
	if err := db.Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}
	artistMeta := h.artistCoverMeta(&artist)
	if got, want := artistMeta.cacheKey, assetkey.ArtistOf(&artist); got != want {
		t.Errorf("artist cacheKey = %q, want %q", got, want)
	}

	// Radio - drive through resolveCoverMeta to test the real implementation
	station := model.InternetRadioStation{
		Name:      "Test Station",
		StreamURL: "http://example.com/stream",
	}
	if err := db.Create(&station).Error; err != nil {
		t.Fatalf("create radio station: %v", err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	radioMeta, ok := h.resolveCoverMeta(w, r, "radio", station.ID)
	if !ok {
		t.Fatalf("resolveCoverMeta returned ok=false for radio")
	}
	if got, want := radioMeta.cacheKey, assetkey.Radio(station.StreamURL); got != want {
		t.Errorf("radio cacheKey = %q, want %q", got, want)
	}

	// Playlist - drive through resolveCoverMeta to test the real implementation
	pl := model.Playlist{
		Name:  "Test Playlist",
		Owner: "admin",
		UUID:  "test-uuid-789",
	}
	if err := db.Create(&pl).Error; err != nil {
		t.Fatalf("create playlist: %v", err)
	}
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/", nil)
	playlistMeta, ok := h.resolveCoverMeta(w, r, "playlist", pl.ID)
	if !ok {
		t.Fatalf("resolveCoverMeta returned ok=false for playlist")
	}
	if got, want := playlistMeta.cacheKey, assetkey.PlaylistOf(&pl); got != want {
		t.Errorf("playlist cacheKey = %q, want %q", got, want)
	}

	// Genre - drive through resolveCoverMeta to test the real implementation
	genre := model.Genre{
		Name: "Test Genre",
	}
	if err := db.Create(&genre).Error; err != nil {
		t.Fatalf("create genre: %v", err)
	}
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/", nil)
	genreMeta, ok := h.resolveCoverMeta(w, r, "genre", genre.ID)
	if !ok {
		t.Fatalf("resolveCoverMeta returned ok=false for genre")
	}
	if got, want := genreMeta.cacheKey, assetkey.GenreOf(&genre); got != want {
		t.Errorf("genre cacheKey = %q, want %q", got, want)
	}
}
