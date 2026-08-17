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
	// The key getCoverArt reads (albumCoverMeta in media.go) is the album DB ID.
	if _, ok := as.Get(assetstore.KindAlbum, strconv.FormatUint(uint64(album.ID), 10)); !ok {
		t.Fatal("expected cover under the album DB-ID key")
	}
	if !servesUploadedCover(t, srv.URL, encodeAlbumID(album.ID)) {
		t.Fatal("getCoverArt should serve the uploaded cover")
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

	key := strconv.FormatUint(uint64(album.ID), 10)
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
