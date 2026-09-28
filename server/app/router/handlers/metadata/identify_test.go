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

	metaHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	"github.com/andresbott/aether/app/router/handlers/problems"
	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/libs/acoustid"
	"github.com/go-bumbu/http/problemjson"
	"github.com/gorilla/mux"
)

type fakeIdentifier struct {
	recs []acoustid.Recording
	err  error
}

func (f fakeIdentifier) IdentifyFile(context.Context, string) ([]acoustid.Recording, error) {
	return f.recs, f.err
}

func newIdentifyHandler(t *testing.T, root string, ident metaHandler.IdentifyService) (*mux.Router, scanfolder.Folder) {
	t.Helper()
	return newIdentifyHandlerWithReason(t, root, ident, "")
}

func newIdentifyHandlerWithReason(
	t *testing.T, root string, ident metaHandler.IdentifyService, reason string,
) (*mux.Router, scanfolder.Folder) {
	t.Helper()
	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Main", Path: root, FollowSymlinks: true}})
	if err != nil {
		t.Fatal(err)
	}
	folder, _ := set.ByName("Main")
	h := &metaHandler.IdentifyHandler{
		Folders:                   set,
		Reader:                    nullReader{},
		Identifier:                ident,
		IdentifyUnavailableReason: reason,
		Problems:                  problems.New(false),
	}
	r := mux.NewRouter()
	h.Routes(r)
	return r, folder
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
	r, folder := newIdentifyHandlerWithReason(t, t.TempDir(), nil, reason)
	w := postIdentify(t, r, map[string]any{"scan_folder": folder.Name, "paths": []string{"a.mp3"}})
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	var body problemjson.Details
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Detail != reason || problemjson.Slug(body.Type) != "identify_unavailable" {
		t.Fatalf("unexpected error body: %s", w.Body.String())
	}
}

func TestIdentify_UnavailableWithoutService(t *testing.T) {
	r, folder := newIdentifyHandler(t, t.TempDir(), nil)
	w := postIdentify(t, r, map[string]any{"scan_folder": folder.Name, "paths": []string{"a.mp3"}})
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
	r, folder := newIdentifyHandler(t, root, ident)

	w := postIdentify(t, r, map[string]any{"scan_folder": folder.Name, "paths": []string{"song.mp3"}})
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
	r, folder := newIdentifyHandler(t, t.TempDir(), fakeIdentifier{})
	w := postIdentify(t, r, map[string]any{"scan_folder": folder.Name, "paths": []string{"../outside.mp3"}})
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
	r, folder := newIdentifyHandler(t, t.TempDir(), fakeIdentifier{})

	// An empty selection is well-formed but invalid: a 422 itemising /paths,
	// the same as the picture endpoints (decodeSelection) — not a 400.
	w := postIdentify(t, r, map[string]any{"scan_folder": folder.Name, "paths": []string{}})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for empty paths, got %d", w.Code)
	}
	var empty problemjson.ValidationDetails
	if err := json.Unmarshal(w.Body.Bytes(), &empty); err != nil {
		t.Fatal(err)
	}
	if len(empty.Errors) == 0 || empty.Errors[0].Pointer != "/paths" {
		t.Fatalf("expected a /paths field error, got %+v", empty.Errors)
	}

	// Over-cap paths[] is likewise a 422 itemising /paths — the same shared
	// bound (checkPaths) as the empty case above.
	tooMany := make([]string, 51)
	for i := range tooMany {
		tooMany[i] = "a.mp3"
	}
	w = postIdentify(t, r, map[string]any{"scan_folder": folder.Name, "paths": tooMany})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for too many paths, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	var validation problemjson.ValidationDetails
	if err := json.Unmarshal(w.Body.Bytes(), &validation); err != nil {
		t.Fatal(err)
	}
	if len(validation.Errors) == 0 || validation.Errors[0].Pointer != "/paths" {
		t.Fatalf("expected a /paths field error, got %+v", validation.Errors)
	}
}
