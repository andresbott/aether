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
	names := map[string][]int{}
	for _, e := range exts {
		names[e.Name] = e.Versions
	}
	expected := []string{
		"musicFolderDefaultView",
		"musicFolderShowArtists",
		"musicFolderIcon",
		"albumList2Index",
		"internetRadioCoverArt",
		"playlistCoverArt",
		"artistCoverArt",
		"genreCoverArt",
		"playlistStar",
		"playlistScrobble",
		"playlistStats",
		"discovery",
		"indexBasedQueue",
	}
	if len(exts) != len(expected) {
		t.Fatalf("expected %d extensions, got %d: %+v", len(expected), len(exts), exts)
	}
	for _, name := range expected {
		if v, ok := names[name]; !ok || len(v) != 1 || v[0] != 1 {
			t.Fatalf("%s versions = %v", name, v)
		}
	}
}
