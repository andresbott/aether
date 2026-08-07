package subsonic

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/andresbott/aether/internal/model"
)

// The Subsonic spec only defines star/unstar for songs, albums and artists;
// playlists are Aether's "playlistStar" extension. Genres and radio stations are
// not starrable in any of them, so ids minted for those must be ignored rather
// than silently persisted as unreadable rows.
func TestStarIgnoresUnstarrableTypes(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	genre := model.Genre{Name: "Jazz"}
	db.Create(&genre)
	station := model.InternetRadioStation{Name: "R", StreamURL: "http://x/y"}
	db.Create(&station)

	srv := newTestServer(t, s)
	defer srv.Close()

	for _, id := range []string{encodeGenreID(genre.ID), encodeRadioID(station.ID)} {
		resp, err := http.Get(srv.URL + "/rest/star.view?id=" + id)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}

	var n int64
	db.Model(&model.StarredItem{}).Count(&n)
	if n != 0 {
		t.Fatalf("expected no starred_items rows for genre/radio ids, got %d", n)
	}
}

// albumId and artistId are typed parameters in the spec, so an id of another
// kind must be dropped rather than starring the row with that numeric id under
// the parameter's own type.
func TestStarRejectsMistypedTypedParams(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "Alb", NameNorm: "alb", AlbumArtistNorm: "a"}
	db.Create(&album)
	track := model.Track{AlbumID: album.ID, Filename: "1.mp3", FilePath: "/1.mp3"}
	db.Create(&track)

	srv := newTestServer(t, s)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rest/star.view?albumId=" + encodeTrackID(track.ID))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	var n int64
	db.Model(&model.StarredItem{}).Count(&n)
	if n != 0 {
		t.Fatalf("expected no starred_items rows for a track id passed as albumId, got %d", n)
	}
}

// Guards the types that ARE starrable, so the allowlist can't be tightened into
// uselessness.
func TestStarAcceptsStarrableTypes(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	artist := model.Artist{Name: "A", NameNorm: "a"}
	db.Create(&artist)
	album := model.Album{Name: "Alb", NameNorm: "alb", AlbumArtistNorm: "a"}
	db.Create(&album)
	track := model.Track{AlbumID: album.ID, Filename: "1.mp3", FilePath: "/1.mp3"}
	db.Create(&track)
	pl := model.Playlist{Name: "P"}
	db.Create(&pl)

	srv := newTestServer(t, s)
	defer srv.Close()

	ids := []string{
		encodeArtistID(artist.ID),
		encodeAlbumID(album.ID),
		encodeTrackID(track.ID),
		encodePlaylistID(pl.ID),
	}
	for _, id := range ids {
		resp, err := http.Get(srv.URL + "/rest/star.view?id=" + id)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}

	var n int64
	db.Model(&model.StarredItem{}).Count(&n)
	if n != int64(len(ids)) {
		t.Fatalf("expected %d starred_items rows, got %d", len(ids), n)
	}
}

func TestStarIsScopedToSessionUser(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "Alb", NameNorm: "alb", AlbumArtistNorm: "x"}
	db.Create(&album)
	srv := newTestServerWithIdentity(t, s)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/rest/star?albumId=al-"+strconv.FormatUint(uint64(album.ID), 10), nil)
	req.Header.Set("X-Test-User", "demo")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	demoAt, err := s.StarredAt("demo", "album", []uint{album.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := demoAt[album.ID]; !ok {
		t.Fatal("demo's star was not recorded under demo")
	}
	adminAt, err := s.StarredAt("admin", "album", []uint{album.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := adminAt[album.ID]; ok {
		t.Fatal("demo's star leaked to admin")
	}
}
