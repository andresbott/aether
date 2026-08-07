package subsonic

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/imagecache"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/gorilla/mux"

	_ "github.com/gen2brain/webp" // registers the webp decoder for image.Decode
)

// realPNG builds a decodable w×h PNG with varied content, standing in for a
// full-resolution cover scan.
func realPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x % 251), uint8(y % 241), uint8((x * y) % 239), 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// albumWithStoredCover seeds an album whose cover lives in the asset store,
// returning the album and the source image bytes.
func albumWithStoredCover(t *testing.T, size int) (*store.Store, *assetstore.Store, model.Album, []byte) {
	t.Helper()
	s := testStore(t)
	album := model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead"}
	if err := s.DB().Create(&album).Error; err != nil {
		t.Fatalf("create album: %v", err)
	}
	src := realPNG(t, size, size)
	as := assetstore.New(t.TempDir())
	if err := as.PutManual(assetstore.KindAlbum, strconv.FormatUint(uint64(album.ID), 10), "png", src); err != nil {
		t.Fatalf("PutManual: %v", err)
	}
	return s, as, album, src
}

// serveCovers starts a test server whose image cache lives in cacheDir.
func serveCovers(t *testing.T, s *store.Store, as *assetstore.Store, cacheDir string) *httptest.Server {
	t.Helper()
	r := mux.NewRouter()
	Register(r, s, as, imagecache.New(cacheDir), nil)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

// decodeServedCover reads a cover response and returns its decoded config and
// format, failing the test when the body is not an image.
func decodeServedCover(t *testing.T, resp *http.Response) (image.Config, string) {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read cover body: %v", err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("served cover is not a decodable image (%d bytes): %v", len(body), err)
	}
	return cfg, format
}

