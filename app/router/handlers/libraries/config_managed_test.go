package libraries_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andresbott/aether/app/router/handlers/httperr"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

// seedConfigLibrary creates a library owned by the config file.
func seedConfigLibrary(t *testing.T, s *store.Store, name, path string) *model.Library {
	t.Helper()
	lib := &model.Library{Name: name, Path: path, Source: model.SourceConfig}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	return lib
}

func TestUpdateConfigManagedLibraryRefused(t *testing.T) {
	_, s, r := newTestHandler(t)
	dir := t.TempDir()
	lib := seedConfigLibrary(t, s, "Rock", dir)

	body := `{"name":"Renamed","path":"` + dir + `"}`
	req := httptest.NewRequest("PUT", "/libraries/"+itoa(lib.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d, body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	var problem httperr.Problem
	_ = json.Unmarshal(w.Body.Bytes(), &problem)
	if got := httperr.Slug(problem.Type); got != "config_managed" {
		t.Fatalf("expected code config_managed, got %q", got)
	}
	// The row must be untouched.
	got, err := s.GetLibrary(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Rock" {
		t.Fatalf("library was modified: %+v", got)
	}
}

func TestDeleteConfigManagedLibraryRefused(t *testing.T) {
	_, s, r := newTestHandler(t)
	lib := seedConfigLibrary(t, s, "Rock", t.TempDir())

	req := httptest.NewRequest("DELETE", "/libraries/"+itoa(lib.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d, body=%s", w.Code, w.Body.String())
	}
	if _, err := s.GetLibrary(lib.ID); err != nil {
		t.Fatalf("library should still exist: %v", err)
	}
}

func TestCreateLibraryShadowingConfigNameRefused(t *testing.T) {
	_, s, r := newTestHandler(t)
	seedConfigLibrary(t, s, "Rock", t.TempDir())

	body := `{"name":"Rock","path":"` + t.TempDir() + `"}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "config") {
		t.Fatalf("error should mention the config file, got %s", w.Body.String())
	}
}

func TestCreateLibraryShadowingConfigPathRefused(t *testing.T) {
	_, s, r := newTestHandler(t)
	dir := t.TempDir()
	seedConfigLibrary(t, s, "Rock", dir)

	body := `{"name":"Something Else","path":"` + dir + `"}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d, body=%s", w.Code, w.Body.String())
	}
}

// A config library is still listed — the two sources are additive — and carries
// its source so the UI can lock the row.
func TestListExposesSource(t *testing.T) {
	_, s, r := newTestHandler(t)
	seedConfigLibrary(t, s, "Rock", t.TempDir())
	if err := s.CreateLibrary(&model.Library{
		Name: "Jazz", Path: t.TempDir(), Source: model.SourceDB,
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/libraries", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body struct {
		Libraries []struct {
			Name   string `json:"name"`
			Source string `json:"source"`
		} `json:"libraries"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Libraries) != 2 {
		t.Fatalf("expected both libraries listed, got %d", len(body.Libraries))
	}
	sources := map[string]string{}
	for _, l := range body.Libraries {
		sources[l.Name] = l.Source
	}
	if sources["Rock"] != model.SourceConfig {
		t.Fatalf("expected Rock source=config, got %q", sources["Rock"])
	}
	if sources["Jazz"] != model.SourceDB {
		t.Fatalf("expected Jazz source=db, got %q", sources["Jazz"])
	}
}

// An explicit follow_symlinks:false must persist; a GORM column default would
// silently turn it back into true.
func TestCreateLibraryHonoursExplicitFollowSymlinksFalse(t *testing.T) {
	_, s, r := newTestHandler(t)
	dir := t.TempDir()
	body := `{"name":"Main","path":"` + dir + `","follow_symlinks":false}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
	}
	lib, err := s.FindLibraryByPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if lib.FollowSymlinks {
		t.Fatal("follow_symlinks:false was not persisted")
	}
}

// An omitted follow_symlinks keeps the historical default of true.
func TestCreateLibraryDefaultsFollowSymlinksTrue(t *testing.T) {
	_, s, r := newTestHandler(t)
	dir := t.TempDir()
	body := `{"name":"Main","path":"` + dir + `"}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
	}
	lib, err := s.FindLibraryByPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !lib.FollowSymlinks {
		t.Fatal("expected follow_symlinks to default to true")
	}
}
