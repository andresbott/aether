package subsonic

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	r := mux.NewRouter()
	Register(r, s, "", "")
	return httptest.NewServer(r)
}

func TestGetMusicFoldersFromDB(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	db.Create(&model.Library{Name: "Zulu", Path: "/z"})
	db.Create(&model.Library{Name: "Alpha", Path: "/a"})

	srv := newTestServer(t, s)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rest/getMusicFolders.view")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		SubsonicResponse struct {
			MusicFolders struct {
				MusicFolder []struct {
					ID   uint   `json:"id"`
					Name string `json:"name"`
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
}
