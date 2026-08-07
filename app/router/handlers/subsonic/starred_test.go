package subsonic

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

// starFixture builds one library with an artist, two albums and two tracks, then
// stars the first of each so every response can be checked for both the present
// and the omitted case.
type starFixture struct {
	store           *store.Store
	artist, artist2 model.Artist
	album, album2   model.Album
	track, track2   model.Track
	at              time.Time
}

func newStarFixture(t *testing.T) starFixture {
	t.Helper()
	s := testStore(t)
	db := s.DB()
	db.Create(&model.Library{Name: "Lib", Path: "/l"})

	f := starFixture{store: s, at: time.Date(2026, 7, 30, 8, 0, 0, 0, time.UTC)}
	f.artist = model.Artist{Name: "Starred Artist", NameNorm: "starred artist"}
	f.artist2 = model.Artist{Name: "Plain Artist", NameNorm: "plain artist"}
	db.Create(&f.artist)
	db.Create(&f.artist2)

	f.album = model.Album{Name: "Starred Album", NameNorm: "starred album", AlbumArtistNorm: "starred artist"}
	f.album2 = model.Album{Name: "Plain Album", NameNorm: "plain album", AlbumArtistNorm: "plain artist"}
	db.Create(&f.album)
	db.Create(&f.album2)
	_ = db.Model(&f.album).Association("Artists").Replace([]*model.Artist{&f.artist})
	_ = db.Model(&f.album2).Association("Artists").Replace([]*model.Artist{&f.artist2})

	f.track = model.Track{AlbumID: f.album.ID, LibraryID: 1, Filename: "1.mp3", FilePath: "/l/1.mp3", Title: "Starred Song", TrackNumber: 1}
	f.track2 = model.Track{AlbumID: f.album.ID, LibraryID: 1, Filename: "2.mp3", FilePath: "/l/2.mp3", Title: "Plain Song", TrackNumber: 2}
	db.Create(&f.track)
	db.Create(&f.track2)
	_ = db.Model(&f.track).Association("Artists").Replace([]*model.Artist{&f.artist})
	_ = db.Model(&f.track2).Association("Artists").Replace([]*model.Artist{&f.artist})

	db.Create(&model.StarredItem{Owner: "admin", ItemType: "artist", ItemID: f.artist.ID, CreatedAt: f.at})
	db.Create(&model.StarredItem{Owner: "admin", ItemType: "album", ItemID: f.album.ID, CreatedAt: f.at})
	db.Create(&model.StarredItem{Owner: "admin", ItemType: "track", ItemID: f.track.ID, CreatedAt: f.at})
	return f
}

// starredItem is the slice of any Subsonic entity this file asserts on.
type starredItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Title   string `json:"title"`
	Starred string `json:"starred"`
}

func (i starredItem) label() string {
	if i.Name != "" {
		return i.Name
	}
	return i.Title
}

func decodeJSON(t *testing.T, url string, out any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: expected 200, got %d", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatal(err)
	}
}

// wantStarred asserts the starred item carries the fixture timestamp as RFC3339
// and the unstarred one omits the field entirely.
func wantStarred(t *testing.T, items []starredItem, starredLabel, plainLabel string, at time.Time) {
	t.Helper()
	seen := map[string]starredItem{}
	for _, it := range items {
		seen[it.label()] = it
	}
	got, ok := seen[starredLabel]
	if !ok {
		t.Fatalf("%q missing from response (got %+v)", starredLabel, items)
	}
	if got.Starred != at.Format(time.RFC3339) {
		t.Errorf("%q starred = %q, want %q", starredLabel, got.Starred, at.Format(time.RFC3339))
	}
	if plainLabel == "" {
		return
	}
	plain, ok := seen[plainLabel]
	if !ok {
		t.Fatalf("%q missing from response (got %+v)", plainLabel, items)
	}
	if plain.Starred != "" {
		t.Errorf("%q must omit starred, got %q", plainLabel, plain.Starred)
	}
}

