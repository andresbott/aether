package scanfolders_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/app/router/handlers/problems"
	"github.com/andresbott/aether/app/router/handlers/scanfolders"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type folderBody struct {
	Name            string   `json:"name"`
	Path            string   `json:"path"`
	ExcludePatterns []string `json:"exclude_patterns"`
	FollowSymlinks  bool     `json:"follow_symlinks"`
	Available       bool     `json:"available"`
	Problem         string   `json:"problem"`
	TrackCount      int64    `json:"track_count"`
}

func newServer(t *testing.T, folders ...scanfolder.Folder) (*httptest.Server, *store.Store) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := store.New(db)
	set, err := scanfolder.NewSet(folders)
	if err != nil {
		t.Fatal(err)
	}
	r := mux.NewRouter()
	(&scanfolders.Handler{Folders: set, Store: s, Problems: problems.New(false)}).Routes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, s
}

func list(t *testing.T, srv *httptest.Server) []folderBody {
	t.Helper()
	resp, err := http.Get(srv.URL + "/scan-folders")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		ScanFolders []folderBody `json:"scan_folders"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body.ScanFolders
}

func TestListScanFolders(t *testing.T) {
	music := t.TempDir()
	missing := filepath.Join(t.TempDir(), "not-mounted")
	srv, s := newServer(t,
		scanfolder.Folder{Name: "Music", Path: music, ExcludePatterns: []string{`^\.`}, FollowSymlinks: true},
		scanfolder.Folder{Name: "Books", Path: missing},
	)
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	if err := s.DB().Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	for _, tr := range []model.Track{
		// Two tracks stamped Music — one of them recorded OUTSIDE the root, as
		// content reached through a symlink is. The count goes by the marker, not
		// by the path.
		{AlbumID: album.ID, ScanFolder: "Music", Filename: "1.mp3", FilePath: filepath.Join(music, "1.mp3")},
		{AlbumID: album.ID, ScanFolder: "Music", Filename: "2.mp3", FilePath: "/mnt/disk2/2.mp3"},
		// Under Music's root but stamped with another name: NOT counted for Music.
		{AlbumID: album.ID, ScanFolder: "Old Name", Filename: "3.mp3", FilePath: filepath.Join(music, "3.mp3")},
	} {
		if err := s.DB().Create(&tr).Error; err != nil {
			t.Fatal(err)
		}
	}

	got := list(t, srv)
	if len(got) != 2 || got[0].Name != "Books" || got[1].Name != "Music" {
		t.Fatalf("expected Books, Music in name order, got %+v", got)
	}
	books, musicF := got[0], got[1]
	if books.Available || books.Problem == "" || books.TrackCount != 0 {
		t.Fatalf("Books must be reported unavailable with a reason: %+v", books)
	}
	if books.ExcludePatterns == nil {
		t.Fatal("exclude_patterns must be [] rather than null when there are none")
	}
	if !musicF.Available || musicF.Problem != "" {
		t.Fatalf("Music must be available: %+v", musicF)
	}
	if musicF.Path != music || !musicF.FollowSymlinks || len(musicF.ExcludePatterns) != 1 {
		t.Fatalf("Music's config not reported: %+v", musicF)
	}
	if musicF.TrackCount != 2 {
		t.Fatalf("Music track_count = %d, want 2 (by marker)", musicF.TrackCount)
	}
}

func TestListScanFoldersEmpty(t *testing.T) {
	srv, _ := newServer(t)
	if got := list(t, srv); got == nil || len(got) != 0 {
		t.Fatalf("expected an empty array, got %#v", got)
	}
}
