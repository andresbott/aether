package subsonic

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"testing"

	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/imagecache"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return store.New(db)
}

// scanFolderFilter builds the filters of a library that selects tracks from
// the named scan folders — the shape most test libraries use to stand in for
// "the tracks stamped with this scan folder".
func scanFolderFilter(names ...string) []model.LibraryFilter {
	return []model.LibraryFilter{{Field: model.FilterScanFolder, Values: names}}
}

func newTestServer(t *testing.T, s *store.Store) *httptest.Server {
	t.Helper()
	as := assetstore.New(t.TempDir())
	r := mux.NewRouter()
	Register(r, s, as, imagecache.New(t.TempDir()), nil)
	return httptest.NewServer(r)
}

// newTestServerWithIdentity registers /rest with a header-based identity
// resolver: X-Test-User names the owner, an empty header means "no
// credentials" (Subsonic error 40). This stands in for the apiKey resolver
// production wires up.
func newTestServerWithIdentity(t *testing.T, s *store.Store) *httptest.Server {
	t.Helper()
	as := assetstore.New(t.TempDir())
	r := mux.NewRouter()
	Register(r, s, as, imagecache.New(t.TempDir()), func(r *http.Request) (string, int) {
		u := r.Header.Get("X-Test-User")
		if u == "" {
			return "", 40
		}
		return u, 0
	})
	return httptest.NewServer(r)
}

// getSong/getAlbum/getArtist each take a typed id. decodeID accepts every prefix
// but these handlers discarded it, so getSong?id=al-N used to return the TRACK
// sharing N's number. A wrong-kind id must fail, not resolve to the entity that
// happens to share the number.
func TestBrowsingRejectsWrongKindID(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	artist := model.Artist{Name: "A", NameNorm: "a"}
	db.Create(&artist)
	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "a"}
	db.Create(&album)
	track := model.Track{AlbumID: album.ID, Filename: "a.mp3", FilePath: "/a.mp3"}
	db.Create(&track)

	srv := newTestServer(t, s)
	defer srv.Close()

	cases := []struct {
		name string
		path string
	}{
		{"getSong with a non-track id", "/rest/getSong.view?id=" + encodeAlbumID(track.ID)},
		{"getAlbum with a non-album id", "/rest/getAlbum.view?id=" + encodeTrackID(album.ID)},
		{"getArtist with a non-artist id", "/rest/getArtist.view?id=" + encodeAlbumID(artist.ID)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := getJSON(t, srv.URL, tc.path)
			if env.SubsonicResponse.Status != "failed" {
				t.Fatalf("status = %q, want failed", env.SubsonicResponse.Status)
			}
		})
	}
}

// musicFoldersBody decodes getMusicFolders with the musicFolderViews and
// musicFolderIcon extension fields.
type musicFoldersBody struct {
	SubsonicResponse struct {
		MusicFolders struct {
			Catalog struct {
				Views       []string `json:"views"`
				DefaultView string   `json:"defaultView"`
				SplitViews  bool     `json:"splitViews"`
			} `json:"catalog"`
			MusicFolder []struct {
				ID          uint     `json:"id"`
				Name        string   `json:"name"`
				Views       []string `json:"views"`
				DefaultView string   `json:"defaultView"`
				SplitViews  bool     `json:"splitViews"`
				Icon        string   `json:"icon"`
			} `json:"musicFolder"`
		} `json:"musicFolders"`
	} `json:"subsonic-response"`
}