func TestGetAlbumEmitsStarred(t *testing.T) {
	f := newStarFixture(t)
	srv := newTestServer(t, f.store)
	defer srv.Close()

	var body struct {
		SubsonicResponse struct {
			Album struct {
				starredItem
				Song []starredItem `json:"song"`
			} `json:"album"`
		} `json:"subsonic-response"`
	}
	decodeJSON(t, srv.URL+"/rest/getAlbum.view?id="+encodeAlbumID(f.album.ID), &body)

	al := body.SubsonicResponse.Album
	wantStarred(t, []starredItem{al.starredItem}, "Starred Album", "", f.at)
	wantStarred(t, al.Song, "Starred Song", "Plain Song", f.at)
}

func TestGetAlbumOmitsStarredWhenUnstarred(t *testing.T) {
	f := newStarFixture(t)
	srv := newTestServer(t, f.store)
	defer srv.Close()

	var body struct {
		SubsonicResponse struct {
			Album starredItem `json:"album"`
		} `json:"subsonic-response"`
	}
	decodeJSON(t, srv.URL+"/rest/getAlbum.view?id="+encodeAlbumID(f.album2.ID), &body)

	if body.SubsonicResponse.Album.Starred != "" {
		t.Fatalf("unstarred album must omit starred, got %q", body.SubsonicResponse.Album.Starred)
	}
}

func TestGetArtistEmitsStarred(t *testing.T) {
	f := newStarFixture(t)
	srv := newTestServer(t, f.store)
	defer srv.Close()

	var body struct {
		SubsonicResponse struct {
			Artist struct {
				starredItem
				Album []starredItem `json:"album"`
			} `json:"artist"`
		} `json:"subsonic-response"`
	}
	decodeJSON(t, srv.URL+"/rest/getArtist.view?id="+encodeArtistID(f.artist.ID), &body)

	ar := body.SubsonicResponse.Artist
	wantStarred(t, []starredItem{ar.starredItem}, "Starred Artist", "", f.at)
	wantStarred(t, ar.Album, "Starred Album", "", f.at)
}

func TestGetArtistOmitsStarredWhenUnstarred(t *testing.T) {
	f := newStarFixture(t)
	srv := newTestServer(t, f.store)
	defer srv.Close()

	var body struct {
		SubsonicResponse struct {
			Artist starredItem `json:"artist"`
		} `json:"subsonic-response"`
	}
	decodeJSON(t, srv.URL+"/rest/getArtist.view?id="+encodeArtistID(f.artist2.ID), &body)

	if body.SubsonicResponse.Artist.Starred != "" {
		t.Fatalf("unstarred artist must omit starred, got %q", body.SubsonicResponse.Artist.Starred)
	}
}

func TestGetSongEmitsStarred(t *testing.T) {
	f := newStarFixture(t)
	srv := newTestServer(t, f.store)
	defer srv.Close()

	var body struct {
		SubsonicResponse struct {
			Song starredItem `json:"song"`
		} `json:"subsonic-response"`
	}
	decodeJSON(t, srv.URL+"/rest/getSong.view?id="+encodeTrackID(f.track.ID), &body)
	wantStarred(t, []starredItem{body.SubsonicResponse.Song}, "Starred Song", "", f.at)

	var plain struct {
		SubsonicResponse struct {
			Song starredItem `json:"song"`
		} `json:"subsonic-response"`
	}
	decodeJSON(t, srv.URL+"/rest/getSong.view?id="+encodeTrackID(f.track2.ID), &plain)
	if plain.SubsonicResponse.Song.Starred != "" {
		t.Fatalf("unstarred song must omit starred, got %q", plain.SubsonicResponse.Song.Starred)
	}
}

func TestGetArtistsEmitStarred(t *testing.T) {
	f := newStarFixture(t)
	srv := newTestServer(t, f.store)
	defer srv.Close()

	var body struct {
		SubsonicResponse struct {
			Artists struct {
				Index []struct {
					Artist []starredItem `json:"artist"`
				} `json:"index"`
			} `json:"artists"`
		} `json:"subsonic-response"`
	}
	decodeJSON(t, srv.URL+"/rest/getArtists.view", &body)

	var all []starredItem
	for _, idx := range body.SubsonicResponse.Artists.Index {
		all = append(all, idx.Artist...)
	}
	wantStarred(t, all, "Starred Artist", "Plain Artist", f.at)
}

