package metadata_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	metaHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// newCapHandler builds one handler with every optional service wired — a
// fake Identifier/AlbumIdentifier so identify/identify-album reach their
// paths[] validation instead of short-circuiting on 503 — for exercising the
// shared maxSelectionPaths cap across every paths[]-accepting endpoint.
func newCapHandler(t *testing.T) (*mux.Router, *model.Library) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := store.New(db)
	lib := &model.Library{Name: "Main", Path: t.TempDir(), FollowSymlinks: true}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	h := &metaHandler.Handler{
		Store:           s,
		Reader:          nullReader{},
		Identifier:      fakeIdentifier{},
		AlbumIdentifier: &fakeAlbumIdentifier{},
	}
	r := mux.NewRouter()
	h.Routes(r)
	return r, lib
}

// TestCapAppliesUniformly confirms maxSelectionPaths (50) and its "too many
// paths in one request" message are shared by every paths[]-accepting
// endpoint. Before this unification (limits.go), identify/identify-album
// enforced the same 50 through their own now-deleted private cap constant
// using this exact wording, while inventory/raw-tags (via decodeSelection)
// enforced a separately-defined maxSelectionPaths behind a
// differently-worded combined empty-or-too-many message ("paths must
// contain between 1 and 50 entries") — same limit, different text. A
// request over the cap must now read identically everywhere.
func TestCapAppliesUniformly(t *testing.T) {
	r, lib := newCapHandler(t)
	paths := make([]string, 51)
	for i := range paths {
		paths[i] = "album/" + strconv.Itoa(i) + ".flac"
	}
	body, err := json.Marshal(map[string]any{"library_id": lib.ID, "paths": paths})
	if err != nil {
		t.Fatal(err)
	}

	for _, route := range []string{
		"/metadata/identify",
		"/metadata/identify-album",
		"/metadata/tracks/raw-tags",
		"/metadata/pictures/inventory",
	} {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest("POST", route, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400: %s", w.Code, w.Body.String())
			}
			var got struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Error != "too many paths in one request" {
				t.Fatalf("error = %q, want the shared errTooManyPaths message", got.Error)
			}
		})
	}
}