func TestGetMusicFoldersFromDB(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	db.Create(&model.Library{
		Name: "Zulu", Views: []model.LibraryView{model.ViewArtists, model.ViewAlbums},
		DefaultView: model.ViewArtists, HideFromArtistIndex: true, SplitViews: true, Icon: "heart",
	})
	db.Create(&model.Library{Name: "Alpha"}) // the column defaults

	srv := newTestServer(t, s)
	defer srv.Close()

	var body musicFoldersBody
	decodeJSON(t, srv.URL+"/rest/getMusicFolders.view", &body)
	folders := body.SubsonicResponse.MusicFolders.MusicFolder
	if len(folders) != 2 {
		t.Fatalf("expected 2 folders, got %d", len(folders))
	}
	// Ordered by name ascending (ListLibraries order).
	if folders[0].Name != "Alpha" || folders[1].Name != "Zulu" {
		t.Fatalf("unexpected order: %+v", folders)
	}
	if want := []string{"discover", "artists", "albums"}; !slices.Equal(folders[0].Views, want) || folders[0].DefaultView != "discover" {
		t.Fatalf("Alpha: got views %v opening on %q, want %v opening on discover", folders[0].Views, folders[0].DefaultView, want)
	}
	if want := []string{"artists", "albums"}; !slices.Equal(folders[1].Views, want) || folders[1].DefaultView != "artists" {
		t.Fatalf("Zulu: got views %v opening on %q, want %v opening on artists", folders[1].Views, folders[1].DefaultView, want)
	}
	if folders[1].Icon != "heart" {
		t.Fatalf("Zulu: expected icon=heart, got %q", folders[1].Icon)
	}
	if folders[0].Icon != "folder" {
		t.Fatalf("Alpha: expected icon=folder (default), got %q", folders[0].Icon)
	}
	if folders[0].SplitViews || !folders[1].SplitViews {
		t.Fatalf("got splitViews Alpha=%v Zulu=%v, want false (default) and true", folders[0].SplitViews, folders[1].SplitViews)
	}

}

// The root is described like a folder, without an id: every view, opening on
// Discover, split until the catalog settings say otherwise.
func TestGetMusicFoldersCatalog(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()

	var body musicFoldersBody
	decodeJSON(t, srv.URL+"/rest/getMusicFolders.view", &body)
	catalog := body.SubsonicResponse.MusicFolders.Catalog
	if want := []string{"discover", "artists", "albums"}; !slices.Equal(catalog.Views, want) || catalog.DefaultView != "discover" {
		t.Fatalf("got views %v opening on %q, want %v opening on discover", catalog.Views, catalog.DefaultView, want)
	}
	if !catalog.SplitViews {
		t.Fatal("expected the root to split its views by default")
	}

	if err := s.SaveCatalogSettings(model.CatalogSettings{SplitViews: false}); err != nil {
		t.Fatal(err)
	}
	decodeJSON(t, srv.URL+"/rest/getMusicFolders.view", &body)
	if body.SubsonicResponse.MusicFolders.Catalog.SplitViews {
		t.Fatal("expected splitViews=false once saved")
	}
}

