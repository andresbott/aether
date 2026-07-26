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
	"strconv"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
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
	Register(r, s, assetstore.New(t.TempDir()), cacheDir)
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
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Errorf("Content-Type = %q, want image/png*", ct)
	}
	if _, err := png.Decode(resp.Body); err != nil {
		t.Errorf("response body is not a valid PNG: %v", err)
	}

	// Cache file must exist for size bucket 256 (200 quantizes up to 256).
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("read cache dir: %v", err)
	}
	var found bool
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "_256.png") {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("no _256.png cached in %s; got %v", cacheDir, names)
	}
}

func TestGetCoverArtRadioUploadedServed(t *testing.T) {
	s := testStore(t)
	st, _ := s.CreateInternetRadioStation("R1", "http://r1", "")

	// Store a PNG into the asset store keyed by RadioKey(streamURL).
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	assetDir := t.TempDir()
	as := assetstore.New(assetDir)
	if err := as.PutManual(assetstore.KindRadio, RadioKey(st.StreamURL), "png", buf.Bytes()); err != nil {
		t.Fatal(err)
	}

	r := mux.NewRouter()
	Register(r, s, as, t.TempDir())
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
	if _, err := png.Decode(resp.Body); err != nil {
		t.Errorf("response body is not a valid PNG: %v", err)
	}
}

func TestGetCoverArtRadioFallbackGenerated(t *testing.T) {
	s := testStore(t)
	st, _ := s.CreateInternetRadioStation("Fallback FM", "http://f", "")

	cacheDir := t.TempDir()
	r := mux.NewRouter()
	Register(r, s, assetstore.New(t.TempDir()), cacheDir)
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
	if _, err := png.Decode(resp.Body); err != nil {
		t.Errorf("response body is not a valid PNG: %v", err)
	}
	// Verify a cache file landed in the generated-covers dir.
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "_256.png") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected generated-cover cache file in %s, got: %v", cacheDir, entries)
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
	Register(r, s, assetstore.New(t.TempDir()), cacheDir)
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
	if _, err := png.Decode(resp.Body); err != nil {
		t.Errorf("response body is not a valid PNG: %v", err)
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "_256.png") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected generated-cover cache file in %s, got: %v", cacheDir, entries)
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
	Register(r, s, assetstore.New(t.TempDir()), t.TempDir())
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

	// Put a PNG image into the asset store for this artist.
	assetDir := t.TempDir()
	as := assetstore.New(assetDir)
	if err := as.PutAuto(assetstore.KindArtist, "mbid-art", "png", []byte("\x89PNG\r\n\x1a\nFAKE")); err != nil {
		t.Fatalf("PutAuto: %v", err)
	}

	r := mux.NewRouter()
	Register(r, s, as, t.TempDir())
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(body), "\x89PNG") {
		t.Errorf("response body does not start with PNG magic bytes; got %q", body[:min(8, len(body))])
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
	if err := as.PutManual(assetstore.KindAlbum, strconv.FormatUint(uint64(album.ID), 10), "png", []byte("\x89PNG\r\n\x1a\nFAKE")); err != nil {
		t.Fatalf("PutManual: %v", err)
	}

	r := mux.NewRouter()
	Register(r, s, as, t.TempDir())
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(body), "\x89PNG") {
		t.Errorf("response body does not start with PNG magic bytes; got %q", body[:min(8, len(body))])
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

	front := []byte("\x89PNG\r\n\x1a\nFRONT")
	back := []byte("\x89PNG\r\n\x1a\nBACK")
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
	Register(r, s, assetstore.New(t.TempDir()), t.TempDir())
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d", srv.URL, album.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) == string(back) {
		t.Fatal("served the embedded BACK cover; want the front cover")
	}
	if string(body) != string(front) {
		t.Errorf("served %q, want the embedded front cover", body[:min(16, len(body))])
	}
}

// A track carrying only a back cover has no cover art: the album must fall
// through to the generated cover rather than serving the back scan.
func TestGetCoverArtAlbumBackCoverOnlyFallsBackToGenerated(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	back := []byte("\x89PNG\r\n\x1a\nBACK")
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
	Register(r, s, assetstore.New(t.TempDir()), t.TempDir())
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=al-%d&size=128", srv.URL, album.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) == string(back) {
		t.Fatal("served the embedded back cover; want the generated fallback cover")
	}
	if _, err := png.Decode(bytes.NewReader(body)); err != nil {
		t.Errorf("expected a decodable generated PNG, got %v", err)
	}
}

// An artist with no fetched or uploaded image falls back to the image found in
// the artist's folder on disk before the generated avatar.
func TestGetCoverArtArtistServesFolderImage(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	musicDir := t.TempDir()
	imgPath := filepath.Join(musicDir, "artist.jpg")
	if err := os.WriteFile(imgPath, []byte("\xff\xd8\xffFAKEJPEG"), 0o600); err != nil {
		t.Fatal(err)
	}

	artist := model.Artist{Name: "Pink Floyd", NameNorm: "pink floyd", ImagePath: imgPath}
	if err := db.Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}

	r := mux.NewRouter()
	Register(r, s, assetstore.New(t.TempDir()), t.TempDir())
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(body), "\xff\xd8\xff") {
		t.Errorf("response body is not the folder JPEG; got %q", body[:min(8, len(body))])
	}
}

// A stored (fetched or uploaded) image outranks the artist-folder image.
func TestGetCoverArtArtistPrefersStoredOverFolderImage(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	musicDir := t.TempDir()
	imgPath := filepath.Join(musicDir, "artist.jpg")
	if err := os.WriteFile(imgPath, []byte("\xff\xd8\xffFAKEJPEG"), 0o600); err != nil {
		t.Fatal(err)
	}

	artist := model.Artist{Name: "Pink Floyd", NameNorm: "pink floyd", MBArtistID: "mbid-pf", ImagePath: imgPath}
	if err := db.Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}

	as := assetstore.New(t.TempDir())
	if err := as.PutAuto(assetstore.KindArtist, "mbid-pf", "png", []byte("\x89PNG\r\n\x1a\nFAKE")); err != nil {
		t.Fatalf("PutAuto: %v", err)
	}

	r := mux.NewRouter()
	Register(r, s, as, t.TempDir())
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/getCoverArt.view?id=ar-%d", srv.URL, artist.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(body), "\x89PNG") {
		t.Errorf("stored image should win over the folder image; got %q", body[:min(8, len(body))])
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
	Register(r, s, assetstore.New(t.TempDir()), t.TempDir())
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
	if _, err := png.Decode(resp.Body); err != nil {
		t.Errorf("expected a generated PNG avatar: %v", err)
	}
}
