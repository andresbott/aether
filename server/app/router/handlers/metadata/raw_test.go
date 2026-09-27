package metadata_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	metaHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	"github.com/andresbott/aether/app/router/handlers/problems"
	"github.com/andresbott/aether/internal/metadataedit"
	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/gorilla/mux"
)

func newRawHandler(t *testing.T, root string, read func(string) (map[string][]string, error)) (*mux.Router, scanfolder.Folder) {
	t.Helper()
	return newRawHandlerUnsupported(t, root, read, func(string) ([]string, error) {
		return []string{}, nil
	})
}

func newRawHandlerUnsupported(
	t *testing.T,
	root string,
	read func(string) (map[string][]string, error),
	readUnsupported func(string) ([]string, error),
) (*mux.Router, scanfolder.Folder) {
	t.Helper()
	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Main", Path: root, FollowSymlinks: true}})
	if err != nil {
		t.Fatal(err)
	}
	folder, _ := set.ByName("Main")
	h := &metaHandler.TagsHandler{
		Folders: set, Reader: nullReader{}, RawTagReader: read, UnsupportedReader: readUnsupported,
		Problems: problems.New(false),
	}
	r := mux.NewRouter()
	h.Routes(r)
	return r, folder
}

// postRaw POSTs a raw-tags selection request: scan_folder + paths[] travel in
// the JSON body (never the URL), the transport this endpoint moved to so a
// large multi-disc selection can never overflow a header buffer.
func postRaw(t *testing.T, r *mux.Router, scanFolder string, paths ...string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"scan_folder": scanFolder, "paths": paths})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/metadata/tracks/raw-tags", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRawTags_ReturnsFullTagMap(t *testing.T) {
	fixture := map[string][]string{
		"TITLE":                 {"Song"},
		"REPLAYGAIN_TRACK_GAIN": {"-3.10 dB"},
		"CUSTOM":                {"a", "b"},
	}
	r, folder := newRawHandler(t, t.TempDir(), func(string) (map[string][]string, error) {
		return fixture, nil
	})
	w := postRaw(t, r, folder.Name, "song.mp3")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Results []struct {
			Path  string              `json:"path"`
			Tags  map[string][]string `json:"tags"`
			Error string              `json:"error"`
		} `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Results) != 1 || body.Results[0].Path != "song.mp3" {
		t.Fatalf("unexpected results: %s", w.Body.String())
	}
	if got := body.Results[0].Tags["CUSTOM"]; len(got) != 2 || got[0] != "a" {
		t.Fatalf("unexpected tags: %s", w.Body.String())
	}
}

// The raw editor locks the keys the structured editor owns; the response
// carries that list so the client never keeps a copy that can drift.
func TestRawTags_ListsManagedKeys(t *testing.T) {
	r, folder := newRawHandler(t, t.TempDir(), func(string) (map[string][]string, error) {
		return map[string][]string{}, nil
	})
	w := postRaw(t, r, folder.Name, "song.mp3")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		ManagedKeys []string `json:"managed_keys"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(body.ManagedKeys, metadataedit.ManagedTagKeys()) {
		t.Fatalf("managed_keys = %v, want %v", body.ManagedKeys, metadataedit.ManagedTagKeys())
	}
}

