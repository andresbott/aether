package metadata_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	metaHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/imagecache"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/mux"
	"gorm.io/gorm"

	_ "github.com/gen2brain/webp" // registers the webp decoder for image.Decode
)

// bigPNG builds a decodable w×h PNG standing in for a full-resolution scan.
func bigPNG(t *testing.T, w, h int) []byte {
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

// pictureImageServer wires a metadata handler with an image cache and seeds an
// album whose front cover lives in the managed store.
func pictureImageServer(t *testing.T, src []byte) (*httptest.Server, *model.Library, string) {
	t.Helper()
	libRoot := t.TempDir()
	trackAbs := filepath.Join(libRoot, "a.flac")
	if err := os.WriteFile(trackAbs, []byte("audio"), 0o600); err != nil {
		t.Fatalf("write track: %v", err)
	}

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
	key := seedAlbum(t, s, lib, trackAbs)

	as := assetstore.New(t.TempDir())
	if err := as.PutManualNamed(assetstore.KindAlbum, key, "cover", "png", src); err != nil {
		t.Fatalf("PutManualNamed: %v", err)
	}

	h := &metaHandler.Handler{
		Store:  s,
		Reader: nullReader{},
		Assets: as,
		Images: imagecache.New(t.TempDir()),
	}
	r := mux.NewRouter()
	h.Routes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, lib, key
}

// pictureImageURL builds a request for the album's db-slot front cover.
func pictureImageURL(srv *httptest.Server, lib *model.Library, size string) string {
	q := url.Values{
		"library_id": {libIDStr(lib)},
		"path":       {"."},
		"type":       {"Front Cover"},
		"slot":       {"db"},
	}
	if size != "" {
		q.Set("size", size)
	}
	return fmt.Sprintf("%s/metadata/pictures/image?%s", srv.URL, q.Encode())
}

func fetchPicture(t *testing.T, rawURL string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Accept", "image/webp,*/*")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, body
}

// The editor's picture grid renders small cells; asking for a size must return
// an optimized derivative rather than the full scan.
func TestPictureImageServesSizedDerivative(t *testing.T) {
	src := bigPNG(t, 1500, 1500)
	srv, lib, _ := pictureImageServer(t, src)

	resp, body := fetchPicture(t, pictureImageURL(srv, lib, "160"))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(body) >= len(src) {
		t.Errorf("served %d bytes for a 160px cell of a %d-byte source; want a smaller derivative", len(body), len(src))
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("decode served image: %v", err)
	}
	if format != "webp" {
		t.Errorf("format = %q, want webp", format)
	}
	if cfg.Width != 160 || cfg.Height != 160 {
		t.Errorf("served %dx%d, want 160x160", cfg.Width, cfg.Height)
	}
}

// Without a size the endpoint still serves the original: the editor copies an
// image from one slot to another through this URL, and a copy must carry the
// full-fidelity bytes, not a downscaled re-encode.
func TestPictureImageServesOriginalWithoutSize(t *testing.T) {
	src := bigPNG(t, 1500, 1500)
	srv, lib, _ := pictureImageServer(t, src)

	resp, body := fetchPicture(t, pictureImageURL(srv, lib, ""))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !bytes.Equal(body, src) {
		t.Errorf("served %d bytes, want the %d-byte original verbatim", len(body), len(src))
	}
}
