package subsonic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/imagecache"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/gorilla/mux"
)

type radioEnvelope struct {
	SubsonicResponse struct {
		Status                string `json:"status"`
		InternetRadioStations struct {
			InternetRadioStation []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				StreamURL   string `json:"streamUrl"`
				HomepageURL string `json:"homepageUrl"`
			} `json:"internetRadioStation"`
		} `json:"internetRadioStations"`
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	} `json:"subsonic-response"`
}

func decodeRadio(t *testing.T, resp *http.Response) radioEnvelope {
	t.Helper()
	var body radioEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

// newRadioServer creates a test server wired to an in-memory asset store and
// returns both the server and the asset store so tests can verify cover
// storage via h.assets.Get(assetstore.KindRadio, assetkey.Radio(streamURL)).
func newRadioServer(t *testing.T, s *store.Store) (*httptest.Server, *assetstore.Store) {
	t.Helper()
	as := assetstore.New(t.TempDir())
	r := mux.NewRouter()
	Register(r, s, as, imagecache.New(t.TempDir()), nil)
	return httptest.NewServer(r), as
}

func buildMultipart(t *testing.T, fields map[string]string, coverFile []byte, coverName string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if coverFile != nil {
		fw, err := mw.CreateFormFile("coverFile", coverName)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(coverFile); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// newRadioServerWithRoles registers /rest with the header identity resolver
// from newTestServerWithIdentity plus an AdminChecker that recognizes exactly
// one admin login, so tests can exercise the radio write gate per role.
func newRadioServerWithRoles(t *testing.T, s *store.Store, adminLogin string) *httptest.Server {
	t.Helper()
	as := assetstore.New(t.TempDir())
	r := mux.NewRouter()
	Register(r, s, as, imagecache.New(t.TempDir()),
		func(r *http.Request) (string, int) {
			u := r.Header.Get("X-Test-User")
			if u == "" {
				return "", 40
			}
			return u, 0
		},
		WithAdminChecker(func(owner string) (bool, error) {
			return owner == adminLogin, nil
		}),
	)
	return httptest.NewServer(r)
}

func radioGetAs(t *testing.T, srv *httptest.Server, user, pathAndQuery string) radioEnvelope {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+pathAndQuery, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Test-User", user)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	return decodeRadio(t, resp)
}

// The spec restricts createInternetRadioStation, updateInternetRadioStation
// and deleteInternetRadioStation to admin users; a non-admin must get error
// 50 and no write may happen. getInternetRadioStations stays open to all.
func TestRadioWritesRequireAdmin(t *testing.T) {
	s := testStore(t)
	st, _ := s.CreateInternetRadioStation("Keep", "http://keep", "")
	srv := newRadioServerWithRoles(t, s, "root")
	defer srv.Close()

	writes := map[string]string{
		"create": "/rest/createInternetRadioStation.view?name=X&streamUrl=http://x",
		"update": fmt.Sprintf("/rest/updateInternetRadioStation.view?id=rs-%d&name=Y&streamUrl=http://y", st.ID),
		"delete": fmt.Sprintf("/rest/deleteInternetRadioStation.view?id=rs-%d", st.ID),
	}
	for name, path := range writes {
		body := radioGetAs(t, srv, "bob", path)
		if body.SubsonicResponse.Status != "failed" || body.SubsonicResponse.Error.Code != 50 {
			t.Errorf("%s as non-admin: expected error 50, got %+v", name, body.SubsonicResponse)
		}
	}

	// Nothing was written: still one station, untouched.
	var loaded model.InternetRadioStation
	s.DB().First(&loaded, st.ID)
	if loaded.Name != "Keep" || loaded.StreamURL != "http://keep" {
		t.Fatalf("station modified by non-admin: %+v", loaded)
	}
	var count int64
	s.DB().Model(&model.InternetRadioStation{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 station, got %d", count)
	}

	// Reads stay open to non-admins.
	body := radioGetAs(t, srv, "bob", "/rest/getInternetRadioStations.view")
	if body.SubsonicResponse.Status != "ok" {
		t.Fatalf("read as non-admin: expected ok, got %+v", body.SubsonicResponse)
	}
}

func TestRadioWritesAllowAdmin(t *testing.T) {
	s := testStore(t)
	srv := newRadioServerWithRoles(t, s, "root")
	defer srv.Close()

	body := radioGetAs(t, srv, "root", "/rest/createInternetRadioStation.view?name=X&streamUrl=http://x")
	if body.SubsonicResponse.Status != "ok" {
		t.Fatalf("create as admin: expected ok, got %+v", body.SubsonicResponse)
	}
	var count int64
	s.DB().Model(&model.InternetRadioStation{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 station persisted, got %d", count)
	}
}

func TestRadioKeyStable(t *testing.T) {
	k1 := assetkey.Radio("http://a/stream")
	k2 := assetkey.Radio("http://a/stream")
	if k1 != k2 {
		t.Fatal("assetkey.Radio not stable")
	}
	if assetkey.Radio("http://a") == assetkey.Radio("http://b") {
		t.Fatal("assetkey.Radio should differ by URL")
	}
}

func TestGetInternetRadioStations(t *testing.T) {
	s := testStore(t)
	_, _ = s.CreateInternetRadioStation("BBC R1", "http://example.com/r1", "http://bbc.co.uk")
	_, _ = s.CreateInternetRadioStation("Nova", "http://example.com/nova", "")

	srv := newTestServer(t, s)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rest/getInternetRadioStations.view")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	body := decodeRadio(t, resp)
	if body.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s", body.SubsonicResponse.Status)
	}
	stations := body.SubsonicResponse.InternetRadioStations.InternetRadioStation
	if len(stations) != 2 {
		t.Fatalf("expected 2, got %d", len(stations))
	}
	// Sorted by name ASC.
	if stations[0].Name != "BBC R1" || stations[1].Name != "Nova" {
		t.Fatalf("unexpected order: %+v", stations)
	}
	if stations[0].ID == "" || stations[0].ID[:3] != "rs-" {
		t.Fatalf("expected id prefix rs-, got %q", stations[0].ID)
	}
	if stations[0].HomepageURL != "http://bbc.co.uk" {
		t.Errorf("expected homepage set, got %q", stations[0].HomepageURL)
	}
	if stations[1].HomepageURL != "" {
		t.Errorf("expected empty homepage when not set, got %q", stations[1].HomepageURL)
	}
}

func TestCreateInternetRadioStationHandler(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()

	q := url.Values{}
	q.Set("name", "BBC R1")
	q.Set("streamUrl", "http://example.com/r1")
	q.Set("homepageUrl", "http://bbc.co.uk")
	resp, err := http.Get(srv.URL + "/rest/createInternetRadioStation.view?" + q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body := decodeRadio(t, resp)
	if body.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", body.SubsonicResponse.Status, body.SubsonicResponse.Error)
	}
	// Verify persisted.
	var count int64
	s.DB().Model(&model.InternetRadioStation{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 station persisted, got %d", count)
	}
}

func TestCreateInternetRadioStationMissingName(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/rest/createInternetRadioStation.view?streamUrl=http://x")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body := decodeRadio(t, resp)
	if body.SubsonicResponse.Status != "failed" || body.SubsonicResponse.Error.Code != 10 {
		t.Fatalf("expected failed + code 10, got %+v", body.SubsonicResponse)
	}
}

func TestCreateInternetRadioStationMissingStreamURL(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/rest/createInternetRadioStation.view?name=X")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body := decodeRadio(t, resp)
	if body.SubsonicResponse.Error.Code != 10 {
		t.Fatalf("expected code 10, got %+v", body.SubsonicResponse.Error)
	}
}

func TestUpdateInternetRadioStationHandler(t *testing.T) {
	s := testStore(t)
	st, _ := s.CreateInternetRadioStation("Old", "http://old", "")
	srv := newTestServer(t, s)
	defer srv.Close()

	q := url.Values{}
	q.Set("id", fmt.Sprintf("rs-%d", st.ID))
	q.Set("name", "New")
	q.Set("streamUrl", "http://new")
	q.Set("homepageUrl", "http://home")
	resp, err := http.Get(srv.URL + "/rest/updateInternetRadioStation.view?" + q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body := decodeRadio(t, resp)
	if body.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", body.SubsonicResponse.Status, body.SubsonicResponse.Error)
	}
	var loaded model.InternetRadioStation
	s.DB().First(&loaded, st.ID)
	if loaded.Name != "New" || loaded.StreamURL != "http://new" || loaded.HomepageURL != "http://home" {
		t.Fatalf("not updated: %+v", loaded)
	}
}

func TestUpdateInternetRadioStationMissingID(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/rest/updateInternetRadioStation.view?name=X&streamUrl=http://x")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body := decodeRadio(t, resp)
	if body.SubsonicResponse.Error.Code != 10 {
		t.Fatalf("expected code 10, got %+v", body.SubsonicResponse.Error)
	}
}

func TestUpdateInternetRadioStationNotFound(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()
	q := url.Values{}
	q.Set("id", "rs-9999")
	q.Set("name", "X")
	q.Set("streamUrl", "http://x")
	resp, err := http.Get(srv.URL + "/rest/updateInternetRadioStation.view?" + q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body := decodeRadio(t, resp)
	if body.SubsonicResponse.Error.Code != 70 {
		t.Fatalf("expected code 70, got %+v", body.SubsonicResponse.Error)
	}
}

func TestUpdateInternetRadioStationBadIDPrefix(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()
	q := url.Values{}
	q.Set("id", "al-1")
	q.Set("name", "X")
	q.Set("streamUrl", "http://x")
	resp, err := http.Get(srv.URL + "/rest/updateInternetRadioStation.view?" + q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body := decodeRadio(t, resp)
	if body.SubsonicResponse.Status != "failed" {
		t.Fatalf("expected failed, got %+v", body.SubsonicResponse)
	}
}

func TestDeleteInternetRadioStationHandler(t *testing.T) {
	s := testStore(t)
	st, _ := s.CreateInternetRadioStation("X", "http://x", "")
	srv := newTestServer(t, s)
	defer srv.Close()

	resp, err := http.Get(srv.URL + fmt.Sprintf("/rest/deleteInternetRadioStation.view?id=rs-%d", st.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body := decodeRadio(t, resp)
	if body.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", body.SubsonicResponse.Status, body.SubsonicResponse.Error)
	}
	var count int64
	s.DB().Model(&model.InternetRadioStation{}).Count(&count)
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}
}

func TestDeleteInternetRadioStationMissingID(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/rest/deleteInternetRadioStation.view")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body := decodeRadio(t, resp)
	if body.SubsonicResponse.Error.Code != 10 {
		t.Fatalf("expected code 10, got %+v", body.SubsonicResponse.Error)
	}
}

func TestDeleteInternetRadioStationNotFound(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/rest/deleteInternetRadioStation.view?id=rs-9999")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body := decodeRadio(t, resp)
	if body.SubsonicResponse.Error.Code != 70 {
		t.Fatalf("expected code 70, got %+v", body.SubsonicResponse.Error)
	}
}

func TestGetInternetRadioStationsIncludesCoverArt(t *testing.T) {
	s := testStore(t)
	st, _ := s.CreateInternetRadioStation("BBC R1", "http://r1", "")
	srv := newTestServer(t, s)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/rest/getInternetRadioStations.view")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		SubsonicResponse struct {
			InternetRadioStations struct {
				InternetRadioStation []struct {
					ID       string `json:"id"`
					CoverArt string `json:"coverArt"`
				} `json:"internetRadioStation"`
			} `json:"internetRadioStations"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	stations := body.SubsonicResponse.InternetRadioStations.InternetRadioStation
	if len(stations) != 1 || stations[0].CoverArt != fmt.Sprintf("rs-%d", st.ID) {
		t.Fatalf("expected coverArt=rs-%d on single entry, got %+v", st.ID, stations)
	}
}

func TestCreateInternetRadioStationMultipartWithCover(t *testing.T) {
	s := testStore(t)
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	const streamURL = "http://r1"
	body, contentType := buildMultipart(t, map[string]string{
		"name":      "R1",
		"streamUrl": streamURL,
	}, pngBytes(t), "c.png")
	resp, err := http.Post(srv.URL+"/rest/createInternetRadioStation.view", contentType, body)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	env := decodeRadio(t, resp)
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	// Cover must be retrievable via the asset store keyed by assetkey.Radio(streamURL).
	if _, ok := as.Get(assetstore.KindRadio, assetkey.Radio(streamURL)); !ok {
		t.Fatalf("expected cover in asset store for assetkey.Radio(%q)", streamURL)
	}
}

func TestCreateInternetRadioStationMultipartWithoutCover(t *testing.T) {
	s := testStore(t)
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	const streamURL = "http://r1"
	body, contentType := buildMultipart(t, map[string]string{
		"name":      "R1",
		"streamUrl": streamURL,
	}, nil, "")
	resp, err := http.Post(srv.URL+"/rest/createInternetRadioStation.view", contentType, body)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	env := decodeRadio(t, resp)
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	// No cover should be stored.
	if _, ok := as.Get(assetstore.KindRadio, assetkey.Radio(streamURL)); ok {
		t.Fatal("expected no cover in asset store when none uploaded")
	}
}

func TestCreateInternetRadioStationMultipartOversize(t *testing.T) {
	s := testStore(t)
	srv, _ := newRadioServer(t, s)
	defer srv.Close()

	big := make([]byte, 6*1024*1024)
	body, contentType := buildMultipart(t, map[string]string{
		"name":      "R1",
		"streamUrl": "http://r1",
	}, big, "c.png")
	resp, err := http.Post(srv.URL+"/rest/createInternetRadioStation.view", contentType, body)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	env := decodeRadio(t, resp)
	if env.SubsonicResponse.Status != "failed" {
		t.Fatalf("expected failed, got %+v", env.SubsonicResponse)
	}
}

func TestCreateInternetRadioStationMultipartBadType(t *testing.T) {
	s := testStore(t)
	srv, _ := newRadioServer(t, s)
	defer srv.Close()
	body, contentType := buildMultipart(t, map[string]string{
		"name":      "R1",
		"streamUrl": "http://r1",
	}, []byte("not an image"), "c.txt")
	resp, err := http.Post(srv.URL+"/rest/createInternetRadioStation.view", contentType, body)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	env := decodeRadio(t, resp)
	if env.SubsonicResponse.Status != "failed" {
		t.Fatalf("expected failed, got %+v", env.SubsonicResponse)
	}
}

func TestUpdateInternetRadioStationMultipartReplaceCover(t *testing.T) {
	s := testStore(t)
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	const streamURL = "http://r1"

	// Seed: create with a cover.
	body, contentType := buildMultipart(t, map[string]string{
		"name":      "R1",
		"streamUrl": streamURL,
	}, pngBytes(t), "c.png")
	resp, _ := http.Post(srv.URL+"/rest/createInternetRadioStation.view", contentType, body)
	_ = resp.Body.Close()
	var st model.InternetRadioStation
	s.DB().First(&st)

	// Verify cover exists after create.
	if _, ok := as.Get(assetstore.KindRadio, assetkey.Radio(streamURL)); !ok {
		t.Fatal("seed cover missing from asset store")
	}

	// Update with a new PNG (same stream URL).
	body2, ct2 := buildMultipart(t, map[string]string{
		"id":        fmt.Sprintf("rs-%d", st.ID),
		"name":      "R1",
		"streamUrl": streamURL,
	}, pngBytes(t), "c.png")
	resp2, err := http.Post(srv.URL+"/rest/updateInternetRadioStation.view", ct2, body2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()
	env := decodeRadio(t, resp2)
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	// Cover must still be retrievable after update.
	if _, ok := as.Get(assetstore.KindRadio, assetkey.Radio(streamURL)); !ok {
		t.Fatal("expected cover in asset store after update")
	}
}

func TestUpdateInternetRadioStationMultipartCoverClear(t *testing.T) {
	s := testStore(t)
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	const streamURL = "http://r1"
	body, contentType := buildMultipart(t, map[string]string{
		"name":      "R1",
		"streamUrl": streamURL,
	}, pngBytes(t), "c.png")
	resp, _ := http.Post(srv.URL+"/rest/createInternetRadioStation.view", contentType, body)
	_ = resp.Body.Close()
	var st model.InternetRadioStation
	s.DB().First(&st)

	body2, ct2 := buildMultipart(t, map[string]string{
		"id":         fmt.Sprintf("rs-%d", st.ID),
		"name":       "R1",
		"streamUrl":  streamURL,
		"coverClear": "true",
	}, nil, "")
	resp2, err := http.Post(srv.URL+"/rest/updateInternetRadioStation.view", ct2, body2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()
	env := decodeRadio(t, resp2)
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	// Cover must be gone after clear.
	if _, ok := as.Get(assetstore.KindRadio, assetkey.Radio(streamURL)); ok {
		t.Fatal("expected cover to be cleared from asset store")
	}
}

func TestUpdateInternetRadioStationRekeysCoverOnURLChange(t *testing.T) {
	s := testStore(t)
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	const oldURL = "http://old-stream"
	const newURL = "http://new-stream"

	// Create station with a cover.
	body, contentType := buildMultipart(t, map[string]string{
		"name":      "ReKey FM",
		"streamUrl": oldURL,
	}, pngBytes(t), "c.png")
	resp, err := http.Post(srv.URL+"/rest/createInternetRadioStation.view", contentType, body)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	var st model.InternetRadioStation
	s.DB().First(&st)

	if _, ok := as.Get(assetstore.KindRadio, assetkey.Radio(oldURL)); !ok {
		t.Fatal("seed cover missing from asset store under old key")
	}

	// Update with new URL and NO new cover — cover must be re-keyed.
	body2, ct2 := buildMultipart(t, map[string]string{
		"id":        fmt.Sprintf("rs-%d", st.ID),
		"name":      "ReKey FM",
		"streamUrl": newURL,
	}, nil, "")
	resp2, err := http.Post(srv.URL+"/rest/updateInternetRadioStation.view", ct2, body2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()
	env := decodeRadio(t, resp2)
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("update failed: %+v", env.SubsonicResponse)
	}

	// Cover must be retrievable under new key.
	if _, ok := as.Get(assetstore.KindRadio, assetkey.Radio(newURL)); !ok {
		t.Fatal("cover not found under new key after URL change")
	}
	// Cover must be gone under old key.
	if _, ok := as.Get(assetstore.KindRadio, assetkey.Radio(oldURL)); ok {
		t.Fatal("cover still present under old key after URL change")
	}
}

func TestDeleteInternetRadioStationRemovesCover(t *testing.T) {
	s := testStore(t)
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	const streamURL = "http://r1"
	body, contentType := buildMultipart(t, map[string]string{
		"name":      "R1",
		"streamUrl": streamURL,
	}, pngBytes(t), "c.png")
	resp, _ := http.Post(srv.URL+"/rest/createInternetRadioStation.view", contentType, body)
	_ = resp.Body.Close()
	var st model.InternetRadioStation
	s.DB().First(&st)

	resp2, err := http.Get(srv.URL + fmt.Sprintf("/rest/deleteInternetRadioStation.view?id=rs-%d", st.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()
	env := decodeRadio(t, resp2)
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	// Cover must be removed from asset store after delete.
	if _, ok := as.Get(assetstore.KindRadio, assetkey.Radio(streamURL)); ok {
		t.Fatal("expected cover to be deleted from asset store after station delete")
	}
}

// TestUpdateInternetRadioStationURLChangePreservesNamedAndAutoVariants verifies
// that a stream-URL edit carries named entries and auto variants across the
// re-key. The bespoke re-key code reads only the primary manual image and
// re-PutManuals it, dropping every other entry — this test fails against that
// implementation and proves Rekey was adopted.
func TestUpdateInternetRadioStationURLChangePreservesNamedAndAutoVariants(t *testing.T) {
	s := testStore(t)
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	const oldURL = "http://old"
	const newURL = "http://new"

	// Create station with a primary cover.
	body, contentType := buildMultipart(t, map[string]string{
		"name":      "Test FM",
		"streamUrl": oldURL,
	}, pngBytes(t), "c.png")
	resp, _ := http.Post(srv.URL+"/rest/createInternetRadioStation.view", contentType, body)
	_ = resp.Body.Close()

	var st model.InternetRadioStation
	s.DB().First(&st)

	// Seed a named entry ("back") and an auto primary variant alongside the
	// manual primary from create. The bespoke re-key code only copies the primary
	// manual image, so these will be lost; Rekey moves the whole directory.
	oldKey := assetkey.Radio(oldURL)
	if err := as.PutManualNamed(assetstore.KindRadio, oldKey, "back", "png", pngBytes(t)); err != nil {
		t.Fatal(err)
	}
	if err := as.PutAuto(assetstore.KindRadio, oldKey, "png", pngBytes(t)); err != nil {
		t.Fatal(err)
	}

	// Update the stream URL without uploading a new cover.
	body2, ct2 := buildMultipart(t, map[string]string{
		"id":        fmt.Sprintf("rs-%d", st.ID),
		"name":      "Test FM",
		"streamUrl": newURL,
	}, nil, "")
	resp2, _ := http.Post(srv.URL+"/rest/updateInternetRadioStation.view", ct2, body2)
	_ = resp2.Body.Close()

	newKey := assetkey.Radio(newURL)
	// All entries must exist under the new key: the manual primary, the named
	// entry "back", and the auto variant of the primary.
	if _, ok := as.Get(assetstore.KindRadio, newKey); !ok {
		t.Fatal("primary cover not found under new key")
	}
	if _, ok := as.GetNamed(assetstore.KindRadio, newKey, "back"); !ok {
		t.Fatal("named entry 'back' not found under new key — bespoke re-key only moved the primary")
	}
	// To verify the auto variant exists, check the directory contents. GetEntry
	// returns the path of the best (manual-preferring) entry, which we can use to
	// find the directory.
	primaryPath, _, ok := as.GetEntry(assetstore.KindRadio, newKey)
	if !ok {
		t.Fatal("no primary entry under new key")
	}
	dir := filepath.Dir(primaryPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read asset directory: %v", err)
	}
	var hasAuto bool
	for _, e := range entries {
		if strings.Contains(e.Name(), ".auto.") {
			hasAuto = true
			break
		}
	}
	if !hasAuto {
		t.Fatal("auto variant not found under new key — bespoke re-key lost it")
	}
	// Old key must be gone.
	if _, ok := as.Get(assetstore.KindRadio, oldKey); ok {
		t.Fatal("old key still holds an image after re-key")
	}
}

// TestUpdateInternetRadioStationURLChangeOccupiedKeyLogged verifies that when
// the destination key already holds an image, Rekey returns ErrKeyOccupied but
// the handler logs it and still answers success — the station's own update
// already succeeded, and both images stay intact.
func TestUpdateInternetRadioStationURLChangeOccupiedKeyLogged(t *testing.T) {
	s := testStore(t)
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	const url1 = "http://station1"
	const url2 = "http://station2"

	// Create two stations with different URLs.
	body1, ct1 := buildMultipart(t, map[string]string{
		"name":      "Station 1",
		"streamUrl": url1,
	}, pngBytes(t), "c.png")
	resp1, _ := http.Post(srv.URL+"/rest/createInternetRadioStation.view", ct1, body1)
	_ = resp1.Body.Close()

	body2, ct2 := buildMultipart(t, map[string]string{
		"name":      "Station 2",
		"streamUrl": url2,
	}, pngBytes(t), "c.png")
	resp2, _ := http.Post(srv.URL+"/rest/createInternetRadioStation.view", ct2, body2)
	_ = resp2.Body.Close()

	var st1 model.InternetRadioStation
	s.DB().Where("stream_url = ?", url1).First(&st1)

	// Update station 1's URL to match station 2's URL (collision).
	body3, ct3 := buildMultipart(t, map[string]string{
		"id":        fmt.Sprintf("rs-%d", st1.ID),
		"name":      "Station 1",
		"streamUrl": url2,
	}, nil, "")
	resp3, _ := http.Post(srv.URL+"/rest/updateInternetRadioStation.view", ct3, body3)
	defer func() { _ = resp3.Body.Close() }()
	env := decodeRadio(t, resp3)

	// The request must succeed despite the collision.
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("expected ok despite key collision, got %+v", env.SubsonicResponse)
	}
	// Both keys must still hold their images.
	if _, ok := as.Get(assetstore.KindRadio, assetkey.Radio(url1)); !ok {
		t.Fatal("station 1's original image was removed")
	}
	if _, ok := as.Get(assetstore.KindRadio, assetkey.Radio(url2)); !ok {
		t.Fatal("station 2's image was removed")
	}
}

// TestPlaylistCoverRoundTripStoredUnderUUIDKey verifies that a playlist cover
// is stored under the UUID-derived key, not the numeric ID.
func TestPlaylistCoverRoundTripStoredUnderUUIDKey(t *testing.T) {
	s := testStore(t)
	// Create a playlist with a UUID.
	pl, _ := s.CreatePlaylist("Test", "admin", false, nil)
	if pl.UUID == "" {
		t.Fatal("playlist must have a non-empty UUID for this test")
	}
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	// Upload a cover.
	body, contentType := buildMultipart(t, map[string]string{
		"playlistId": encodePlaylistID(pl.ID),
	}, pngBytes(t), "c.png")
	resp, _ := http.Post(srv.URL+"/rest/updatePlaylist.view", contentType, body)
	_ = resp.Body.Close()

	// Must NOT be stored under the numeric ID.
	idKey := strconv.FormatUint(uint64(pl.ID), 10)
	if _, ok := as.Get(assetstore.KindPlaylist, idKey); ok {
		t.Fatal("cover was stored under numeric ID, not UUID-derived key")
	}
	// Must be stored under the UUID-derived key.
	uuidKey := assetkey.PlaylistOf(pl)
	if _, ok := as.Get(assetstore.KindPlaylist, uuidKey); !ok {
		t.Fatal("cover not found under UUID-derived key")
	}
	// getCoverArt must serve the uploaded cover.
	if !servesUploadedCover(t, srv.URL, encodePlaylistID(pl.ID)) {
		t.Fatal("getCoverArt should serve the uploaded cover")
	}
}
