package subsonic

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetOpenSubsonicExtensions(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rest/getOpenSubsonicExtensions.view")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		SubsonicResponse struct {
			Status     string `json:"status"`
			Extensions []struct {
				Name     string `json:"name"`
				Versions []int  `json:"versions"`
			} `json:"openSubsonicExtensions"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.SubsonicResponse.Status != "ok" {
		t.Fatalf("expected status=ok, got %q", body.SubsonicResponse.Status)
	}
	exts := body.SubsonicResponse.Extensions
	if len(exts) != 1 {
		t.Fatalf("expected 1 extension, got %d: %+v", len(exts), exts)
	}
	if exts[0].Name != "musicFolderDefaultView" {
		t.Fatalf("expected name=musicFolderDefaultView, got %q", exts[0].Name)
	}
	if len(exts[0].Versions) != 1 || exts[0].Versions[0] != 1 {
		t.Fatalf("expected versions=[1], got %v", exts[0].Versions)
	}
}