func TestGetAlbumDiscTitles(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	db.Create(&model.Library{Name: "Lib"})
	album := &model.Album{Name: "Box Set", NameNorm: "box set", AlbumArtistNorm: "a"}
	db.Create(album)
	db.Create(&model.Track{AlbumID: album.ID, Filename: "1.flac", FilePath: "/l/1.flac",
		Title: "One", TrackNumber: 1, DiscNumber: 1, DiscSubtitle: "The Album"})
	// A disc with no subtitle must not produce an entry.
	db.Create(&model.Track{AlbumID: album.ID, Filename: "2.flac", FilePath: "/l/2.flac",
		Title: "Two", TrackNumber: 1, DiscNumber: 2})
	db.Create(&model.Track{AlbumID: album.ID, Filename: "3.flac", FilePath: "/l/3.flac",
		Title: "Three", TrackNumber: 1, DiscNumber: 3, DiscSubtitle: "Bonus Tracks"})

	srv := newTestServer(t, s)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rest/getAlbum.view?id=" + encodeAlbumID(album.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	var body struct {
		SubsonicResponse struct {
			Album struct {
				DiscTitles []struct {
					Disc  int    `json:"disc"`
					Title string `json:"title"`
				} `json:"discTitles"`
			} `json:"album"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	titles := body.SubsonicResponse.Album.DiscTitles
	if len(titles) != 2 {
		t.Fatalf("expected 2 disc titles, got %+v", titles)
	}
	if titles[0].Disc != 1 || titles[0].Title != "The Album" {
		t.Fatalf("unexpected first disc title: %+v", titles[0])
	}
	if titles[1].Disc != 3 || titles[1].Title != "Bonus Tracks" {
		t.Fatalf("unexpected second disc title: %+v", titles[1])
	}
}

// getArtist's album array feeds an artist page's album cards, which show a track
// count — GetArtist does not preload Tracks, so the aggregates must be batched in
// the same way getAlbumList2 does it.
func TestGetArtistAlbumsIncludeSongCountAndDuration(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	db.Create(&model.Library{Name: "Lib"})
	artist := &model.Artist{Name: "Radiohead", NameNorm: "radiohead"}
	db.Create(artist)
	album := &model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead"}
	db.Create(album)
	_ = db.Model(album).Association("Artists").Replace([]*model.Artist{artist})
	db.Create(&model.Track{AlbumID: album.ID, Filename: "1.flac", FilePath: "/l/1.flac", TrackNumber: 1, Duration: 260})
	db.Create(&model.Track{AlbumID: album.ID, Filename: "2.flac", FilePath: "/l/2.flac", TrackNumber: 2, Duration: 240})

	srv := newTestServer(t, s)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rest/getArtist.view?id=" + encodeArtistID(artist.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	var body struct {
		SubsonicResponse struct {
			Artist struct {
				Album []struct {
					Name      string `json:"name"`
					SongCount int    `json:"songCount"`
					Duration  int    `json:"duration"`
				} `json:"album"`
			} `json:"artist"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	albums := body.SubsonicResponse.Artist.Album
	if len(albums) != 1 {
		t.Fatalf("expected 1 album, got %d", len(albums))
	}
	if albums[0].SongCount != 2 {
		t.Errorf("songCount = %d, want 2", albums[0].SongCount)
	}
	if albums[0].Duration != 500 {
		t.Errorf("duration = %d, want 500", albums[0].Duration)
	}
}

// A client iterates views, so the wire carries an array even for a row stored
// without any (only a direct write can produce one; the API refuses it).
func TestGetMusicFoldersViewsIsAlwaysAnArray(t *testing.T) {
	s := testStore(t)
	if err := s.DB().Exec("INSERT INTO libraries (name, views) VALUES (?, ?)", "Bare", "").Error; err != nil {
		t.Fatal(err)
	}
	srv := newTestServer(t, s)
	defer srv.Close()

	var body struct {
		SubsonicResponse struct {
			MusicFolders struct {
				MusicFolder []map[string]json.RawMessage `json:"musicFolder"`
			} `json:"musicFolders"`
		} `json:"subsonic-response"`
	}
	decodeJSON(t, srv.URL+"/rest/getMusicFolders.view", &body)
	folders := body.SubsonicResponse.MusicFolders.MusicFolder
	if len(folders) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(folders))
	}
	if got := string(folders[0]["views"]); got != "[]" {
		t.Fatalf(`views = %s, want the JSON array "[]"`, got)
	}
}

func TestAlbumToMapReleaseTypes(t *testing.T) {
	m := albumToMap(&model.Album{ReleaseTypes: []string{"Album", "Compilation"}})
	got, ok := m["releaseTypes"].([]string)
	if !ok || len(got) != 2 || got[0] != "Album" || got[1] != "Compilation" {
		t.Errorf("releaseTypes = %v, want [Album Compilation]", m["releaseTypes"])
	}

	// Omitted entirely when the album carries no release types.
	if _, ok := albumToMap(&model.Album{})["releaseTypes"]; ok {
		t.Error("releaseTypes should be omitted when the list is empty")
	}
}

// isCompilation unions the two independent sources OpenSubsonic keeps side by
// side: the iTunes compilation flag and a MusicBrainz "Compilation" secondary
// type. Either one alone is enough; matching is case-insensitive.
func TestAlbumToMapIsCompilation(t *testing.T) {
	cases := []struct {
		name  string
		album model.Album
		want  bool
	}{
		{"neither", model.Album{ReleaseTypes: []string{"Album"}}, false},
		{"itunes flag only", model.Album{Compilation: true}, true},
		{"release type only", model.Album{ReleaseTypes: []string{"Album", "Compilation"}}, true},
		{"release type any case", model.Album{ReleaseTypes: []string{"compilation"}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := albumToMap(&tc.album)["isCompilation"].(bool)
			if got != tc.want {
				t.Errorf("isCompilation = %v, want %v", got, tc.want)
			}
		})
	}
}

