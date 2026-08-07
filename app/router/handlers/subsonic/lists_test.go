package subsonic

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

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

func TestGetAlbumList2Index(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	db.Create(&model.Album{Name: "Abba", NameNorm: "abba", AlbumArtistNorm: "x"})
	db.Create(&model.Album{Name: "Beta", NameNorm: "beta", AlbumArtistNorm: "x"})
	db.Create(&model.Album{Name: "Zed", NameNorm: "zed", AlbumArtistNorm: "x"})

	srv := newTestServer(t, s)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rest/getAlbumList2Index.view")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		SubsonicResponse struct {
			AlbumList2Index struct {
				Total int `json:"total"`
				Index []struct {
					Name   string `json:"name"`
					Offset int    `json:"offset"`
					Count  int    `json:"count"`
				} `json:"index"`
			} `json:"albumList2Index"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	idx := body.SubsonicResponse.AlbumList2Index
	if idx.Total != 3 {
		t.Fatalf("expected total 3, got %d", idx.Total)
	}
	if len(idx.Index) != 3 {
		t.Fatalf("expected 3 letter buckets, got %d", len(idx.Index))
	}
	if idx.Index[0].Name != "A" || idx.Index[0].Offset != 0 || idx.Index[0].Count != 1 {
		t.Fatalf("first bucket = %+v, want {A 0 1}", idx.Index[0])
	}
}

func TestGetStarred2IncludesPlaylists(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	a := model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead"}
	db.Create(&a)
	tr := model.Track{AlbumID: a.ID, Filename: "1.mp3", FilePath: "/1.mp3", Duration: 100}
	db.Create(&tr)
	pl, err := s.CreatePlaylist("Starred Mix", "admin", true, []uint{tr.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Star("admin", "playlist", pl.ID); err != nil {
		t.Fatal(err)
	}

	srv := newTestServer(t, s)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rest/getStarred2.view")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	var body struct {
		SubsonicResponse struct {
			Starred2 struct {
				Playlist []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"playlist"`
			} `json:"starred2"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	got := body.SubsonicResponse.Starred2.Playlist
	if len(got) != 1 || got[0].Name != "Starred Mix" {
		t.Fatalf("expected 1 starred playlist named 'Starred Mix', got %+v", got)
	}
	if got[0].ID != fmt.Sprintf("pl-%d", pl.ID) {
		t.Fatalf("id = %q, want pl-%d", got[0].ID, pl.ID)
	}
}

func TestGetNowPlayingReportsRealUsernames(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "Alb", NameNorm: "alb", AlbumArtistNorm: "x"}
	db.Create(&album)
	tr := model.Track{AlbumID: album.ID, Filename: "a.mp3", FilePath: "/a.mp3", Title: "A"}
	db.Create(&tr)
	if err := s.RecordPlay("demo", tr.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	srv := newTestServerWithIdentity(t, s)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/rest/getNowPlaying", nil)
	req.Header.Set("X-Test-User", "admin")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		SubsonicResponse struct {
			NowPlaying struct {
				Entry []struct {
					Username string `json:"username"`
				} `json:"entry"`
			} `json:"nowPlaying"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.SubsonicResponse.NowPlaying.Entry) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(body.SubsonicResponse.NowPlaying.Entry))
	}
	if got := body.SubsonicResponse.NowPlaying.Entry[0].Username; got != "demo" {
		t.Fatalf("expected username demo, got %q", got)
	}
}
