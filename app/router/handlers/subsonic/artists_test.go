package subsonic

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
)

// postStatus posts a multipart body and returns the subsonic status + error code.
func postArtist(t *testing.T, srvURL string, body io.Reader, contentType string) (string, int) {
	t.Helper()
	resp, err := http.Post(srvURL+"/rest/updateArtist.view", contentType, body)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var env struct {
		SubsonicResponse struct {
			Status string `json:"status"`
			Error  struct {
				Code int `json:"code"`
			} `json:"error"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	return env.SubsonicResponse.Status, env.SubsonicResponse.Error.Code
}

func servesPNG(t *testing.T, srvURL, id string) bool {
	t.Helper()
	resp, err := http.Get(srvURL + "/rest/getCoverArt.view?id=" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("getCoverArt status=%d", resp.StatusCode)
	}
	data, _ := io.ReadAll(resp.Body)
	return detectImageContentType(data) == "image/png"
}

func TestUpdateArtistMatchedUploadsUnderMBID(t *testing.T) {
	s := testStore(t)
	artist := model.Artist{Name: "Matched", NameNorm: "matched", MBArtistID: "mbid-up"}
	if err := s.DB().Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	body, ct := buildMultipart(t, map[string]string{"id": encodeArtistID(artist.ID)}, pngBytes(t), "a.png")
	if status, code := postArtist(t, srv.URL, body, ct); status != "ok" {
		t.Fatalf("status=%s code=%d", status, code)
	}
	if _, ok := as.Get(assetstore.KindArtist, "mbid-up"); !ok {
		t.Fatal("expected cover under MBID key")
	}
	if !servesPNG(t, srv.URL, encodeArtistID(artist.ID)) {
		t.Fatal("getCoverArt should serve the uploaded png")
	}
}

func TestUpdateArtistUnmatchedUploadsUnderDBID(t *testing.T) {
	s := testStore(t)
	artist := model.Artist{Name: "Unmatched", NameNorm: "unmatched"}
	if err := s.DB().Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	dbKey := strconv.FormatUint(uint64(artist.ID), 10)
	body, ct := buildMultipart(t, map[string]string{"id": encodeArtistID(artist.ID)}, pngBytes(t), "a.png")
	if status, code := postArtist(t, srv.URL, body, ct); status != "ok" {
		t.Fatalf("status=%s code=%d", status, code)
	}
	if _, ok := as.Get(assetstore.KindArtist, dbKey); !ok {
		t.Fatal("expected cover under DB-ID key")
	}
	if !servesPNG(t, srv.URL, encodeArtistID(artist.ID)) {
		t.Fatal("getCoverArt should serve the uploaded png")
	}
}

func TestUpdateArtistCoverClear(t *testing.T) {
	s := testStore(t)
	artist := model.Artist{Name: "Clear", NameNorm: "clear", MBArtistID: "mbid-clear"}
	if err := s.DB().Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	if err := as.PutManual(assetstore.KindArtist, "mbid-clear", "png", pngBytes(t)); err != nil {
		t.Fatal(err)
	}
	body, ct := buildMultipart(t, map[string]string{
		"id":         encodeArtistID(artist.ID),
		"coverClear": "true",
	}, nil, "")
	if status, code := postArtist(t, srv.URL, body, ct); status != "ok" {
		t.Fatalf("status=%s code=%d", status, code)
	}
	if _, ok := as.Get(assetstore.KindArtist, "mbid-clear"); ok {
		t.Fatal("expected cover removed after coverClear")
	}
}

func TestUpdateArtistRejectsNonMultipart(t *testing.T) {
	s := testStore(t)
	srv, _ := newRadioServer(t, s)
	defer srv.Close()
	env := getJSON(t, srv.URL, "/rest/updateArtist.view?id=ar-1")
	if env.SubsonicResponse.Status != "failed" {
		t.Fatalf("expected failed for non-multipart, got %s", env.SubsonicResponse.Status)
	}
}

func TestUpdateArtistNotFound(t *testing.T) {
	s := testStore(t)
	srv, _ := newRadioServer(t, s)
	defer srv.Close()
	body, ct := buildMultipart(t, map[string]string{"id": encodeArtistID(999)}, pngBytes(t), "a.png")
	if _, code := postArtist(t, srv.URL, body, ct); code != 70 {
		t.Fatalf("expected code 70, got %d", code)
	}
}
