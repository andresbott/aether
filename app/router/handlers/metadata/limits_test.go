package metadata_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	metaHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	"github.com/andresbott/aether/app/router/handlers/problems"
	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/go-bumbu/http/problemjson"
	"github.com/gorilla/mux"
)

// newCapHandler wires all three metadata handlers (identify, tags, images) onto
// one router — with a fake Identifier/AlbumIdentifier so identify/identify-album
// reach their paths[] validation instead of short-circuiting on 503 — for
// exercising the shared maxSelectionPaths cap across every paths[]-accepting
// endpoint, which now span all three handlers.
func newCapHandler(t *testing.T) (*mux.Router, scanfolder.Folder) {
	t.Helper()
	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Main", Path: t.TempDir(), FollowSymlinks: true}})
	if err != nil {
		t.Fatal(err)
	}
	folder, _ := set.ByName("Main")
	r := mux.NewRouter()
	(&metaHandler.IdentifyHandler{
		Folders:         set,
		Reader:          nullReader{},
		Identifier:      fakeIdentifier{},
		AlbumIdentifier: &fakeAlbumIdentifier{},
		Problems:        problems.New(false),
	}).Routes(r)
	(&metaHandler.TagsHandler{Folders: set, Reader: nullReader{}, Problems: problems.New(false)}).Routes(r)
	(&metaHandler.ImagesHandler{Folders: set, Reader: nullReader{}, Problems: problems.New(false)}).Routes(r)
	return r, folder
}

// TestCapAppliesUniformly confirms maxSelectionPaths (50) and its "too many
// paths in one request" message are shared by every paths[]-accepting
// endpoint. Before this unification (limits.go), identify/identify-album
// enforced the same 50 through their own now-deleted private cap constant
// using this exact wording, while inventory/raw-tags (via decodeSelection)
// enforced a separately-defined maxSelectionPaths behind a
// differently-worded combined empty-or-too-many message ("paths must
// contain between 1 and 50 entries") — same limit, different text. A
// request over the cap must now read identically everywhere. All five
// paths[]-accepting endpoints (including updateTracks, PUT /metadata/tracks,
// which decodes a distinct updateRequest shape carrying fields alongside
// paths) share one bound via checkPaths/resolveSelection, so the cap — and the
// empty-selection 422 — can no longer drift between endpoints.
func TestCapAppliesUniformly(t *testing.T) {
	r, folder := newCapHandler(t)
	paths := make([]string, 51)
	for i := range paths {
		paths[i] = "album/" + strconv.Itoa(i) + ".flac"
	}
	// updateTracks additionally requires a non-empty fields (a request that
	// writes nothing is a 400, ahead of the cap check); the other four endpoints
	// ignore the extra key, so one shared body still reaches every endpoint's
	// paths[] cap.
	body, err := json.Marshal(map[string]any{
		"scan_folder": folder.Name,
		"paths":       paths,
		"fields":      map[string]any{"title": "x"},
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
			var got problemjson.ValidationDetails
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
