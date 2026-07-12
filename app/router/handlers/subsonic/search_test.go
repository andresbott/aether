package subsonic

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/andresbott/aether/internal/model"
)

func TestSearch3ArtistIncludesCoverArt(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	db.Create(&model.Artist{Name: "Radiohead", NameNorm: "radiohead"})

	srv := newTestServer(t, s)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rest/search3.view?query=radio")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		SubsonicResponse struct {
			SearchResult3 struct {
				Artist []struct {
					ID       string `json:"id"`
					Name     string `json:"name"`
					CoverArt string `json:"coverArt"`
				} `json:"artist"`
			} `json:"searchResult3"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	artists := body.SubsonicResponse.SearchResult3.Artist
	if len(artists) != 1 {
		t.Fatalf("expected 1 artist, got %d", len(artists))
	}
	if artists[0].CoverArt == "" {
		t.Fatalf("expected non-empty coverArt, got empty for artist %+v", artists[0])
	}
	if artists[0].CoverArt != artists[0].ID {
		t.Fatalf("expected coverArt to equal artist id (%q), got %q", artists[0].ID, artists[0].CoverArt)
	}
}