func TestRawTags_IncludesUnsupportedFrames(t *testing.T) {
	r, folder := newRawHandlerUnsupported(t, t.TempDir(),
		func(string) (map[string][]string, error) {
			return map[string][]string{"TITLE": {"Song"}}, nil
		},
		func(string) ([]string, error) {
			return []string{"PRIV/com.example.junk", "GEOB", "UNKNOWN/XXXX"}, nil
		},
	)
	w := postRaw(t, r, folder.Name, "song.mp3")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Results []struct {
			Unsupported []string `json:"unsupported"`
		} `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Results) != 1 || len(body.Results[0].Unsupported) != 3 {
		t.Fatalf("expected 3 unsupported descriptors, got %s", w.Body.String())
	}
	if body.Results[0].Unsupported[0] != "PRIV/com.example.junk" {
		t.Fatalf("unexpected descriptors: %v", body.Results[0].Unsupported)
	}
}

func TestRawTags_FiltersCoverArtDescriptors(t *testing.T) {
	// Embedded cover art (APIC/covr/WM/Picture/APE Cover Art) must not be
	// listed as deletable hidden frames.
	r, folder := newRawHandlerUnsupported(t, t.TempDir(),
		func(string) (map[string][]string, error) {
			return map[string][]string{}, nil
		},
		func(string) ([]string, error) {
			return []string{"APIC", "covr", "WM/Picture", "Cover Art (Front)", "PRIV/junk"}, nil
		},
	)
	w := postRaw(t, r, folder.Name, "song.mp3")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body struct {
		Results []struct {
			Unsupported []string `json:"unsupported"`
		} `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	got := body.Results[0].Unsupported
	if len(got) != 1 || got[0] != "PRIV/junk" {
		t.Fatalf("expected only PRIV/junk to survive the cover filter, got %v", got)
	}
}

func TestUpdateTracks_RejectsCoverDescriptorRemoval(t *testing.T) {
	r, folder := newRawHandler(t, t.TempDir(), nil)
	body := `{"scan_folder": "` + folder.Name + `", "paths": ["a.mp3"], "fields": {"remove_unsupported": ["APIC"]}}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for cover descriptor, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRawTags_UnsupportedReadFailureDegrades(t *testing.T) {
	// A hidden-frame read error must not fail the row: tags still return,
	// unsupported comes back empty.
	r, folder := newRawHandlerUnsupported(t, t.TempDir(),
		func(string) (map[string][]string, error) {
			return map[string][]string{"TITLE": {"Song"}}, nil
		},
		func(string) ([]string, error) {
			return nil, errors.New("boom")
		},
	)
	w := postRaw(t, r, folder.Name, "song.mp3")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body struct {
		Results []struct {
			Tags        map[string][]string `json:"tags"`
			Unsupported []string            `json:"unsupported"`
			Error       string              `json:"error"`
		} `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	res := body.Results[0]
	if res.Error != "" || len(res.Tags["TITLE"]) != 1 || res.Unsupported == nil || len(res.Unsupported) != 0 {
		t.Fatalf("expected degraded row with tags and empty unsupported, got %s", w.Body.String())
	}
}

func TestRawTags_PerPathErrors(t *testing.T) {
	r, folder := newRawHandler(t, t.TempDir(), func(string) (map[string][]string, error) {
		return nil, errors.New("boom")
	})
	w := postRaw(t, r, folder.Name, "song.mp3", "../outside.mp3")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body struct {
		Results []struct {
			Error string `json:"error"`
		} `json:"results"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if len(body.Results) != 2 || body.Results[0].Error == "" || body.Results[1].Error == "" {
		t.Fatalf("expected per-path errors, got %s", w.Body.String())
	}
}

func TestRawTags_Validation(t *testing.T) {
	r, folder := newRawHandler(t, t.TempDir(), nil)
	// Empty paths[] is well-formed but invalid input: 422, not 400.
	if w := postRaw(t, r, folder.Name); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for missing paths, got %d", w.Code)
	}
	// An omitted scan_folder is a missing required field: 400, not a lookup
	// failure — unlike the numeric-id era, where an absent id decoded to 0 and
	// answered 404 ("no such library"). A name has no such accident.
	req := httptest.NewRequest("POST", "/metadata/tracks/raw-tags", strings.NewReader(`{"paths":["a.mp3"]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a missing scan_folder, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateTracks_RejectsManagedRawKey(t *testing.T) {
	r, folder := newRawHandler(t, t.TempDir(), nil)
	body := `{"scan_folder": "` + folder.Name + `", "paths": ["a.mp3"], "fields": {"raw_tags": {"MUSICBRAINZ_TRACKID": ["x"]}}}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for managed raw key, got %d: %s", w.Code, w.Body.String())
	}
}
