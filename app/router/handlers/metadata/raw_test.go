package metadata_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	metaHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

func newRawHandler(t *testing.T, libRoot string, read func(string) (map[string][]string, error)) (*mux.Router, *model.Library) {
	t.Helper()
	return newRawHandlerUnsupported(t, libRoot, read, func(string) ([]string, error) {
		return []string{}, nil
	})
}

func newRawHandlerUnsupported(
	t *testing.T,
	libRoot string,
	read func(string) (map[string][]string, error),
	readUnsupported func(string) ([]string, error),
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
	h := &metaHandler.Handler{Store: s, Reader: nullReader{}, RawTagReader: read, UnsupportedReader: readUnsupported}
	r := mux.NewRouter()
	h.Routes(r)
	return r, lib
}

func getRaw(t *testing.T, r *mux.Router, libID uint, paths ...string) *httptest.ResponseRecorder {
	t.Helper()
	url := "/metadata/tracks/raw?library_id=" + strconv.FormatUint(uint64(libID), 10)
	for _, p := range paths {
		url += "&paths=" + p
	}
	req := httptest.NewRequest("GET", url, nil)
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
	r, lib := newRawHandler(t, t.TempDir(), func(string) (map[string][]string, error) {
		return fixture, nil
	})
	w := getRaw(t, r, lib.ID, "song.mp3")
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

func TestRawTags_IncludesUnsupportedFrames(t *testing.T) {
	r, lib := newRawHandlerUnsupported(t, t.TempDir(),
		func(string) (map[string][]string, error) {
			return map[string][]string{"TITLE": {"Song"}}, nil
		},
		func(string) ([]string, error) {
			return []string{"PRIV/com.example.junk", "GEOB", "UNKNOWN/XXXX"}, nil
		},
	)
	w := getRaw(t, r, lib.ID, "song.mp3")
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
	r, lib := newRawHandlerUnsupported(t, t.TempDir(),
		func(string) (map[string][]string, error) {
			return map[string][]string{}, nil
		},
		func(string) ([]string, error) {
			return []string{"APIC", "covr", "WM/Picture", "Cover Art (Front)", "PRIV/junk"}, nil
		},
	)
	w := getRaw(t, r, lib.ID, "song.mp3")
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
	r, lib := newRawHandler(t, t.TempDir(), nil)
	body := `{"library_id": ` + strconv.FormatUint(uint64(lib.ID), 10) +
		`, "paths": ["a.mp3"], "fields": {"remove_unsupported": ["APIC"]}}`
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
	r, lib := newRawHandlerUnsupported(t, t.TempDir(),
		func(string) (map[string][]string, error) {
			return map[string][]string{"TITLE": {"Song"}}, nil
		},
		func(string) ([]string, error) {
			return nil, errors.New("boom")
		},
	)
	w := getRaw(t, r, lib.ID, "song.mp3")
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
	r, lib := newRawHandler(t, t.TempDir(), func(string) (map[string][]string, error) {
		return nil, errors.New("boom")
	})
	w := getRaw(t, r, lib.ID, "song.mp3", "../outside.mp3")
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
	r, lib := newRawHandler(t, t.TempDir(), nil)
	if w := getRaw(t, r, lib.ID); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing paths, got %d", w.Code)
	}
	req := httptest.NewRequest("GET", "/metadata/tracks/raw?paths=a.mp3", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing library, got %d", w.Code)
	}
}

func TestUpdateTracks_RejectsManagedRawKey(t *testing.T) {
	r, lib := newRawHandler(t, t.TempDir(), nil)
	body := `{"library_id": ` + strconv.FormatUint(uint64(lib.ID), 10) +
		`, "paths": ["a.mp3"], "fields": {"raw_tags": {"MUSICBRAINZ_TRACKID": ["x"]}}}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for managed raw key, got %d: %s", w.Code, w.Body.String())
	}
}
