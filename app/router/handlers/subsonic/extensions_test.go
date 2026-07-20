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
	if len(exts) != 6 {
		t.Fatalf("expected 6 extensions, got %d: %+v", len(exts), exts)
	}
	if v, ok := names["musicFolderDefaultView"]; !ok || len(v) != 1 || v[0] != 1 {
		t.Fatalf("musicFolderDefaultView versions = %v", v)
	}
	if v, ok := names["musicFolderShowArtists"]; !ok || len(v) != 1 || v[0] != 1 {
		t.Fatalf("musicFolderShowArtists versions = %v", v)
	}
	if v, ok := names["albumList2Index"]; !ok || len(v) != 1 || v[0] != 1 {
		t.Fatalf("albumList2Index versions = %v", v)
	}
	if v, ok := names["internetRadioCoverArt"]; !ok || len(v) != 1 || v[0] != 1 {
		t.Fatalf("internetRadioCoverArt versions = %v", v)
	}
	if v, ok := names["playlistCoverArt"]; !ok || len(v) != 1 || v[0] != 1 {
		t.Fatalf("playlistCoverArt versions = %v", v)
	}
	if v, ok := names["artistCoverArt"]; !ok || len(v) != 1 || v[0] != 1 {
		t.Fatalf("artistCoverArt versions = %v", v)
	}
}
