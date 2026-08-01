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

func postGenre(t *testing.T, srvURL string, body io.Reader, contentType string) (string, int) {
	t.Helper()
	resp, err := http.Post(srvURL+"/rest/updateGenre.view", contentType, body)
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

func TestGetGenresIncludesCoverArt(t *testing.T) {
	s := testStore(t)
	rock := model.Genre{Name: "Rock"}
	if err := s.DB().Create(&rock).Error; err != nil {
		t.Fatal(err)
	}
	srv := newTestServer(t, s)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rest/getGenres.view")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		SubsonicResponse struct {
			Genres struct {
				Genre []struct {
					Value    string `json:"value"`
					CoverArt string `json:"coverArt"`
				} `json:"genre"`
			} `json:"genres"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	genres := body.SubsonicResponse.Genres.Genre
	if len(genres) != 1 {
		t.Fatalf("expected 1 genre, got %d", len(genres))
	}
	if genres[0].CoverArt != encodeGenreID(rock.ID) {
		t.Fatalf("expected coverArt %q, got %q", encodeGenreID(rock.ID), genres[0].CoverArt)
	}
}

func TestUpdateGenreUploadsCover(t *testing.T) {
	s := testStore(t)
	genre := model.Genre{Name: "Jazz"}
	if err := s.DB().Create(&genre).Error; err != nil {
		t.Fatal(err)
	}
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	body, ct := buildMultipart(t, map[string]string{"id": encodeGenreID(genre.ID)}, pngBytes(t), "g.png")
	if status, code := postGenre(t, srv.URL, body, ct); status != "ok" {
		t.Fatalf("status=%s code=%d", status, code)
	}
	if _, ok := as.Get(assetstore.KindGenre, strconv.FormatUint(uint64(genre.ID), 10)); !ok {
		t.Fatal("expected cover under DB-ID key")
	}
	if !servesUploadedCover(t, srv.URL, encodeGenreID(genre.ID)) {
		t.Fatal("getCoverArt should serve the uploaded cover")
	}
}

func TestUpdateGenreCoverClear(t *testing.T) {
	s := testStore(t)
	genre := model.Genre{Name: "Blues"}
	if err := s.DB().Create(&genre).Error; err != nil {
		t.Fatal(err)
	}
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	key := strconv.FormatUint(uint64(genre.ID), 10)
	if err := as.PutManual(assetstore.KindGenre, key, "png", pngBytes(t)); err != nil {
		t.Fatal(err)
	}
	body, ct := buildMultipart(t, map[string]string{
		"id":         encodeGenreID(genre.ID),
		"coverClear": "true",
	}, nil, "")
	if status, code := postGenre(t, srv.URL, body, ct); status != "ok" {
		t.Fatalf("status=%s code=%d", status, code)
	}
	if _, ok := as.Get(assetstore.KindGenre, key); ok {
		t.Fatal("expected cover removed after coverClear")
	}
}

// A genre without an upload still serves a name-seeded generated cover.
func TestGetCoverArtGenreGeneratedFallback(t *testing.T) {
	s := testStore(t)
	genre := model.Genre{Name: "Ambient"}
	if err := s.DB().Create(&genre).Error; err != nil {
		t.Fatal(err)
	}
	srv := newTestServer(t, s)
	defer srv.Close()

	if servesUploadedCover(t, srv.URL, encodeGenreID(genre.ID)) {
		t.Fatal("getCoverArt served an uploaded cover; want the generated fallback")
	}
}

func TestUpdateGenreRejectsNonMultipart(t *testing.T) {
	s := testStore(t)
	srv, _ := newRadioServer(t, s)
	defer srv.Close()
	env := getJSON(t, srv.URL, "/rest/updateGenre.view?id=ge-1")
	if env.SubsonicResponse.Status != "failed" {
		t.Fatalf("expected failed for non-multipart, got %s", env.SubsonicResponse.Status)
	}
}

func TestUpdateGenreNotFound(t *testing.T) {
	s := testStore(t)
	srv, _ := newRadioServer(t, s)
	defer srv.Close()
	body, ct := buildMultipart(t, map[string]string{"id": encodeGenreID(999)}, pngBytes(t), "g.png")
	if _, code := postGenre(t, srv.URL, body, ct); code != 70 {
		t.Fatalf("expected code 70, got %d", code)
	}
}
