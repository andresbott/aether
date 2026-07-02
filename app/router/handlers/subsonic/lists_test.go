package subsonic

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/andresbott/aether/internal/model"
)

func TestGetAlbumList2IncludesSongCountAndDuration(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	a := model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead"}
	db.Create(&a)
	db.Create(&model.Track{AlbumID: a.ID, Filename: "1.mp3", FilePath: "/1.mp3", Duration: 100})
	db.Create(&model.Track{AlbumID: a.ID, Filename: "2.mp3", FilePath: "/2.mp3", Duration: 150})

	srv := newTestServer(t, s)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rest/getAlbumList2.view?type=alphabeticalByName")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		SubsonicResponse struct {
			AlbumList2 struct {
				Album []struct {
					SongCount int `json:"songCount"`
					Duration  int `json:"duration"`
				} `json:"album"`
			} `json:"albumList2"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	albums := body.SubsonicResponse.AlbumList2.Album
	if len(albums) != 1 {
		t.Fatalf("expected 1 album, got %d", len(albums))
	}
	if albums[0].SongCount != 2 || albums[0].Duration != 250 {
		t.Fatalf("got songCount=%d duration=%d, want 2/250", albums[0].SongCount, albums[0].Duration)
	}
}
