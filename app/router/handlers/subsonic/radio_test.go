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
	"testing"

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

func newRadioServer(t *testing.T, s *store.Store) (*httptest.Server, string) {
	t.Helper()
	radioDir := t.TempDir()
	r := mux.NewRouter()
	Register(r, s, t.TempDir(), radioDir)
	return httptest.NewServer(r), radioDir
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

func TestGetInternetRadioStations(t *testing.T) {
	s := testStore(t)
	s.CreateInternetRadioStation("BBC R1", "http://example.com/r1", "http://bbc.co.uk")
	s.CreateInternetRadioStation("Nova", "http://example.com/nova", "")

	srv := newTestServer(t, s)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rest/getInternetRadioStations.view")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
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
	defer resp.Body.Close()
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
	defer resp.Body.Close()
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
	defer resp.Body.Close()
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
	defer resp.Body.Close()
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
	defer resp.Body.Close()
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
	defer resp.Body.Close()
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
	defer resp.Body.Close()
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
	defer resp.Body.Close()
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
	defer resp.Body.Close()
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
	defer resp.Body.Close()
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
	defer resp.Body.Close()
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
	srv, coverDir := newRadioServer(t, s)
	defer srv.Close()

	body, contentType := buildMultipart(t, map[string]string{
		"name":      "R1",
		"streamUrl": "http://r1",
	}, pngBytes(t), "c.png")
	resp, err := http.Post(srv.URL+"/rest/createInternetRadioStation.view", contentType, body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	env := decodeRadio(t, resp)
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	var st model.InternetRadioStation
	s.DB().First(&st)
	expected := filepath.Join(coverDir, fmt.Sprintf("%d.png", st.ID))
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected cover file at %s: %v", expected, err)
	}
	if st.CoverPath != expected {
		t.Fatalf("CoverPath = %q, want %q", st.CoverPath, expected)
	}
}

func TestCreateInternetRadioStationMultipartWithoutCover(t *testing.T) {
	s := testStore(t)
	srv, coverDir := newRadioServer(t, s)
	defer srv.Close()

	body, contentType := buildMultipart(t, map[string]string{
		"name":      "R1",
		"streamUrl": "http://r1",
	}, nil, "")
	resp, err := http.Post(srv.URL+"/rest/createInternetRadioStation.view", contentType, body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	env := decodeRadio(t, resp)
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	entries, _ := os.ReadDir(coverDir)
	if len(entries) != 0 {
		t.Fatalf("expected no cover files, got %d", len(entries))
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
	defer resp.Body.Close()
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
	defer resp.Body.Close()
	env := decodeRadio(t, resp)
	if env.SubsonicResponse.Status != "failed" {
		t.Fatalf("expected failed, got %+v", env.SubsonicResponse)
	}
}

func TestUpdateInternetRadioStationMultipartReplaceCover(t *testing.T) {
	s := testStore(t)
	srv, coverDir := newRadioServer(t, s)
	defer srv.Close()

	// Seed: create with a cover.
	body, contentType := buildMultipart(t, map[string]string{
		"name":      "R1",
		"streamUrl": "http://r1",
	}, pngBytes(t), "c.png")
	resp, _ := http.Post(srv.URL+"/rest/createInternetRadioStation.view", contentType, body)
	resp.Body.Close()
	var st model.InternetRadioStation
	s.DB().First(&st)
	original := st.CoverPath
	if _, err := os.Stat(original); err != nil {
		t.Fatalf("seed cover missing: %v", err)
	}

	// Update with a new PNG.
	body2, ct2 := buildMultipart(t, map[string]string{
		"id":        fmt.Sprintf("rs-%d", st.ID),
		"name":      "R1",
		"streamUrl": "http://r1",
	}, pngBytes(t), "c.png")
	resp2, err := http.Post(srv.URL+"/rest/updateInternetRadioStation.view", ct2, body2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	env := decodeRadio(t, resp2)
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	s.DB().First(&st, st.ID)
	if st.CoverPath == "" {
		t.Fatal("expected CoverPath to still be set")
	}
	// Same extension → single file overwrite (via temp+rename).
	entries, _ := os.ReadDir(coverDir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 cover file (same ext overwrite), got %d", len(entries))
	}
}

func TestUpdateInternetRadioStationMultipartCoverClear(t *testing.T) {
	s := testStore(t)
	srv, coverDir := newRadioServer(t, s)
	defer srv.Close()

	body, contentType := buildMultipart(t, map[string]string{
		"name":      "R1",
		"streamUrl": "http://r1",
	}, pngBytes(t), "c.png")
	resp, _ := http.Post(srv.URL+"/rest/createInternetRadioStation.view", contentType, body)
	resp.Body.Close()
	var st model.InternetRadioStation
	s.DB().First(&st)

	body2, ct2 := buildMultipart(t, map[string]string{
		"id":         fmt.Sprintf("rs-%d", st.ID),
		"name":       "R1",
		"streamUrl":  "http://r1",
		"coverClear": "true",
	}, nil, "")
	resp2, err := http.Post(srv.URL+"/rest/updateInternetRadioStation.view", ct2, body2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	env := decodeRadio(t, resp2)
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	s.DB().First(&st, st.ID)
	if st.CoverPath != "" {
		t.Fatalf("expected CoverPath cleared, got %q", st.CoverPath)
	}
	entries, _ := os.ReadDir(coverDir)
	if len(entries) != 0 {
		t.Fatalf("expected 0 cover files, got %d", len(entries))
	}
}

func TestDeleteInternetRadioStationRemovesCover(t *testing.T) {
	s := testStore(t)
	srv, coverDir := newRadioServer(t, s)
	defer srv.Close()

	body, contentType := buildMultipart(t, map[string]string{
		"name":      "R1",
		"streamUrl": "http://r1",
	}, pngBytes(t), "c.png")
	resp, _ := http.Post(srv.URL+"/rest/createInternetRadioStation.view", contentType, body)
	resp.Body.Close()
	var st model.InternetRadioStation
	s.DB().First(&st)

	resp2, err := http.Get(srv.URL + fmt.Sprintf("/rest/deleteInternetRadioStation.view?id=rs-%d", st.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	env := decodeRadio(t, resp2)
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	entries, _ := os.ReadDir(coverDir)
	if len(entries) != 0 {
		t.Fatalf("expected 0 cover files after delete, got %d", len(entries))
	}
}
