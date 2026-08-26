package metadata_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/andresbott/aether/app/router/handlers/httperr"
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
// request over the cap must now read identically everywhere. updateTracks
// (PUT /metadata/tracks) reached parity later — it used to enforce no cap at
// all — via its own inline check in metadata.go rather than decodeSelection,
// since it decodes a distinct updateRequest shape carrying fields alongside
// paths.
func TestCapAppliesUniformly(t *testing.T) {
	r, lib := newCapHandler(t)
	paths := make([]string, 51)
	for i := range paths {
		paths[i] = "album/" + strconv.Itoa(i) + ".flac"
	}
	// updateTracks additionally requires a non-empty fields (a request that
	// writes nothing is a 400, ahead of the cap check); the other four endpoints
	// ignore the extra key, so one shared body still reaches every endpoint's
	// paths[] cap.
	body, err := json.Marshal(map[string]any{
		"library_id": lib.ID,
		"paths":      paths,
		"fields":     map[string]any{"title": "x"},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ method, route string }{
		{http.MethodPost, "/metadata/identify"},
		{http.MethodPost, "/metadata/identify-album"},
		{http.MethodPost, "/metadata/tracks/raw-tags"},
		{http.MethodPost, "/metadata/pictures/inventory"},
		{http.MethodPut, "/metadata/tracks"},
	} {
		t.Run(tc.route, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.route, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			// Over-cap paths[] is well-formed but invalid input: 422, not 400.
			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status %d, want 422: %s", w.Code, w.Body.String())
			}
			if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
				t.Fatalf("Content-Type = %q, want application/problem+json", ct)
			}
			var got httperr.ValidationProblem
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Detail != "too many paths in one request" {
				t.Fatalf("detail = %q, want the shared errTooManyPaths message", got.Detail)
			}
			if len(got.Errors) == 0 || got.Errors[0].Pointer != "/paths" {
				t.Fatalf("expected a /paths field error, got %+v", got.Errors)
			}
		})
	}
}