// Hiding a library from the artist index keeps its artists off the
// cross-library index only: the library's own index still lists them.
func TestGetArtistsOfHiddenLibraryListsItsArtists(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	hid := model.Library{Name: "Hid", Filters: scanFolderFilter("Hid"), HideFromArtistIndex: true}
	db.Create(&hid)
	artist := model.Artist{Name: "Hidden Artist", NameNorm: "hidden artist"}
	db.Create(&artist)
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "hidden artist"}
	db.Create(&album)
	_ = db.Model(&album).Association("Artists").Replace([]*model.Artist{&artist})
	db.Create(&model.Track{AlbumID: album.ID, ScanFolder: "Hid", Filename: "1.mp3", FilePath: "/hid/1.mp3"})

	srv := newTestServer(t, s)
	defer srv.Close()

	artistsAt := func(query string) []string {
		t.Helper()
		var body struct {
			SubsonicResponse struct {
				Status  string `json:"status"`
				Artists struct {
					Index []struct {
						Artist []starredItem `json:"artist"`
					} `json:"index"`
				} `json:"artists"`
			} `json:"subsonic-response"`
		}
		decodeJSON(t, srv.URL+"/rest/getArtists.view"+query, &body)
		if body.SubsonicResponse.Status != "ok" {
			t.Fatalf("%s: status = %q, want ok", query, body.SubsonicResponse.Status)
		}
		var ids []string
		for _, letter := range body.SubsonicResponse.Artists.Index {
			ids = append(ids, idsOf(letter.Artist)...)
		}
		return ids
	}

	if got := artistsAt("?musicFolderId=" + strconv.FormatUint(uint64(hid.ID), 10)); !slices.Equal(got, []string{encodeArtistID(artist.ID)}) {
		t.Fatalf("the hidden library's own index = %v, want its artist", got)
	}
	if got := artistsAt(""); len(got) != 0 {
		t.Fatalf("the cross-library index = %v, want the hidden library's artist left out", got)
	}
}

// musicFolderId is optional and unvalidated by the spec: an id that names no
// library must keep answering empty lists, not an error and not everything.
func TestUnknownMusicFolderAnswersEmptyLists(t *testing.T) {
	f := newStarFixture(t)
	srv := newTestServer(t, f.store)
	defer srv.Close()

	var body struct {
		SubsonicResponse struct {
			Status     string `json:"status"`
			AlbumList2 struct {
				Album []starredItem `json:"album"`
			} `json:"albumList2"`
		} `json:"subsonic-response"`
	}
	decodeJSON(t, srv.URL+"/rest/getAlbumList2.view?type=alphabeticalByName&musicFolderId=999", &body)
	if body.SubsonicResponse.Status != "ok" {
		t.Fatalf("status = %q, want ok", body.SubsonicResponse.Status)
	}
	if n := len(body.SubsonicResponse.AlbumList2.Album); n != 0 {
		t.Fatalf("expected no albums for an unknown music folder, got %d", n)
	}
}

// A store failure while resolving musicFolderId is an error, not "no tracks":
// an empty successful list is something a client caches.
func TestMusicFolderLookupFailureIsAnError(t *testing.T) {
	f := newStarFixture(t)
	if err := f.store.DB().Exec("DROP TABLE libraries").Error; err != nil {
		t.Fatal(err)
	}
	srv := newTestServer(t, f.store)
	defer srv.Close()

	var body struct {
		SubsonicResponse struct {
			Status string `json:"status"`
		} `json:"subsonic-response"`
	}
	decodeJSON(t, srv.URL+"/rest/getAlbumList2.view?type=alphabeticalByName&musicFolderId=1", &body)
	if body.SubsonicResponse.Status != "failed" {
		t.Fatalf("status = %q, want failed", body.SubsonicResponse.Status)
	}
}

