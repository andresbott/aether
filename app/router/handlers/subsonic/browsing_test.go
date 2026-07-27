package subsonic

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andresbott/aether/internal/assetstore"
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

func newTestServer(t *testing.T, s *store.Store) *httptest.Server {
	t.Helper()
	as := assetstore.New(t.TempDir())
	r := mux.NewRouter()
	Register(r, s, as, t.TempDir())
	return httptest.NewServer(r)
}

func TestGetMusicFoldersFromDB(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	db.Create(&model.Library{Name: "Zulu", Path: "/z", DefaultView: "artists", HideArtists: true, Icon: "heart"})
	db.Create(&model.Library{Name: "Alpha", Path: "/a", DefaultView: "albums", HideArtists: false})

	srv := newTestServer(t, s)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rest/getMusicFolders.view")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		SubsonicResponse struct {
			MusicFolders struct {
				MusicFolder []struct {
					ID          uint   `json:"id"`
					Name        string `json:"name"`
					DefaultView string `json:"defaultView"`
					ShowArtists bool   `json:"showArtists"`
					Icon        string `json:"icon"`
				} `json:"musicFolder"`
			} `json:"musicFolders"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	folders := body.SubsonicResponse.MusicFolders.MusicFolder
	if len(folders) != 2 {
		t.Fatalf("expected 2 folders, got %d", len(folders))
	}
	// Ordered by name ascending (ListLibraries order).
	if folders[0].Name != "Alpha" || folders[1].Name != "Zulu" {
		t.Fatalf("unexpected order: %+v", folders)
	}
	if folders[0].DefaultView != "albums" {
		t.Fatalf("Alpha: expected defaultView=albums, got %q", folders[0].DefaultView)
	}
	if folders[1].DefaultView != "artists" {
		t.Fatalf("Zulu: expected defaultView=artists, got %q", folders[1].DefaultView)
	}
	if !folders[0].ShowArtists {
		t.Fatalf("Alpha: expected showArtists=true, got false")
	}
	if folders[1].ShowArtists {
		t.Fatalf("Zulu: expected showArtists=false, got true")
	}
	if folders[1].Icon != "heart" {
		t.Fatalf("Zulu: expected icon=heart, got %q", folders[1].Icon)
	}
	if folders[0].Icon != "folder" {
		t.Fatalf("Alpha: expected icon=folder (default), got %q", folders[0].Icon)
	}
}

func TestGetAlbumDiscTitles(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	db.Create(&model.Library{Name: "Lib", Path: "/l"})
	album := &model.Album{Name: "Box Set", NameNorm: "box set", AlbumArtistNorm: "a"}
	db.Create(album)
	db.Create(&model.Track{AlbumID: album.ID, LibraryID: 1, Filename: "1.flac", FilePath: "/l/1.flac",
		Title: "One", TrackNumber: 1, DiscNumber: 1, DiscSubtitle: "The Album"})
	// A disc with no subtitle must not produce an entry.
	db.Create(&model.Track{AlbumID: album.ID, LibraryID: 1, Filename: "2.flac", FilePath: "/l/2.flac",
		Title: "Two", TrackNumber: 1, DiscNumber: 2})
	db.Create(&model.Track{AlbumID: album.ID, LibraryID: 1, Filename: "3.flac", FilePath: "/l/3.flac",
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

func TestGetMusicFoldersDefaultViewFallback(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	// Explicitly insert a library with empty DefaultView to simulate legacy rows.
	db.Exec("INSERT INTO libraries (name, path, default_view, hide_artists) VALUES (?, ?, ?, ?)", "Legacy", "/l", "", false)

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
				MusicFolder []struct {
					Name        string `json:"name"`
					DefaultView string `json:"defaultView"`
				} `json:"musicFolder"`
			} `json:"musicFolders"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	folders := body.SubsonicResponse.MusicFolders.MusicFolder
	if len(folders) != 1 || folders[0].DefaultView != "albums" {
		t.Fatalf("expected one folder with defaultView=albums, got %+v", folders)
	}
}