func TestGetAlbumList2EmitsStarred(t *testing.T) {
	f := newStarFixture(t)
	srv := newTestServer(t, f.store)
	defer srv.Close()

	var body struct {
		SubsonicResponse struct {
			AlbumList2 struct {
				Album []starredItem `json:"album"`
			} `json:"albumList2"`
		} `json:"subsonic-response"`
	}
	decodeJSON(t, srv.URL+"/rest/getAlbumList2.view?type=alphabeticalByName&size=50", &body)
	wantStarred(t, body.SubsonicResponse.AlbumList2.Album, "Starred Album", "Plain Album", f.at)
}

func TestSearch3EmitsStarred(t *testing.T) {
	f := newStarFixture(t)
	srv := newTestServer(t, f.store)
	defer srv.Close()

	var body struct {
		SubsonicResponse struct {
			SearchResult3 struct {
				Artist []starredItem `json:"artist"`
				Album  []starredItem `json:"album"`
				Song   []starredItem `json:"song"`
			} `json:"searchResult3"`
		} `json:"subsonic-response"`
	}
	decodeJSON(t, srv.URL+"/rest/search3.view?query=&artistCount=20&albumCount=20&songCount=20", &body)

	res := body.SubsonicResponse.SearchResult3
	wantStarred(t, res.Artist, "Starred Artist", "Plain Artist", f.at)
	wantStarred(t, res.Album, "Starred Album", "Plain Album", f.at)
	wantStarred(t, res.Song, "Starred Song", "Plain Song", f.at)
}

func TestGetStarred2EmitsStarred(t *testing.T) {
	f := newStarFixture(t)
	srv := newTestServer(t, f.store)
	defer srv.Close()

	var body struct {
		SubsonicResponse struct {
			Starred2 struct {
				Artist []starredItem `json:"artist"`
				Album  []starredItem `json:"album"`
				Song   []starredItem `json:"song"`
			} `json:"starred2"`
		} `json:"subsonic-response"`
	}
	decodeJSON(t, srv.URL+"/rest/getStarred2.view", &body)

	res := body.SubsonicResponse.Starred2
	wantStarred(t, res.Artist, "Starred Artist", "", f.at)
	wantStarred(t, res.Album, "Starred Album", "", f.at)
	wantStarred(t, res.Song, "Starred Song", "", f.at)
}

// The library views render the favorites list with the SAME rows and cards as the
// full library, whose count columns and alphabet rail need these fields — so
// getStarred2 has to carry them, not just id/name/starred.
func TestGetStarred2EnrichesAlbumsAndArtists(t *testing.T) {
	f := newStarFixture(t)
	// Star the second album too, out of alphabetical order, to pin the ordering.
	db := f.store.DB()
	db.Create(&model.StarredItem{Owner: "admin", ItemType: "album", ItemID: f.album2.ID, CreatedAt: f.at})

	srv := newTestServer(t, f.store)
	defer srv.Close()

	var body struct {
		SubsonicResponse struct {
			Starred2 struct {
				Artist []struct {
					ID         string `json:"id"`
					Name       string `json:"name"`
					CoverArt   string `json:"coverArt"`
					AlbumCount int    `json:"albumCount"`
				} `json:"artist"`
				Album []struct {
					Name      string `json:"name"`
					Artist    string `json:"artist"`
					SongCount int    `json:"songCount"`
					Duration  int    `json:"duration"`
				} `json:"album"`
			} `json:"starred2"`
		} `json:"subsonic-response"`
	}
	decodeJSON(t, srv.URL+"/rest/getStarred2.view", &body)
	res := body.SubsonicResponse.Starred2

	if len(res.Artist) != 1 {
		t.Fatalf("expected 1 starred artist, got %+v", res.Artist)
	}
	artist := res.Artist[0]
	if artist.CoverArt != encodeArtistID(f.artist.ID) {
		t.Errorf("artist coverArt = %q, want %q", artist.CoverArt, encodeArtistID(f.artist.ID))
	}
	// Both fixture tracks live on album 1 and credit the starred artist.
	if artist.AlbumCount != 1 {
		t.Errorf("artist albumCount = %d, want 1", artist.AlbumCount)
	}

	if len(res.Album) != 2 {
		t.Fatalf("expected 2 starred albums, got %+v", res.Album)
	}
	// name_norm ASC, so "Plain Album" precedes "Starred Album" — the same order
	// getAlbumList2's alphabeticalByName uses, which the alphabet rail assumes.
	if res.Album[0].Name != "Plain Album" || res.Album[1].Name != "Starred Album" {
		t.Fatalf("albums not name-ordered: %+v", res.Album)
	}
	starredAlbum := res.Album[1]
	if starredAlbum.SongCount != 2 {
		t.Errorf("album songCount = %d, want 2", starredAlbum.SongCount)
	}
	if starredAlbum.Artist != "Starred Artist" {
		t.Errorf("album artist = %q, want %q", starredAlbum.Artist, "Starred Artist")
	}
}

