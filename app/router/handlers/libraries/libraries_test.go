package libraries_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/andresbott/aether/app/router/handlers/libraries"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

func newTestHandler(t *testing.T) (*libraries.Handler, *store.Store, *mux.Router) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := store.New(db)
	h := &libraries.Handler{Store: s}
	r := mux.NewRouter()
	h.Routes(r)
	return h, s, r
}

func TestListLibrariesEmpty(t *testing.T) {
	_, _, r := newTestHandler(t)
	req := httptest.NewRequest("GET", "/libraries", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body struct {
		Libraries []map[string]any `json:"libraries"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if len(body.Libraries) != 0 {
		t.Fatalf("expected empty list, got %d", len(body.Libraries))
	}
}

func TestGetLibraryNotFound(t *testing.T) {
	_, _, r := newTestHandler(t)
	req := httptest.NewRequest("GET", "/libraries/999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetLibraryOK(t *testing.T) {
	_, s, r := newTestHandler(t)
	lib := &model.Library{Name: "Main", Path: "/srv/music", FollowSymlinks: true}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/libraries/"+itoa(lib.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["name"] != "Main" {
		t.Fatalf("expected name=Main, got %v", got["name"])
	}
}

func itoa(n uint) string {
	return strconv.FormatUint(uint64(n), 10)
}

func TestCreateLibraryOK(t *testing.T) {
	_, _, r := newTestHandler(t)
	dir := t.TempDir()
	body := `{"name":"Main","path":"` + dir + `","exclude_patterns":["^\\..*"],"follow_symlinks":true}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestCreateLibraryBadPath(t *testing.T) {
	_, _, r := newTestHandler(t)
	body := `{"name":"X","path":"/nonexistent-aether-test-xyz"}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateLibraryBadRegex(t *testing.T) {
	_, _, r := newTestHandler(t)
	dir := t.TempDir()
	body := `{"name":"X","path":"` + dir + `","exclude_patterns":["["]}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateLibraryDuplicate(t *testing.T) {
	_, s, r := newTestHandler(t)
	dir := t.TempDir()
	if err := s.CreateLibrary(&model.Library{Name: "X", Path: dir}); err != nil {
		t.Fatal(err)
	}
	body := `{"name":"X","path":"` + dir + `"}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestUpdateLibraryRename(t *testing.T) {
	_, s, r := newTestHandler(t)
	dir := t.TempDir()
	lib := &model.Library{Name: "A", Path: dir}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	body := `{"name":"B","path":"` + dir + `"}`
	req := httptest.NewRequest("PUT", "/libraries/"+itoa(lib.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["name"] != "B" {
		t.Fatalf("expected name=B, got %v", got["name"])
	}
	if got["path_changed"] == true {
		t.Fatalf("path_changed should be false on rename")
	}
}

func TestUpdateLibraryPathChangeWipesTracks(t *testing.T) {
	_, s, r := newTestHandler(t)
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	lib := &model.Library{Name: "A", Path: dir1}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	db := s.DB()
	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "x"}
	db.Create(&album)
	db.Create(&model.Track{AlbumID: album.ID, LibraryID: lib.ID, Filename: "1.mp3", FilePath: dir1 + "/1.mp3"})

	body := `{"name":"A","path":"` + dir2 + `"}`
	req := httptest.NewRequest("PUT", "/libraries/"+itoa(lib.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["path_changed"] != true {
		t.Fatalf("expected path_changed=true")
	}
	var trackCount int64
	db.Model(&model.Track{}).Where("library_id = ?", lib.ID).Count(&trackCount)
	if trackCount != 0 {
		t.Fatalf("expected tracks wiped, got %d", trackCount)
	}
}

func TestDeleteLibrary(t *testing.T) {
	_, s, r := newTestHandler(t)
	dir := t.TempDir()
	lib := &model.Library{Name: "A", Path: dir}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("DELETE", "/libraries/"+itoa(lib.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	var count int64
	s.DB().Model(&model.Library{}).Count(&count)
	if count != 0 {
		t.Fatalf("expected library deleted, %d remaining", count)
	}
}

func TestDeleteLibraryNotFound(t *testing.T) {
	_, _, r := newTestHandler(t)
	req := httptest.NewRequest("DELETE", "/libraries/999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// gorm.Delete on non-existent ID is a no-op (no error), so we accept 204 here.
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestCreateLibraryWithDefaultView(t *testing.T) {
	_, _, r := newTestHandler(t)
	dir := t.TempDir()
	body := `{"name":"Classical","path":"` + dir + `","default_view":"artists"}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["default_view"] != "artists" {
		t.Fatalf("expected default_view=artists, got %v", got["default_view"])
	}
}

func TestCreateLibraryDefaultsToAlbums(t *testing.T) {
	_, _, r := newTestHandler(t)
	dir := t.TempDir()
	body := `{"name":"Main","path":"` + dir + `"}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["default_view"] != "albums" {
		t.Fatalf("expected default_view=albums, got %v", got["default_view"])
	}
}

func TestCreateLibraryRejectsBadDefaultView(t *testing.T) {
	_, _, r := newTestHandler(t)
	dir := t.TempDir()
	body := `{"name":"X","path":"` + dir + `","default_view":"songs"}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestUpdateLibraryDefaultView(t *testing.T) {
	_, s, r := newTestHandler(t)
	dir := t.TempDir()
	lib := &model.Library{Name: "A", Path: dir, DefaultView: "albums"}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	body := `{"name":"A","path":"` + dir + `","default_view":"artists"}`
	req := httptest.NewRequest("PUT", "/libraries/"+itoa(lib.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["default_view"] != "artists" {
		t.Fatalf("expected default_view=artists, got %v", got["default_view"])
	}
}

func TestCreateLibraryShowArtistsRoundTrip(t *testing.T) {
	_, _, r := newTestHandler(t)
	dir := t.TempDir()
	body := `{"name":"Main","path":"` + dir + `","follow_symlinks":true,"show_artists":false}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	v, ok := got["show_artists"].(bool)
	if !ok || v {
		t.Fatalf("expected show_artists=false in response, got %v", got["show_artists"])
	}
}

func TestCreateLibraryShowArtistsOmitted(t *testing.T) {
	_, _, r := newTestHandler(t)
	dir := t.TempDir()
	body := `{"name":"Main","path":"` + dir + `","follow_symlinks":true}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	v, ok := got["show_artists"].(bool)
	if !ok || !v {
		t.Fatalf("expected show_artists=true (default) in response, got %v", got["show_artists"])
	}
}
