package libraries_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/andresbott/aether/app/router/handlers/libraries"
	"github.com/andresbott/aether/app/router/handlers/problems"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"github.com/go-bumbu/http/problemjson"
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
	h := &libraries.Handler{Store: s, Problems: problems.New(false)}
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
	lib := &model.Library{Name: "Main"}
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
	body := `{"name":"Main"}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestCreateLibraryDuplicate(t *testing.T) {
	_, s, r := newTestHandler(t)
	if err := s.CreateLibrary(&model.Library{Name: "X"}); err != nil {
		t.Fatal(err)
	}
	body := `{"name":"X"}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestCreateLibraryIcon(t *testing.T) {
	_, _, r := newTestHandler(t)
	body := `{"name":"X","icon":"headphones"}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["icon"] != "headphones" {
		t.Fatalf("expected icon=headphones, got %v", got["icon"])
	}
}

func TestCreateLibraryIconDefaultsToFolder(t *testing.T) {
	_, _, r := newTestHandler(t)
	body := `{"name":"X"}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["icon"] != "folder" {
		t.Fatalf("expected icon=folder, got %v", got["icon"])
	}
}

func TestCreateLibraryBadIcon(t *testing.T) {
	_, _, r := newTestHandler(t)
	body := `{"name":"X","icon":"not a valid icon!"}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
	var problem problemjson.ValidationDetails
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if len(problem.Errors) == 0 || problem.Errors[0].Pointer != "/icon" {
		t.Fatalf("expected an /icon field error, got %+v", problem.Errors)
	}
}

func TestUpdateLibraryIcon(t *testing.T) {
	_, s, r := newTestHandler(t)
	lib := &model.Library{Name: "A", Icon: "folder"}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	body := `{"name":"A","icon":"heart"}`
	req := httptest.NewRequest("PUT", "/libraries/"+itoa(lib.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["icon"] != "heart" {
		t.Fatalf("expected icon=heart, got %v", got["icon"])
	}
}

func TestUpdateLibraryRename(t *testing.T) {
	_, s, r := newTestHandler(t)
	lib := &model.Library{Name: "A"}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	body := `{"name":"B"}`
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
}

// A library is a view: deleting it must leave every track (and, by extension,
// anything derived from it) untouched — only the row that grouped them goes.
func TestDeleteLibrary(t *testing.T) {
	_, s, r := newTestHandler(t)
	lib := &model.Library{Name: "A"}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	db := s.DB()
	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "x"}
	db.Create(&album)
	db.Create(&model.Track{AlbumID: album.ID, ScanFolder: lib.Name, Filename: "1.mp3", FilePath: "/a/1.mp3"})

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
	var trackCount int64
	db.Model(&model.Track{}).Count(&trackCount)
	if trackCount != 1 {
		t.Fatalf("expected the track to survive, got %d", trackCount)
	}
}

func TestDeleteLibraryNotFound(t *testing.T) {
	_, _, r := newTestHandler(t)
	req := httptest.NewRequest("DELETE", "/libraries/999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// The handler loads the library first: the store's delete does not error
	// on a missing row, so without this a missing ID would 204 instead of 404.
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestCreateLibraryWithDefaultView(t *testing.T) {
	_, _, r := newTestHandler(t)
	body := `{"name":"Classical","default_view":"artists"}`
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
	body := `{"name":"Main"}`
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
	body := `{"name":"X","default_view":"songs"}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestUpdateLibraryDefaultView(t *testing.T) {
	_, s, r := newTestHandler(t)
	lib := &model.Library{Name: "A", DefaultView: "albums"}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	body := `{"name":"A","default_view":"artists"}`
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
	body := `{"name":"Main","show_artists":false}`
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
	body := `{"name":"Main"}`
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

func TestUpdateLibraryShowArtistsOmittedPreservesCurrent(t *testing.T) {
	_, s, r := newTestHandler(t)
	lib := &model.Library{Name: "Main", HideArtists: true}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	body := `{"name":"Updated"}`
	req := httptest.NewRequest("PUT", "/libraries/"+itoa(lib.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	v, ok := got["show_artists"].(bool)
	if !ok || v {
		t.Fatalf("expected show_artists=false (hidden state preserved on omitted key), got %v", got["show_artists"])
	}
}