// The favorites filter inside a library scopes by musicFolderId, so an entity with
// no track in that library must drop out.
func TestGetStarred2ScopesByLibrary(t *testing.T) {
	f := newStarFixture(t)
	db := f.store.DB()
	db.Create(&model.Library{Name: "Other", Path: "/o"})
	// album2 is starred but has no tracks at all, so no library claims it.
	db.Create(&model.StarredItem{Owner: "admin", ItemType: "album", ItemID: f.album2.ID, CreatedAt: f.at})

	srv := newTestServer(t, f.store)
	defer srv.Close()

	var body struct {
		SubsonicResponse struct {
			Starred2 struct {
				Artist []starredItem `json:"artist"`
				Album  []starredItem `json:"album"`
			} `json:"starred2"`
		} `json:"subsonic-response"`
	}
	decodeJSON(t, srv.URL+"/rest/getStarred2.view?musicFolderId=1", &body)
	res := body.SubsonicResponse.Starred2

	if len(res.Album) != 1 || res.Album[0].Name != "Starred Album" {
		t.Fatalf("library 1 albums = %+v, want only 'Starred Album'", res.Album)
	}
	if len(res.Artist) != 1 || res.Artist[0].Name != "Starred Artist" {
		t.Fatalf("library 1 artists = %+v, want only 'Starred Artist'", res.Artist)
	}

	// Library 2 holds no tracks, so nothing starred belongs to it.
	decodeJSON(t, srv.URL+"/rest/getStarred2.view?musicFolderId=2", &body)
	res = body.SubsonicResponse.Starred2
	if len(res.Album) != 0 || len(res.Artist) != 0 {
		t.Fatalf("library 2 should hold no favorites, got albums=%+v artists=%+v", res.Album, res.Artist)
	}
}

func TestGetRandomSongsEmitStarred(t *testing.T) {
	f := newStarFixture(t)
	srv := newTestServer(t, f.store)
	defer srv.Close()

	var body struct {
		SubsonicResponse struct {
			RandomSongs struct {
				Song []starredItem `json:"song"`
			} `json:"randomSongs"`
		} `json:"subsonic-response"`
	}
	decodeJSON(t, srv.URL+"/rest/getRandomSongs.view?size=50", &body)
	wantStarred(t, body.SubsonicResponse.RandomSongs.Song, "Starred Song", "Plain Song", f.at)
}

func TestGetPlaylistEntriesEmitStarred(t *testing.T) {
	f := newStarFixture(t)
	db := f.store.DB()
	pl, err := f.store.CreatePlaylist("Mix", "admin", true, []uint{f.track.ID, f.track2.ID})
	if err != nil {
		t.Fatal(err)
	}
	_ = db

	srv := newTestServer(t, f.store)
	defer srv.Close()

	var body struct {
		SubsonicResponse struct {
			Playlist struct {
				Entry []starredItem `json:"entry"`
			} `json:"playlist"`
		} `json:"subsonic-response"`
	}
	decodeJSON(t, srv.URL+"/rest/getPlaylist.view?id="+encodePlaylistID(pl.ID), &body)
	wantStarred(t, body.SubsonicResponse.Playlist.Entry, "Starred Song", "Plain Song", f.at)
}
