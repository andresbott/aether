package subsonic

import (
	"encoding/json"
	"net/http"
	"strconv"
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

// searchGenreResult decodes just the "searchGenres" extension's array, plus a
// raw view of the container so a test can assert the key is absent entirely.
func searchGenreResult(t *testing.T, srvURL, query string) ([]struct {
	Value      string `json:"value"`
	SongCount  int    `json:"songCount"`
	AlbumCount int    `json:"albumCount"`
	CoverArt   string `json:"coverArt"`
}, map[string]json.RawMessage) {
	t.Helper()
	resp, err := http.Get(srvURL + "/rest/search3.view?" + query)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		SubsonicResponse struct {
			SearchResult3 map[string]json.RawMessage `json:"searchResult3"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	container := body.SubsonicResponse.SearchResult3
	var genres []struct {
		Value      string `json:"value"`
		SongCount  int    `json:"songCount"`
		AlbumCount int    `json:"albumCount"`
		CoverArt   string `json:"coverArt"`
	}
	if raw, ok := container["genre"]; ok {
		if err := json.Unmarshal(raw, &genres); err != nil {
			t.Fatal(err)
		}
	}
	return genres, container
}

func TestSearch3ReturnsMatchingGenres(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	rock := model.Genre{Name: "Rock"}
	db.Create(&rock)
	db.Create(&model.Genre{Name: "Jazz"})
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "a"}
	db.Create(&album)
	_ = db.Model(&album).Association("Genres").Replace([]*model.Genre{&rock})
	track := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3"}
	db.Create(&track)
	_ = db.Model(&track).Association("Genres").Replace([]*model.Genre{&rock})

	srv := newTestServer(t, s)
	defer srv.Close()

	genres, _ := searchGenreResult(t, srv.URL, "query=roc&genreCount=10")
	if len(genres) != 1 {
		t.Fatalf("expected 1 genre, got %d: %+v", len(genres), genres)
	}
	if genres[0].Value != "Rock" {
		t.Fatalf("expected Rock, got %q", genres[0].Value)
	}
	if genres[0].SongCount != 1 || genres[0].AlbumCount != 1 {
		t.Fatalf("expected 1 song / 1 album, got %d / %d", genres[0].SongCount, genres[0].AlbumCount)
	}
	if genres[0].CoverArt != encodeGenreID(rock.ID) {
		t.Fatalf("expected coverArt %q, got %q", encodeGenreID(rock.ID), genres[0].CoverArt)
	}
}

// A client that does not know the extension must get the standard shape: no
// genre key at all, not an empty array.
func TestSearch3OmitsGenresWithoutGenreCount(t *testing.T) {
	s := testStore(t)
	s.DB().Create(&model.Genre{Name: "Rock"})

	srv := newTestServer(t, s)
	defer srv.Close()

	_, container := searchGenreResult(t, srv.URL, "query=roc")
	if _, ok := container["genre"]; ok {
		t.Fatal("expected no genre key when genreCount is absent")
	}
	for _, key := range []string{"artist", "album", "song"} {
		if _, ok := container[key]; !ok {
			t.Fatalf("expected the standard %q key to stay present", key)
		}
	}
}

// An explicit genreCount that matches nothing still emits the (empty) array, so
// a client that asked can distinguish "no matches" from "not supported".
func TestSearch3EmitsEmptyGenreArrayWhenAsked(t *testing.T) {
	s := testStore(t)
	s.DB().Create(&model.Genre{Name: "Rock"})

	srv := newTestServer(t, s)
	defer srv.Close()

	genres, container := searchGenreResult(t, srv.URL, "query=zzz&genreCount=10")
	if _, ok := container["genre"]; !ok {
		t.Fatal("expected the genre key when genreCount is set")
	}
	if len(genres) != 0 {
		t.Fatalf("expected no genres, got %+v", genres)
	}
}

func TestSearch3GenresRespectMusicFolder(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	rock := model.Genre{Name: "Rock"}
	prog := model.Genre{Name: "Prog Rock"}
	db.Create(&rock)
	db.Create(&prog)
	libA := model.Library{Name: "A", Path: "/a"}
	libB := model.Library{Name: "B", Path: "/b"}
	db.Create(&libA)
	db.Create(&libB)
	trackA := model.Track{Filename: "a.mp3", FilePath: "/a/a.mp3", LibraryID: libA.ID}
	trackB := model.Track{Filename: "b.mp3", FilePath: "/b/b.mp3", LibraryID: libB.ID}
	db.Create(&trackA)
	db.Create(&trackB)
	_ = db.Model(&trackA).Association("Genres").Replace([]*model.Genre{&rock})
	_ = db.Model(&trackB).Association("Genres").Replace([]*model.Genre{&prog})

	srv := newTestServer(t, s)
	defer srv.Close()

	genres, _ := searchGenreResult(t, srv.URL,
		"query=rock&genreCount=10&musicFolderId="+strconv.FormatUint(uint64(libA.ID), 10))
	if len(genres) != 1 || genres[0].Value != "Rock" {
		t.Fatalf("expected only library A's genre, got %+v", genres)
	}
}
