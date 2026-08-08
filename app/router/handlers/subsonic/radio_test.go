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
	"testing"

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
// storage via h.assets.Get(assetstore.KindRadio, RadioKey(streamURL)).
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
	k1 := RadioKey("http://a/stream")
	k2 := RadioKey("http://a/stream")
	if k1 != k2 {
		t.Fatal("RadioKey not stable")
	}
	if RadioKey("http://a") == RadioKey("http://b") {
		t.Fatal("RadioKey should differ by URL")
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
	// Cover must be retrievable via the asset store keyed by RadioKey(streamURL).
	if _, ok := as.Get(assetstore.KindRadio, RadioKey(streamURL)); !ok {
		t.Fatalf("expected cover in asset store for RadioKey(%q)", streamURL)
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
	if _, ok := as.Get(assetstore.KindRadio, RadioKey(streamURL)); ok {
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
	if _, ok := as.Get(assetstore.KindRadio, RadioKey(streamURL)); !ok {
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
	if _, ok := as.Get(assetstore.KindRadio, RadioKey(streamURL)); !ok {
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
	if _, ok := as.Get(assetstore.KindRadio, RadioKey(streamURL)); ok {
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

	if _, ok := as.Get(assetstore.KindRadio, RadioKey(oldURL)); !ok {
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
	if _, ok := as.Get(assetstore.KindRadio, RadioKey(newURL)); !ok {
		t.Fatal("cover not found under new key after URL change")
	}
	// Cover must be gone under old key.
	if _, ok := as.Get(assetstore.KindRadio, RadioKey(oldURL)); ok {
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
	if _, ok := as.Get(assetstore.KindRadio, RadioKey(streamURL)); ok {
		t.Fatal("expected cover to be deleted from asset store after station delete")
	}
}
