package subsonic

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/gorilla/mux"
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

	cacheDir := filepath.Join(t.TempDir(), "generated-covers")
	r := mux.NewRouter()
	Register(r, s, cacheDir, "")
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

	// Create a real PNG file on disk and point CoverPath at it.
	dir := t.TempDir()
	pngPath := filepath.Join(dir, fmt.Sprintf("%d.png", st.ID))
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	f, err := os.Create(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if err := s.UpdateInternetRadioStationCoverPath(st.ID, pngPath); err != nil {
		t.Fatal(err)
	}

	r := mux.NewRouter()
	Register(r, s, t.TempDir(), dir)
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
	Register(r, s, cacheDir, t.TempDir())
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