// cachedDerivativeNames lists the derivative filenames cached for one entity.
func cachedDerivativeNames(t *testing.T, cacheDir, kind, key string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(cacheDir, kind, key))
	if err != nil {
		t.Fatalf("read cache dir for %s/%s: %v", kind, key, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// getCover requests a cover, optionally with an Accept header.
func getCover(t *testing.T, url, accept string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// The whole point of the change: a request for an 80-pixel thumbnail must not
// ship the full-resolution cover.
func TestGetCoverArtServesSizedDerivativeNotTheOriginal(t *testing.T) {
	s, as, album, src := albumWithStoredCover(t, 1500)
	srv := serveCovers(t, s, as, t.TempDir())

	resp := getCover(t, fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d&size=80", srv.URL, album.ID), "image/webp,*/*")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if len(body) >= len(src) {
		t.Errorf("served %d bytes for an 80px thumbnail of a %d-byte source; want a much smaller derivative", len(body), len(src))
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("decode served image: %v", err)
	}
	if format != "webp" {
		t.Errorf("format = %q, want webp", format)
	}
	// 80 quantizes up to the 96 bucket: derivatives are built at a fixed set of
	// sizes so the cache stays bounded, and the browser scales the rest.
	if want := quantizeCoverSize(80); cfg.Width != want || cfg.Height != want {
		t.Errorf("served %dx%d, want %dx%d", cfg.Width, cfg.Height, want, want)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/webp" {
		t.Errorf("Content-Type = %q, want image/webp", ct)
	}
}

// A client that does not advertise WebP gets JPEG, not a broken image.
func TestGetCoverArtServesJPEGWhenWebPNotAccepted(t *testing.T) {
	s, as, album, _ := albumWithStoredCover(t, 1500)
	srv := serveCovers(t, s, as, t.TempDir())

	resp := getCover(t, fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d&size=200", srv.URL, album.ID), "*/*")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	_, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("decode served image: %v", err)
	}
	if format != "jpeg" {
		t.Errorf("format = %q, want jpeg for a client that did not accept webp", format)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", ct)
	}
}

// Serving two formats from one URL means caches must key on Accept.
func TestGetCoverArtVariesOnAccept(t *testing.T) {
	s, as, album, _ := albumWithStoredCover(t, 800)
	srv := serveCovers(t, s, as, t.TempDir())

	resp := getCover(t, fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d&size=200", srv.URL, album.ID), "image/webp")
	if got := resp.Header.Get("Vary"); got != "Accept" {
		t.Errorf("Vary = %q, want Accept", got)
	}
}

// A client that sends no size still must not be handed a multi-megabyte
// original: the response is capped.
func TestGetCoverArtCapsSizeWhenNoSizeRequested(t *testing.T) {
	s, as, album, src := albumWithStoredCover(t, 3000)
	srv := serveCovers(t, s, as, t.TempDir())

	resp := getCover(t, fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d", srv.URL, album.ID), "image/webp")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(body) >= len(src) {
		t.Errorf("served %d bytes with no size param (source is %d); want a capped derivative", len(body), len(src))
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("decode served image: %v", err)
	}
	if cfg.Width > maxCoverSize || cfg.Height > maxCoverSize {
		t.Errorf("served %dx%d, want both edges <= %d", cfg.Width, cfg.Height, maxCoverSize)
	}
}

// Requesting an absurd size must not make the server render a huge image on
// demand — it clamps to the cap.
func TestGetCoverArtClampsOversizedRequest(t *testing.T) {
	s, as, album, _ := albumWithStoredCover(t, 3000)
	srv := serveCovers(t, s, as, t.TempDir())

	resp := getCover(t, fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d&size=9000", srv.URL, album.ID), "image/webp")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("decode served image: %v", err)
	}
	if cfg.Width > maxCoverSize {
		t.Errorf("served width %d for size=9000, want <= %d", cfg.Width, maxCoverSize)
	}
}

// Derivatives land in the cache tree, keyed by entity, so they survive
// restarts instead of being rebuilt per request.
func TestGetCoverArtWritesDerivativeToCacheTree(t *testing.T) {
	s, as, album, _ := albumWithStoredCover(t, 800)
	cacheDir := t.TempDir()
	srv := serveCovers(t, s, as, cacheDir)

	getCover(t, fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d&size=200", srv.URL, album.ID), "image/webp")

	entries, err := os.ReadDir(filepath.Join(cacheDir, assetstore.KindAlbum, strconv.FormatUint(uint64(album.ID), 10)))
	if err != nil {
		t.Fatalf("read cache entry dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("cache holds %d entries, want 1 derivative", len(entries))
	}
	if got := entries[0].Name(); filepath.Ext(got) != ".webp" {
		t.Errorf("cached derivative %q, want a .webp file", got)
	}
}

// Embedded art was re-extracted from the audio file on every single request.
// Once cached, the derivative must be served without touching the file again.
func TestGetCoverArtCachesEmbeddedCoverAcrossRequests(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	front := realPNG(t, 1000, 1000)
	trackDir := t.TempDir()
	trackPath := embeddedFixture(t, trackDir, "01.flac", embeddedPic{"Front Cover", front})

	album := model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead", HasEmbeddedCover: true}
	if err := db.Create(&album).Error; err != nil {
		t.Fatalf("create album: %v", err)
	}
	track := model.Track{AlbumID: album.ID, Filename: "01.flac", FilePath: trackPath, HasEmbeddedCover: true}
	if err := db.Create(&track).Error; err != nil {
		t.Fatalf("create track: %v", err)
	}

	srv := serveCovers(t, s, assetstore.New(t.TempDir()), t.TempDir())
	url := fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d&size=200", srv.URL, album.ID)

	first := getCover(t, url, "image/webp")
	firstBody, err := io.ReadAll(first.Body)
	if err != nil {
		t.Fatalf("read first body: %v", err)
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(firstBody)); err != nil {
		t.Fatalf("first response is not a decodable image: %v", err)
	}

	// Making the audio file unreadable proves the second response came from the
	// cache: a stat (used to fingerprint the source) still succeeds, but any
	// attempt to re-extract the picture from the file would fail.
	if err := os.Chmod(trackPath, 0o000); err != nil {
		t.Fatalf("chmod track: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(trackPath, 0o600) })

	second := getCover(t, url, "image/webp")
	if second.StatusCode != http.StatusOK {
		t.Fatalf("second status = %d, want 200 (served from cache)", second.StatusCode)
	}
	secondBody, err := io.ReadAll(second.Body)
	if err != nil {
		t.Fatalf("read second body: %v", err)
	}
	if !bytes.Equal(firstBody, secondBody) {
		t.Error("second response differs from the first; the derivative was not reused")
	}
}

// Generated fallback covers go through the same cache and format negotiation as
// real ones — the separate generated-covers PNG tree is gone.
func TestGetCoverArtGeneratedFallbackIsCachedAsWebP(t *testing.T) {
	s := testStore(t)
	album := model.Album{Name: "19", NameNorm: "19", AlbumArtistNorm: "adele"}
	if err := s.DB().Create(&album).Error; err != nil {
		t.Fatalf("create album: %v", err)
	}
	cacheDir := t.TempDir()
	srv := serveCovers(t, s, assetstore.New(t.TempDir()), cacheDir)

	resp := getCover(t, fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d&size=256", srv.URL, album.ID), "image/webp")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("decode generated cover: %v", err)
	}
	if format != "webp" {
		t.Errorf("generated cover format = %q, want webp", format)
	}
	if cfg.Width != 256 {
		t.Errorf("generated cover width = %d, want 256", cfg.Width)
	}
}

// A real library can hold a truncated or corrupt cover file. Re-encoding it
// fails, and answering 500 would leave a broken image in every grid cell it
// appears in — the generated cover is the graceful answer.
func TestGetCoverArtFallsBackToGeneratedWhenSourceIsUndecodable(t *testing.T) {
	s := testStore(t)
	album := model.Album{Name: "Corrupt", NameNorm: "corrupt", AlbumArtistNorm: "someone"}
	if err := s.DB().Create(&album).Error; err != nil {
		t.Fatalf("create album: %v", err)
	}
	as := assetstore.New(t.TempDir())
	// Right magic bytes, no decodable image behind them.
	if err := as.PutManual(assetstore.KindAlbum, strconv.FormatUint(uint64(album.ID), 10), "png",
		[]byte("\x89PNG\r\n\x1a\nTRUNCATED")); err != nil {
		t.Fatalf("PutManual: %v", err)
	}
	srv := serveCovers(t, s, as, t.TempDir())

	resp := getCover(t, fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d&size=256", srv.URL, album.ID), "image/webp")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (generated fallback for a corrupt cover)", resp.StatusCode)
	}
	cfg, _ := decodeServedCover(t, resp)
	if cfg.Width != 256 {
		t.Errorf("served width = %d, want a 256 generated cover", cfg.Width)
	}
}

// A track flagged as carrying embedded art whose picture cannot be read (the
// file was re-tagged since the scan) must also degrade to the generated cover.
func TestGetCoverArtFallsBackToGeneratedWhenEmbeddedCoverUnreadable(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	// A fixture with a back cover only: flagged as having embedded art, but no
	// front cover to serve.
	trackPath := embeddedFixture(t, t.TempDir(), "01.flac", embeddedPic{"Back Cover", realPNG(t, 300, 300)})
	album := model.Album{Name: "Backonly", NameNorm: "backonly", AlbumArtistNorm: "someone", HasEmbeddedCover: true}
	if err := db.Create(&album).Error; err != nil {
		t.Fatalf("create album: %v", err)
	}
	track := model.Track{AlbumID: album.ID, Filename: "01.flac", FilePath: trackPath, HasEmbeddedCover: true}
	if err := db.Create(&track).Error; err != nil {
		t.Fatalf("create track: %v", err)
	}
	srv := serveCovers(t, s, assetstore.New(t.TempDir()), t.TempDir())

	resp := getCover(t, fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d&size=256", srv.URL, album.ID), "image/webp")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (generated fallback)", resp.StatusCode)
	}
	cfg, _ := decodeServedCover(t, resp)
	if cfg.Width != 256 {
		t.Errorf("served width = %d, want a 256 generated cover", cfg.Width)
	}
}

// A changed cover must not keep serving the old derivative: cover URLs are
// stable while the image behind them is not.
func TestGetCoverArtRebuildsDerivativeAfterCoverChanges(t *testing.T) {
	s, as, album, _ := albumWithStoredCover(t, 800)
	srv := serveCovers(t, s, as, t.TempDir())
	url := fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d&size=200", srv.URL, album.ID)

	before, err := io.ReadAll(getCover(t, url, "image/webp").Body)
	if err != nil {
		t.Fatalf("read first body: %v", err)
	}

	// A new upload replaces the stored cover with visibly different art.
	key := strconv.FormatUint(uint64(album.ID), 10)
	if err := as.PutManual(assetstore.KindAlbum, key, "png", realPNG(t, 400, 900)); err != nil {
		t.Fatalf("replace cover: %v", err)
	}

	after, err := io.ReadAll(getCover(t, url, "image/webp").Body)
	if err != nil {
		t.Fatalf("read second body: %v", err)
	}
	if bytes.Equal(before, after) {
		t.Fatal("served the same derivative after the cover changed")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(after))
	if err != nil {
		t.Fatalf("decode new derivative: %v", err)
	}
	// The replacement is 400x900 — portrait, so the box bounds its height.
	box := quantizeCoverSize(200)
	if cfg.Height != box || cfg.Width != 400*box/900 {
		t.Errorf("new derivative = %dx%d, want %dx%d (built from the replacement)",
			cfg.Width, cfg.Height, 400*box/900, box)
	}
}
