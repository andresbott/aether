package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestMediaGuardWiredFromConfiguredScanFolders proves that New() actually
// reaches the /rest media handlers with the configured scan folder roots
// (main.go: subsonic.WithMediaRoots(cfg.ScanFolders.Roots()...)), not just
// that the subsonic package's WithMediaRoots option works in isolation —
// handlers/subsonic/media_test.go already covers that at the unit level.
// Dropping that wiring, or dropping ScanFolders from Cfg, leaves Roots()
// answering nil on a nil set, installs no guard, and serves every
// DB-recorded path — a miswire that fails OPEN and that no other test would
// notice.
func TestMediaGuardWiredFromConfiguredScanFolders(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := store.New(db)

	rootA := t.TempDir() // the only configured scan folder root
	rootB := t.TempDir() // outside every scan folder

	insidePath := filepath.Join(rootA, "inside.mp3")
	if err := os.WriteFile(insidePath, []byte("inside-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	outsidePath := filepath.Join(rootB, "outside.mp3")
	if err := os.WriteFile(outsidePath, []byte("outside-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y"}
	if err := db.Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	inside := model.Track{AlbumID: album.ID, Filename: "inside.mp3", FilePath: insidePath}
	if err := db.Create(&inside).Error; err != nil {
		t.Fatal(err)
	}
	outside := model.Track{AlbumID: album.ID, Filename: "outside.mp3", FilePath: outsidePath}
	if err := db.Create(&outside).Error; err != nil {
		t.Fatal(err)
	}

	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Music", Path: rootA}})
	if err != nil {
		t.Fatal(err)
	}

	h, err := New(Cfg{
		Store:       s,
		DataDir:     t.TempDir(),
		ScanFolders: set,
		AuthMethod:  "none",
	})
	if err != nil {
		t.Fatal(err)
	}

	// The outside track: the media guard, wired from the configured scan
	// folder set, must refuse it — a Subsonic "not found" (code 70), the same
	// answer a client gets for any id that does not exist. A miswire that
	// installs no guard would instead serve it with a 200.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/rest/stream.view?f=json&id=tr-%d", outside.ID), nil))
	var body struct {
		SubsonicResponse struct {
			Status string `json:"status"`
			Error  *struct {
				Code int `json:"code"`
			} `json:"error"`
		} `json:"subsonic-response"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad body %q: %v", w.Body.String(), err)
	}
	if body.SubsonicResponse.Status != "failed" || body.SubsonicResponse.Error == nil || body.SubsonicResponse.Error.Code != 70 {
		t.Fatalf("stream for a track outside every scan folder root = %+v (http %d), want status failed / code 70 (media guard not wired)",
			body.SubsonicResponse, w.Code)
	}

	// The inside track streams normally: the guard must not be over-broad.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/rest/stream.view?f=json&id=tr-%d", inside.ID), nil))
	if w.Code != http.StatusOK {
		t.Fatalf("stream for a track inside the configured scan folder root = %d, want 200: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "inside-bytes" {
		t.Fatalf("served %q, want the track's file bytes", w.Body.String())
	}
}
