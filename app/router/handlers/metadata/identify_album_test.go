package metadata_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	metaHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	"github.com/andresbott/aether/internal/albumidentify"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type fakeAlbumIdentifier struct {
	options []albumidentify.AlbumOption
	err     error
	// callHistory records every Resolve call's inputs, so tests can verify what
	// paths reached (or never reached) the resolver across multiple calls.
	callHistory [][]albumidentify.Input
}

func (f *fakeAlbumIdentifier) Resolve(
	_ context.Context, inputs []albumidentify.Input,
) ([]albumidentify.AlbumOption, error) {
	f.callHistory = append(f.callHistory, inputs)
	return f.options, f.err
}

func newAlbumIdentifyHandler(
	t *testing.T, libRoot string, svc metaHandler.AlbumIdentifyService,
) (*mux.Router, *model.Library) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := store.New(db)
	lib := &model.Library{Name: "Main", Path: libRoot, FollowSymlinks: true}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	h := &metaHandler.Handler{
		Store:           s,
		Reader:          nullReader{},
		Identifier:      fakeIdentifier{},
		AlbumIdentifier: svc,
	}
	r := mux.NewRouter()
	h.Routes(r)
	return r, lib
}

func postIdentifyAlbum(t *testing.T, r *mux.Router, body any) *httptest.ResponseRecorder {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/metadata/identify-album", bytes.NewReader(buf))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestIdentifyAlbum_UnavailableWithoutService(t *testing.T) {
	r, lib := newAlbumIdentifyHandler(t, t.TempDir(), nil)
	w := postIdentifyAlbum(t, r, map[string]any{
		"library_id": lib.ID, "paths": []string{"a.mp3", "b.mp3"},
	})
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIdentifyAlbum_ValidationErrors(t *testing.T) {
	r, lib := newAlbumIdentifyHandler(t, t.TempDir(), &fakeAlbumIdentifier{})

	if w := postIdentifyAlbum(t, r, map[string]any{
		"library_id": lib.ID, "paths": []string{},
	}); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty paths, got %d", w.Code)
	}

	// Album identification is meaningless for a single file.
	if w := postIdentifyAlbum(t, r, map[string]any{
		"library_id": lib.ID, "paths": []string{"only.mp3"},
	}); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a single path, got %d", w.Code)
	}

	if w := postIdentifyAlbum(t, r, map[string]any{
		"paths": []string{"a.mp3", "b.mp3"},
	}); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a missing library_id, got %d", w.Code)
	}

	tooMany := make([]string, 51)
	for i := range tooMany {
		tooMany[i] = "a.mp3"
	}
	if w := postIdentifyAlbum(t, r, map[string]any{
		"library_id": lib.ID, "paths": tooMany,
	}); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for too many paths, got %d", w.Code)
	}

	req := httptest.NewRequest("POST", "/metadata/identify-album", bytes.NewReader([]byte("{not json")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestIdentifyAlbum_UnknownLibrary(t *testing.T) {
	r, _ := newAlbumIdentifyHandler(t, t.TempDir(), &fakeAlbumIdentifier{})
	w := postIdentifyAlbum(t, r, map[string]any{
		"library_id": 4242, "paths": []string{"a.mp3", "b.mp3"},
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIdentifyAlbum_RejectsTraversalPerPath(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "song.mp3"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := &fakeAlbumIdentifier{}
	r, lib := newAlbumIdentifyHandler(t, root, svc)

	w := postIdentifyAlbum(t, r, map[string]any{
		"library_id": lib.ID, "paths": []string{"song.mp3", "../outside.mp3"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	// The resolver was called exactly once.
	if len(svc.callHistory) != 1 {
		t.Fatalf("expected 1 resolver call, got %d", len(svc.callHistory))
	}
	// The traversal path never reached the resolver: only song.mp3 did.
	if len(svc.callHistory[0]) != 1 || svc.callHistory[0][0].Path != "song.mp3" {
		t.Fatalf("unexpected inputs: %+v", svc.callHistory[0])
	}
	// Confirm ../outside.mp3 appears in NO recorded call.
	for _, call := range svc.callHistory {
		for _, input := range call {
			if input.Path == "../outside.mp3" {
				t.Fatalf("traversal path ../outside.mp3 leaked to resolver")
			}
		}
	}
	var body struct {
		Errors []struct {
			Path  string `json:"path"`
			Error string `json:"error"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if len(body.Errors) != 1 || body.Errors[0].Path != "../outside.mp3" || body.Errors[0].Error == "" {
		t.Fatalf("expected a per-path error, got %s", w.Body.String())
	}
}

func TestIdentifyAlbum_AllPathsRejected(t *testing.T) {
	root := t.TempDir()
	svc := &fakeAlbumIdentifier{}
	r, lib := newAlbumIdentifyHandler(t, root, svc)

	w := postIdentifyAlbum(t, r, map[string]any{
		"library_id": lib.ID, "paths": []string{"../a.mp3", "../b.mp3"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	// The resolver was never called at all.
	if len(svc.callHistory) != 0 {
		t.Fatalf("expected 0 resolver calls, got %d", len(svc.callHistory))
	}
	var body struct {
		Options []any `json:"options"`
		Errors  []struct {
			Path  string `json:"path"`
			Error string `json:"error"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	// options is an empty JSON array (not null).
	if body.Options == nil || len(body.Options) != 0 {
		t.Fatalf("expected empty options array, got %s", w.Body.String())
	}
	// Both paths appear in the errors array.
	if len(body.Errors) != 2 {
		t.Fatalf("expected 2 errors, got %d: %s", len(body.Errors), w.Body.String())
	}
	paths := map[string]bool{body.Errors[0].Path: true, body.Errors[1].Path: true}
	if !paths["../a.mp3"] || !paths["../b.mp3"] {
		t.Fatalf("expected both ../a.mp3 and ../b.mp3 in errors, got %s", w.Body.String())
	}
	if body.Errors[0].Error == "" || body.Errors[1].Error == "" {
		t.Fatalf("expected error messages, got %s", w.Body.String())
	}
}

// assertResolvedPathsAreValid verifies every input in the call has an absolute
// AbsPath that lies inside libRoot (the library the handler looked up).
func assertResolvedPathsAreValid(t *testing.T, libRoot string, inputs []albumidentify.Input) {
	t.Helper()
	for _, input := range inputs {
		if !filepath.IsAbs(input.AbsPath) {
			t.Fatalf("expected absolute path, got %q", input.AbsPath)
		}
		relPath, err := filepath.Rel(libRoot, input.AbsPath)
		if err != nil || filepath.IsAbs(relPath) || len(relPath) >= 3 && relPath[:3] == ".."+string(filepath.Separator) {
			t.Fatalf("path %q is not inside library root %q", input.AbsPath, libRoot)
		}
	}
}

func TestIdentifyAlbum_ReturnsRankedOptions(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"01.mp3", "02.mp3"} {
		if err := os.WriteFile(filepath.Join(root, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	svc := &fakeAlbumIdentifier{options: []albumidentify.AlbumOption{{
		ReleaseMBID: "rel-A", ReleaseGroupMBID: "rg-A", Album: "Album A", Year: 1991,
		Artists:      []albumidentify.Artist{{Name: "Artist", MBID: "art-1"}},
		TrackCount:   2, DiscCount: 1, Enriched: true, MatchedCount: 2, MeanScore: 0.9,
		Tracks: []albumidentify.Slot{
			{DiscNumber: 1, TrackNumber: 1, Title: "One", RecordingMBID: "rec-1", DurationSeconds: 180},
			{DiscNumber: 1, TrackNumber: 2, Title: "Two", RecordingMBID: "rec-2", DurationSeconds: 200},
		},
		Assignments: []albumidentify.Assignment{
			{Path: "01.mp3", Source: albumidentify.SourceFingerprint, Title: "One",
				RecordingMBID: "rec-1", DiscNumber: 1, TrackNumber: 1, Score: 0.9},
			{Path: "02.mp3", Source: albumidentify.SourceInferred, Title: "Two",
				RecordingMBID: "rec-2", DiscNumber: 1, TrackNumber: 2},
		},
	}}}
	r, lib := newAlbumIdentifyHandler(t, root, svc)

	w := postIdentifyAlbum(t, r, map[string]any{
		"library_id": lib.ID, "paths": []string{"01.mp3", "02.mp3"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Options []struct {
			ReleaseMBID string `json:"release_mbid"`
			Album       string `json:"album"`
			Year        int    `json:"year"`
			TrackCount  int    `json:"track_count"`
			Enriched    bool   `json:"enriched"`
			Assignments []struct {
				Path        string `json:"path"`
				Source      string `json:"source"`
				Title       string `json:"title"`
				TrackNumber int    `json:"track_number"`
			} `json:"assignments"`
			Tracks []struct {
				TrackNumber int    `json:"track_number"`
				Title       string `json:"title"`
			} `json:"tracks"`
		} `json:"options"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Options) != 1 {
		t.Fatalf("expected 1 option: %s", w.Body.String())
	}
	o := body.Options[0]
	if o.ReleaseMBID != "rel-A" || o.Album != "Album A" || o.Year != 1991 || !o.Enriched {
		t.Fatalf("unexpected option: %s", w.Body.String())
	}
	if len(o.Assignments) != 2 || o.Assignments[1].Source != "inferred" ||
		o.Assignments[1].TrackNumber != 2 {
		t.Fatalf("unexpected assignments: %s", w.Body.String())
	}
	if len(o.Tracks) != 2 || o.Tracks[0].Title != "One" {
		t.Fatalf("unexpected tracklist: %s", w.Body.String())
	}
	// The handler must pass the current tags down as ranking signals. Verify
	// the resolver was called exactly once with two inputs, and that each
	// AbsPath is absolute and inside the library root.
	if len(svc.callHistory) != 1 {
		t.Fatalf("expected 1 resolver call, got %d", len(svc.callHistory))
	}
	if len(svc.callHistory[0]) != 2 {
		t.Fatalf("expected 2 inputs, got %d", len(svc.callHistory[0]))
	}
	assertResolvedPathsAreValid(t, root, svc.callHistory[0])
}

func TestIdentifyAlbum_ResolverErrorIsBadGateway(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"01.mp3", "02.mp3"} {
		if err := os.WriteFile(filepath.Join(root, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	svc := &fakeAlbumIdentifier{err: errors.New("acoustid down")}
	r, lib := newAlbumIdentifyHandler(t, root, svc)

	w := postIdentifyAlbum(t, r, map[string]any{
		"library_id": lib.ID, "paths": []string{"01.mp3", "02.mp3"},
	})
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
}