// TestGetMusicFoldersShapeIsUnchanged pins getMusicFolders to exactly the
// fields OpenSubsonic plus its two advertised extensions (musicFolderViews,
// musicFolderIcon) define. A library's filters and whether it is hidden from
// the artist index are purely internal, server-side details and must never
// leak onto the wire.
func TestGetMusicFoldersShapeIsUnchanged(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	db.Create(&model.Library{
		Name: "Filtered", DefaultView: model.ViewArtists, HideFromArtistIndex: true, Icon: "heart",
		Filters: []model.LibraryFilter{{Field: model.FilterFormat, Values: []string{"flac"}}},
	})

	srv := newTestServer(t, s)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rest/getMusicFolders.view")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	var body struct {
		SubsonicResponse struct {
			MusicFolders struct {
				MusicFolder []map[string]json.RawMessage `json:"musicFolder"`
			} `json:"musicFolders"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	folders := body.SubsonicResponse.MusicFolders.MusicFolder
	if len(folders) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(folders))
	}
	got := make([]string, 0, len(folders[0]))
	for k := range folders[0] {
		got = append(got, k)
	}
	slices.Sort(got)
	want := []string{"defaultView", "icon", "id", "name", "splitViews", "views"}
	if !slices.Equal(got, want) {
		t.Fatalf("musicFolder keys = %v, want exactly %v — no filter may leak into /rest", got, want)
	}
}

// narrowEnvelope decodes whichever /rest endpoint
// TestMusicFolderIdNarrowsByFilters is currently calling; each request
// populates only its own field and leaves the rest at their zero value.
type narrowEnvelope struct {
	SubsonicResponse struct {
		AlbumList2 struct {
			Album []starredItem `json:"album"`
		} `json:"albumList2"`
		AlbumList2Index struct {
			Total int `json:"total"`
			Index []struct {
				Name string `json:"name"`
			} `json:"index"`
		} `json:"albumList2Index"`
		Artists struct {
			Index []struct {
				Artist []starredItem `json:"artist"`
			} `json:"index"`
		} `json:"artists"`
		SearchResult3 struct {
			Artist []starredItem `json:"artist"`
			Album  []starredItem `json:"album"`
			Song   []starredItem `json:"song"`
			Genre  []struct {
				Value string `json:"value"`
			} `json:"genre"`
		} `json:"searchResult3"`
		Starred2 struct {
			Artist []starredItem `json:"artist"`
			Album  []starredItem `json:"album"`
			Song   []starredItem `json:"song"`
		} `json:"starred2"`
		RandomSongs struct {
			Song []starredItem `json:"song"`
		} `json:"randomSongs"`
		SongsByGenre struct {
			Song []starredItem `json:"song"`
		} `json:"songsByGenre"`
		Discovery struct {
			Album []starredItem `json:"album"`
		} `json:"discovery"`
	} `json:"subsonic-response"`
}

// idsOf pulls the id out of a starredItem slice — every Subsonic entity this
// file decodes carries one.
func idsOf(items []starredItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

// assertIDSet compares two id lists as SETS: several of the endpoints under
// test (getRandomSongs, search3, the discovery/searchGenres extensions) make
// no ordering promise, so membership is all a caller may rely on.
func assertIDSet(t *testing.T, what string, got, want []string) {
	t.Helper()
	g, w := slices.Clone(got), slices.Clone(want)
	slices.Sort(g)
	slices.Sort(w)
	if !slices.Equal(g, w) {
		t.Errorf("%s ids = %v, want %v", what, g, w)
	}
}

// TestMusicFolderIdNarrowsByFilters proves musicFolderId narrows every /rest
// list and search endpoint to a library's compiled filters — the contract
// libraryScope promises via store.LibraryScope(&found) (subsonic.go). One
// catalog is shared by every case: two scan folders, a flac and an mp3 track,
// three release-type shapes (Single, Album, untyped) plus a compilation, and
// two genres. Four libraries slice it differently:
//
//	Lossless    = {format: [flac]}
//	Singles     = {release_type: [Single]}
//	JazzInMusic = {scan_folder: [Music]} AND {genre: [Jazz]}
//	Everything  = no filters (the whole catalog)
func TestMusicFolderIdNarrowsByFilters(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	rock := model.Genre{Name: "Rock"}
	jazz := model.Genre{Name: "Jazz"}
	db.Create(&rock)
	db.Create(&jazz)

	// single: release type Single, mp3, Music folder, Rock.
	artistSingle := model.Artist{Name: "Artist Aardvark", NameNorm: "artist aardvark"}
	db.Create(&artistSingle)
	single := model.Album{Name: "Aardvark", NameNorm: "aardvark", AlbumArtistNorm: "artist aardvark", ReleaseTypes: []string{"Single"}}
	db.Create(&single)
	trackSingle := model.Track{AlbumID: single.ID, ScanFolder: "Music", Suffix: "mp3", Filename: "1.mp3", FilePath: "/music/1.mp3", Title: "One"}
	db.Create(&trackSingle)

	// lp: release type Album, flac, Music folder, Jazz.
	artistLP := model.Artist{Name: "Artist Bebop", NameNorm: "artist bebop"}
	db.Create(&artistLP)
	lp := model.Album{Name: "Bebop", NameNorm: "bebop", AlbumArtistNorm: "artist bebop", ReleaseTypes: []string{"Album"}}
	db.Create(&lp)
	trackLP := model.Track{AlbumID: lp.ID, ScanFolder: "Music", Suffix: "flac", Filename: "2.flac", FilePath: "/music/2.flac", Title: "Two"}
	db.Create(&trackLP)

	// untyped: no release type, mp3, Vinyl folder, Jazz.
	artistUntyped := model.Artist{Name: "Artist Crate", NameNorm: "artist crate"}
	db.Create(&artistUntyped)
	untyped := model.Album{Name: "Crate", NameNorm: "crate", AlbumArtistNorm: "artist crate"}
	db.Create(&untyped)
	trackUntyped := model.Track{AlbumID: untyped.ID, ScanFolder: "Vinyl", Suffix: "mp3", Filename: "3.mp3", FilePath: "/vinyl/3.mp3", Title: "Three"}
	db.Create(&trackUntyped)

	// comp: a compilation, flac, Vinyl folder, Rock.
	artistComp := model.Artist{Name: "Artist Disco", NameNorm: "artist disco"}
	db.Create(&artistComp)
	comp := model.Album{Name: "Disco", NameNorm: "disco", AlbumArtistNorm: "artist disco", ReleaseTypes: []string{"Compilation"}, Compilation: true}
	db.Create(&comp)
	trackComp := model.Track{AlbumID: comp.ID, ScanFolder: "Vinyl", Suffix: "flac", Filename: "4.flac", FilePath: "/vinyl/4.flac", Title: "Four"}
	db.Create(&trackComp)

	// Credit each artist on its album (album_artists — what getArtists joins)
	// and its track (track_artists — what search3 joins), and tag each
	// track's genre.
	_ = db.Model(&single).Association("Artists").Replace([]*model.Artist{&artistSingle})
	_ = db.Model(&trackSingle).Association("Artists").Replace([]*model.Artist{&artistSingle})
	_ = db.Model(&trackSingle).Association("Genres").Replace([]*model.Genre{&rock})

	_ = db.Model(&lp).Association("Artists").Replace([]*model.Artist{&artistLP})
	_ = db.Model(&trackLP).Association("Artists").Replace([]*model.Artist{&artistLP})
	_ = db.Model(&trackLP).Association("Genres").Replace([]*model.Genre{&jazz})

	_ = db.Model(&untyped).Association("Artists").Replace([]*model.Artist{&artistUntyped})
	_ = db.Model(&trackUntyped).Association("Artists").Replace([]*model.Artist{&artistUntyped})
	_ = db.Model(&trackUntyped).Association("Genres").Replace([]*model.Genre{&jazz})

	_ = db.Model(&comp).Association("Artists").Replace([]*model.Artist{&artistComp})
	_ = db.Model(&trackComp).Association("Artists").Replace([]*model.Artist{&artistComp})
	_ = db.Model(&trackComp).Association("Genres").Replace([]*model.Genre{&rock})

	// Star everything so getStarred2 has content to narrow.
	for _, al := range []model.Album{single, lp, untyped, comp} {
		db.Create(&model.StarredItem{Owner: "admin", ItemType: "album", ItemID: al.ID})
	}
	for _, ar := range []model.Artist{artistSingle, artistLP, artistUntyped, artistComp} {
		db.Create(&model.StarredItem{Owner: "admin", ItemType: "artist", ItemID: ar.ID})
	}
	for _, tr := range []model.Track{trackSingle, trackLP, trackUntyped, trackComp} {
		db.Create(&model.StarredItem{Owner: "admin", ItemType: "track", ItemID: tr.ID})
	}

	libLossless := model.Library{Name: "Lossless", Filters: []model.LibraryFilter{
		{Field: model.FilterFormat, Values: []string{"flac"}},
	}}
	libSingles := model.Library{Name: "Singles", Filters: []model.LibraryFilter{
		{Field: model.FilterReleaseType, Values: []string{"Single"}},
	}}
	libJazzInMusic := model.Library{Name: "JazzInMusic", Filters: []model.LibraryFilter{
		{Field: model.FilterScanFolder, Values: []string{"Music"}},
		{Field: model.FilterGenre, Values: []string{"Jazz"}},
	}}
	libEverything := model.Library{Name: "Everything"}
	for _, lib := range []*model.Library{&libLossless, &libSingles, &libJazzInMusic, &libEverything} {
		db.Create(lib)
	}

	srv := newTestServer(t, s)
	defer srv.Close()

	cases := []struct {
		name        string
		hasLib      bool // false = no musicFolderId param at all
		libID       uint
		wantAlbums  []string
		wantArtists []string
		wantSongs   []string
		wantJazz    []string // getSongsByGenre?genre=Jazz membership
		wantGenres  []string // search3's searchGenres extension, by value
		wantLetters []string // getAlbumList2Index letter buckets
	}{
		{
			name: "Lossless", hasLib: true, libID: libLossless.ID,
			wantAlbums:  []string{encodeAlbumID(lp.ID), encodeAlbumID(comp.ID)},
			wantArtists: []string{encodeArtistID(artistLP.ID), encodeArtistID(artistComp.ID)},
			wantSongs:   []string{encodeTrackID(trackLP.ID), encodeTrackID(trackComp.ID)},
			wantJazz:    []string{encodeTrackID(trackLP.ID)},
			wantGenres:  []string{"Jazz", "Rock"},
			wantLetters: []string{"B", "D"},
		},
		{
			name: "Singles", hasLib: true, libID: libSingles.ID,
			wantAlbums:  []string{encodeAlbumID(single.ID)},
			wantArtists: []string{encodeArtistID(artistSingle.ID)},
			wantSongs:   []string{encodeTrackID(trackSingle.ID)},
			wantJazz:    nil,
			wantGenres:  []string{"Rock"},
			wantLetters: []string{"A"},
		},
		{
			name: "JazzInMusic", hasLib: true, libID: libJazzInMusic.ID,
			wantAlbums:  []string{encodeAlbumID(lp.ID)},
			wantArtists: []string{encodeArtistID(artistLP.ID)},
			wantSongs:   []string{encodeTrackID(trackLP.ID)},
			wantJazz:    []string{encodeTrackID(trackLP.ID)},
			wantGenres:  []string{"Jazz"},
			wantLetters: []string{"B"},
		},
		{
			name: "Everything", hasLib: true, libID: libEverything.ID,
			wantAlbums: []string{encodeAlbumID(single.ID), encodeAlbumID(lp.ID), encodeAlbumID(untyped.ID), encodeAlbumID(comp.ID)},
			wantArtists: []string{
				encodeArtistID(artistSingle.ID), encodeArtistID(artistLP.ID),
				encodeArtistID(artistUntyped.ID), encodeArtistID(artistComp.ID),
			},
			wantSongs: []string{
				encodeTrackID(trackSingle.ID), encodeTrackID(trackLP.ID),
				encodeTrackID(trackUntyped.ID), encodeTrackID(trackComp.ID),
			},
			wantJazz:    []string{encodeTrackID(trackLP.ID), encodeTrackID(trackUntyped.ID)},
			wantGenres:  []string{"Jazz", "Rock"},
			wantLetters: []string{"A", "B", "C", "D"},
		},
		{
			// No musicFolderId at all must answer exactly what Everything
			// does: both compile to the same zero-value scope
			// (store.ScopeOf(nil) — see store/scope.go).
			name: "no musicFolderId", hasLib: false,
			wantAlbums: []string{encodeAlbumID(single.ID), encodeAlbumID(lp.ID), encodeAlbumID(untyped.ID), encodeAlbumID(comp.ID)},
			wantArtists: []string{
				encodeArtistID(artistSingle.ID), encodeArtistID(artistLP.ID),
				encodeArtistID(artistUntyped.ID), encodeArtistID(artistComp.ID),
			},
			wantSongs: []string{
				encodeTrackID(trackSingle.ID), encodeTrackID(trackLP.ID),
				encodeTrackID(trackUntyped.ID), encodeTrackID(trackComp.ID),
			},
			wantJazz:    []string{encodeTrackID(trackLP.ID), encodeTrackID(trackUntyped.ID)},
			wantGenres:  []string{"Jazz", "Rock"},
			wantLetters: []string{"A", "B", "C", "D"},
		},
		{
			// A musicFolderId naming no library must answer empty lists
			// everywhere — not an error, and not the whole catalog.
			name: "unknown musicFolderId", hasLib: true, libID: 999999,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fetch := func(path string, params url.Values) narrowEnvelope {
				if tc.hasLib {
					params.Set("musicFolderId", strconv.FormatUint(uint64(tc.libID), 10))
				}
				var body narrowEnvelope
				decodeJSON(t, srv.URL+"/rest/"+path+".view?"+params.Encode(), &body)
				return body
			}

			albumList := fetch("getAlbumList2", url.Values{"type": {"alphabeticalByName"}, "size": {"50"}})
			assertIDSet(t, "getAlbumList2", idsOf(albumList.SubsonicResponse.AlbumList2.Album), tc.wantAlbums)

			letterIdx := fetch("getAlbumList2Index", url.Values{})
			var gotLetters []string
			for _, l := range letterIdx.SubsonicResponse.AlbumList2Index.Index {
				gotLetters = append(gotLetters, l.Name)
			}
			assertIDSet(t, "getAlbumList2Index letters", gotLetters, tc.wantLetters)
			if got, want := letterIdx.SubsonicResponse.AlbumList2Index.Total, len(tc.wantAlbums); got != want {
				t.Errorf("getAlbumList2Index total = %d, want %d", got, want)
			}

			artistsResp := fetch("getArtists", url.Values{})
			var gotArtists []string
			for _, idx := range artistsResp.SubsonicResponse.Artists.Index {
				gotArtists = append(gotArtists, idsOf(idx.Artist)...)
			}
			assertIDSet(t, "getArtists", gotArtists, tc.wantArtists)

			search := fetch("search3", url.Values{
				"query": {""}, "artistCount": {"50"}, "albumCount": {"50"}, "songCount": {"50"}, "genreCount": {"10"},
			})
			assertIDSet(t, "search3 artist", idsOf(search.SubsonicResponse.SearchResult3.Artist), tc.wantArtists)
			assertIDSet(t, "search3 album", idsOf(search.SubsonicResponse.SearchResult3.Album), tc.wantAlbums)
			assertIDSet(t, "search3 song", idsOf(search.SubsonicResponse.SearchResult3.Song), tc.wantSongs)
			var gotGenres []string
			for _, g := range search.SubsonicResponse.SearchResult3.Genre {
				gotGenres = append(gotGenres, g.Value)
			}
			assertIDSet(t, "search3 searchGenres extension", gotGenres, tc.wantGenres)

			starred := fetch("getStarred2", url.Values{})
			assertIDSet(t, "getStarred2 album", idsOf(starred.SubsonicResponse.Starred2.Album), tc.wantAlbums)
			assertIDSet(t, "getStarred2 artist", idsOf(starred.SubsonicResponse.Starred2.Artist), tc.wantArtists)
			assertIDSet(t, "getStarred2 song", idsOf(starred.SubsonicResponse.Starred2.Song), tc.wantSongs)

			random := fetch("getRandomSongs", url.Values{"size": {"50"}})
			assertIDSet(t, "getRandomSongs", idsOf(random.SubsonicResponse.RandomSongs.Song), tc.wantSongs)

			byGenre := fetch("getSongsByGenre", url.Values{"genre": {"Jazz"}, "count": {"50"}})
			assertIDSet(t, "getSongsByGenre", idsOf(byGenre.SubsonicResponse.SongsByGenre.Song), tc.wantJazz)

			disc := fetch("getDiscovery", url.Values{"size": {"50"}})
			assertIDSet(t, "getDiscovery", idsOf(disc.SubsonicResponse.Discovery.Album), tc.wantAlbums)
		})
	}
}
