package subsonic

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
)

func postAlbum(t *testing.T, srvURL string, body io.Reader, contentType string) (string, int) {
	t.Helper()
	resp, err := http.Post(srvURL+"/rest/updateAlbum.view", contentType, body)
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

func TestUpdateAlbumUploadsCover(t *testing.T) {
	s := testStore(t)
	album := model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead"}
	if err := s.DB().Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	body, ct := buildMultipart(t, map[string]string{"id": encodeAlbumID(album.ID)}, pngBytes(t), "a.png")
	if status, code := postAlbum(t, srv.URL, body, ct); status != "ok" {
		t.Fatalf("status=%s code=%d", status, code)
	}
	// Round trip: the uploaded cover must be served through getCoverArt.
	if !servesUploadedCover(t, srv.URL, encodeAlbumID(album.ID)) {
		t.Fatal("getCoverArt should serve the uploaded cover")
	}
	// The cover must be stored under the identity-derived key, not the DB id.
	if _, ok := as.Get(assetstore.KindAlbum, strconv.FormatUint(uint64(album.ID), 10)); ok {
		t.Fatal("cover still stored under the DB-ID key; expected identity-derived key")
	}
	if _, ok := as.Get(assetstore.KindAlbum, assetkey.AlbumOf(&album)); !ok {
		t.Fatal("expected cover under the identity-derived key")
	}
}

func TestUpdateAlbumCoverClear(t *testing.T) {
	s := testStore(t)
	album := model.Album{Name: "Amnesiac", NameNorm: "amnesiac", AlbumArtistNorm: "radiohead"}
	if err := s.DB().Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	key := assetkey.AlbumOf(&album)
	if err := as.PutManual(assetstore.KindAlbum, key, "png", pngBytes(t)); err != nil {
		t.Fatal(err)
	}
	body, ct := buildMultipart(t, map[string]string{
		"id":         encodeAlbumID(album.ID),
		"coverClear": "true",
	}, nil, "")
	if status, code := postAlbum(t, srv.URL, body, ct); status != "ok" {
		t.Fatalf("status=%s code=%d", status, code)
	}
	if _, ok := as.Get(assetstore.KindAlbum, key); ok {
		t.Fatal("expected cover removed after coverClear")
	}
}

func TestUpdateAlbumRejectsNonMultipart(t *testing.T) {
	s := testStore(t)
	srv, _ := newRadioServer(t, s)
	defer srv.Close()
	env := getJSON(t, srv.URL, "/rest/updateAlbum.view?id=al-1")
	if env.SubsonicResponse.Status != "failed" {
		t.Fatalf("expected failed for non-multipart, got %s", env.SubsonicResponse.Status)
	}
}

func TestUpdateAlbumNotFound(t *testing.T) {
	s := testStore(t)
	srv, _ := newRadioServer(t, s)
	defer srv.Close()
	body, ct := buildMultipart(t, map[string]string{"id": encodeAlbumID(999)}, pngBytes(t), "a.png")
	if _, code := postAlbum(t, srv.URL, body, ct); code != 70 {
		t.Fatalf("expected code 70, got %d", code)
	}
}

// An artist id must not be accepted by an album endpoint.
func TestUpdateAlbumRejectsForeignIDKind(t *testing.T) {
	s := testStore(t)
	srv, _ := newRadioServer(t, s)
	defer srv.Close()
	body, ct := buildMultipart(t, map[string]string{"id": "ar-1"}, pngBytes(t), "a.png")
	if status, _ := postAlbum(t, srv.URL, body, ct); status != "failed" {
		t.Fatalf("expected failed for an artist id, got %s", status)
	}
}

// updateAlbum must require admin privileges (error 50 for non-admins).
func TestUpdateAlbumRequiresAdmin(t *testing.T) {
	s := testStore(t)
	album := model.Album{Name: "OK Computer", NameNorm: "ok computer", AlbumArtistNorm: "radiohead"}
	if err := s.DB().Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	srv := newRadioServerWithRoles(t, s, "root")
	defer srv.Close()

	body, ct := buildMultipart(t, map[string]string{"id": encodeAlbumID(album.ID)}, pngBytes(t), "a.png")
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/rest/updateAlbum.view", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-Test-User", "bob")
	resp, err := http.DefaultClient.Do(req)
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
	if env.SubsonicResponse.Status != "failed" || env.SubsonicResponse.Error.Code != 50 {
		t.Fatalf("expected error 50 for non-admin, got status=%s code=%d", env.SubsonicResponse.Status, env.SubsonicResponse.Error.Code)
	}
}
