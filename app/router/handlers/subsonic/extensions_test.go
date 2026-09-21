package subsonic

import (
	"encoding/json"
	"net/http"
	"slices"
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
	expected := map[string][]int{
		"musicFolderDefaultView": {1},
		"musicFolderShowArtists": {1},
		"musicFolderIcon":        {1},
		"albumList2Index":        {1},
		"internetRadioCoverArt":  {1},
		"playlistCoverArt":       {1},
		"artistCoverArt":         {1},
		"genreCoverArt":          {1},
		"albumCoverArt":          {1},
		"playlistStar":           {1},
		"playlistScrobble":       {1},
		"playlistStats":          {1},
		"discovery":              {1},
		"indexBasedQueue":        {1},
		"searchGenres":           {1},
		"apiKeyAuthentication":   {1},
		// v2 adds getReleaseTypes; v1's releaseType parameter is unchanged.
		"releaseTypeFilter": {1, 2},
		"generatedCovers":   {1},
	}
	if len(exts) != len(expected) {
		t.Fatalf("expected %d extensions, got %d: %+v", len(expected), len(exts), exts)
	}
	for name, want := range expected {
		if v, ok := names[name]; !ok || !slices.Equal(v, want) {
			t.Fatalf("%s versions = %v, want %v", name, v, want)
		}
	}
}
