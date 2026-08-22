package metadata_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/app/router/handlers/httperr"
	metaHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/libs/acoustid"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type fakeIdentifier struct {
	recs []acoustid.Recording
	err  error
}

func (f fakeIdentifier) IdentifyFile(context.Context, string) ([]acoustid.Recording, error) {
	return f.recs, f.err
}

func newIdentifyHandler(t *testing.T, libRoot string, ident metaHandler.IdentifyService) (*mux.Router, *model.Library) {
	t.Helper()
	return newIdentifyHandlerWithReason(t, libRoot, ident, "")
}

func newIdentifyHandlerWithReason(
	t *testing.T, libRoot string, ident metaHandler.IdentifyService, reason string,
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
		Store:                     s,
		Reader:                    nullReader{},
		Identifier:                ident,
		IdentifyUnavailableReason: reason,
	}
	r := mux.NewRouter()
	h.Routes(r)
	return r, lib
}

func postIdentify(t *testing.T, r *mux.Router, body any) *httptest.ResponseRecorder {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/metadata/identify", bytes.NewReader(buf))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func getCapabilities(t *testing.T, r *mux.Router) (bool, string, int) {
	t.Helper()
	req := httptest.NewRequest("GET", "/metadata/capabilities", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var body struct {
		Identify bool   `json:"identify"`
		Reason   string `json:"identify_unavailable_reason"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return body.Identify, body.Reason, w.Code
}

func TestCapabilities_ReportsIdentify(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ident metaHandler.IdentifyService
		want  bool
	}{
		{"disabled", nil, false},
		{"enabled", fakeIdentifier{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := newIdentifyHandler(t, t.TempDir(), tc.ident)
			got, _, code := getCapabilities(t, r)
			if code != http.StatusOK {
				t.Fatalf("expected 200, got %d", code)
			}
			if got != tc.want {
				t.Fatalf("expected identify=%v, got %v", tc.want, got)
			}
		})
	}
}

// The UI greys out Identify and shows this reason, so it must reach the client
// verbatim — and never leak when the feature is actually available.
func TestCapabilities_ReportsUnavailableReason(t *testing.T) {
	const reason = "fpcalc not found; install libchromaprint-tools"

	r, _ := newIdentifyHandlerWithReason(t, t.TempDir(), nil, reason)
	if _, got, _ := getCapabilities(t, r); got != reason {
		t.Fatalf("expected reason %q, got %q", reason, got)
	}

	// No reason configured: a generic explanation still reaches the UI.
	r, _ = newIdentifyHandlerWithReason(t, t.TempDir(), nil, "")
	if _, got, _ := getCapabilities(t, r); got == "" {
		t.Fatal("expected a fallback reason when none is configured")
	}

	// Enabled: no reason at all.
	r, _ = newIdentifyHandlerWithReason(t, t.TempDir(), fakeIdentifier{}, reason)
	if _, got, _ := getCapabilities(t, r); got != "" {
		t.Fatalf("expected no reason when identify is enabled, got %q", got)
	}
}

// The 503 body carries the same explanation, for clients that POST anyway.
func TestIdentify_UnavailableIncludesReason(t *testing.T) {
	const reason = "fpcalc not found; install libchromaprint-tools"
	r, lib := newIdentifyHandlerWithReason(t, t.TempDir(), nil, reason)
	w := postIdentify(t, r, map[string]any{"library_id": lib.ID, "paths": []string{"a.mp3"}})
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	var body httperr.Problem
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Detail != reason || httperr.Slug(body.Type) != "identify_unavailable" {
		t.Fatalf("unexpected error body: %s", w.Body.String())
	}
}

func TestIdentify_UnavailableWithoutService(t *testing.T) {
	r, lib := newIdentifyHandler(t, t.TempDir(), nil)
	w := postIdentify(t, r, map[string]any{"library_id": lib.ID, "paths": []string{"a.mp3"}})
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIdentify_ReturnsCandidatesPerPath(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "song.mp3"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ident := fakeIdentifier{recs: []acoustid.Recording{
		{
			Score: 0.95, MBID: "rec-uuid", Title: "Song",
			Artists: []acoustid.ArtistCredit{{MBID: "artist-uuid", Name: "Artist"}},
			Release: []acoustid.Release{{MBID: "rel-uuid", ReleaseGroupMBID: "rg-uuid", Title: "Album", Year: 2001}},
		},
	}}
	r, lib := newIdentifyHandler(t, root, ident)

	w := postIdentify(t, r, map[string]any{"library_id": lib.ID, "paths": []string{"song.mp3"}})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Results []struct {
			Path       string `json:"path"`
			Candidates []struct {
				Score         float64 `json:"score"`
				RecordingMBID string  `json:"recording_mbid"`
				Title         string  `json:"title"`
				Artists       []struct {
					Name string `json:"name"`
					MBID string `json:"mbid"`
				} `json:"artists"`
				Releases []struct {
					ReleaseMBID      string `json:"release_mbid"`
					ReleaseGroupMBID string `json:"release_group_mbid"`
					Album            string `json:"album"`
					Year             int    `json:"year"`
				} `json:"releases"`
			} `json:"candidates"`
			Error string `json:"error"`
		} `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Results) != 1 || body.Results[0].Path != "song.mp3" {
		t.Fatalf("unexpected results: %s", w.Body.String())
	}
	c := body.Results[0].Candidates
	if len(c) != 1 || c[0].RecordingMBID != "rec-uuid" || c[0].Title != "Song" || c[0].Score != 0.95 {
		t.Fatalf("unexpected candidates: %s", w.Body.String())
	}
	if len(c[0].Artists) != 1 || c[0].Artists[0].Name != "Artist" {
		t.Fatalf("unexpected artists: %s", w.Body.String())
	}
	if len(c[0].Releases) != 1 || c[0].Releases[0].Album != "Album" || c[0].Releases[0].Year != 2001 {
		t.Fatalf("unexpected releases: %s", w.Body.String())
	}
}

func TestIdentify_RejectsTraversalPerPath(t *testing.T) {
	r, lib := newIdentifyHandler(t, t.TempDir(), fakeIdentifier{})
	w := postIdentify(t, r, map[string]any{"library_id": lib.ID, "paths": []string{"../outside.mp3"}})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with per-path error, got %d", w.Code)
	}
	var body struct {
		Results []struct {
			Error string `json:"error"`
		} `json:"results"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if len(body.Results) != 1 || body.Results[0].Error == "" {
		t.Fatalf("expected per-path error, got %s", w.Body.String())
	}
}

func TestIdentify_ValidationErrors(t *testing.T) {
	r, lib := newIdentifyHandler(t, t.TempDir(), fakeIdentifier{})

	w := postIdentify(t, r, map[string]any{"library_id": lib.ID, "paths": []string{}})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty paths, got %d", w.Code)
	}

	// Over-cap paths[] is well-formed but invalid (422), unlike the missing-
	// input case above, which stays 400 — see decodeSelection's identical cap.
	tooMany := make([]string, 51)
	for i := range tooMany {
		tooMany[i] = "a.mp3"
	}
	w = postIdentify(t, r, map[string]any{"library_id": lib.ID, "paths": tooMany})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for too many paths, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	var validation httperr.ValidationProblem
	if err := json.Unmarshal(w.Body.Bytes(), &validation); err != nil {
		t.Fatal(err)
	}
	if len(validation.Errors) == 0 || validation.Errors[0].Pointer != "/paths" {
		t.Fatalf("expected a /paths field error, got %+v", validation.Errors)
	}
}
