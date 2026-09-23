package libraries_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/andresbott/aether/app/router/handlers/libraries"
	"github.com/andresbott/aether/app/router/handlers/problems"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"github.com/go-bumbu/http/problemjson"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// newTestHandler builds a handler backed by an in-memory DB and a scan-folder
// set of two folders, Music and Books, so filter tests have real configured
// names to validate scan_folder values against.
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
	folders, err := scanfolder.NewSet([]scanfolder.Folder{
		{Name: "Music", Path: t.TempDir()},
		{Name: "Books", Path: t.TempDir()},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := &libraries.Handler{Store: s, Folders: folders, Problems: problems.New(false)}
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

// TestCreateLibraryStoresTheNameTrimmed: ValidateName judges the TRIMMED name,
// so the trimmed one is what gets stored. Storing the raw value would let
// "  Padded  " and "Padded" be two libraries — two music folders a /rest client
// cannot tell apart — and would ship the padding to every one of them.
func TestCreateLibraryStoresTheNameTrimmed(t *testing.T) {
	_, _, r := newTestHandler(t)
	post := func(name string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/libraries", strings.NewReader(`{"name":"`+name+`"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	w := post("  Padded  ")
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["name"] != "Padded" {
		t.Fatalf("expected name=Padded, got %v", got["name"])
	}

	// The unique index is on the stored name, so the padded one must not have
	// left room for a second library that reads identically.
	if w := post("Padded"); w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for the same name unpadded, got %d, body=%s", w.Code, w.Body.String())
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

// send issues one JSON request against the handler's router.
func send(t *testing.T, r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// libraryViews is the part of a library response the views tests read.
type libraryViews struct {
	ID                  uint                `json:"id"`
	Views               []model.LibraryView `json:"views"`
	DefaultView         model.LibraryView   `json:"default_view"`
	HideFromArtistIndex bool                `json:"hide_from_artist_index"`
	SplitViews          bool                `json:"split_views"`
}

// decodeViews fails unless w answered want, then decodes its body.
func decodeViews(t *testing.T, w *httptest.ResponseRecorder, want int) libraryViews {
	t.Helper()
	if w.Code != want {
		t.Fatalf("expected %d, got %d, body=%s", want, w.Code, w.Body.String())
	}
	var got libraryViews
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

// A library created with just a name browses like the whole catalog: every
// view, opening on Discover, its artists in the main index.
func TestCreateLibraryDefaultsToEveryView(t *testing.T) {
	_, _, r := newTestHandler(t)
	got := decodeViews(t, send(t, r, "POST", "/libraries", `{"name":"Main"}`), http.StatusCreated)
	if !slices.Equal(got.Views, model.LibraryViews()) || got.DefaultView != model.ViewDiscover {
		t.Fatalf("got views %v opening on %q, want every view opening on discover", got.Views, got.DefaultView)
	}
	if got.HideFromArtistIndex {
		t.Fatal("expected hide_from_artist_index=false by default")
	}
	if got.SplitViews {
		t.Fatal("expected split_views=false by default")
	}
}

// Views are stored in display order, each once, whatever order the request
// lists them in; without a default_view the library opens on the first.
func TestCreateLibraryNormalizesViews(t *testing.T) {
	_, _, r := newTestHandler(t)
	for _, tc := range []struct {
		name, body  string
		wantDefault model.LibraryView
	}{
		{"default named", `{"name":"Classical","views":["releases","artists","releases"],"default_view":"releases"}`, model.ViewReleases},
		{"default omitted", `{"name":"Jazz","views":["releases","artists"]}`, model.ViewArtists},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeViews(t, send(t, r, "POST", "/libraries", tc.body), http.StatusCreated)
			want := []model.LibraryView{model.ViewArtists, model.ViewReleases}
			if !slices.Equal(got.Views, want) || got.DefaultView != tc.wantDefault {
				t.Fatalf("got views %v opening on %q, want %v opening on %q", got.Views, got.DefaultView, want, tc.wantDefault)
			}
		})
	}
}

func TestCreateLibraryRejectsBadViews(t *testing.T) {
	_, _, r := newTestHandler(t)
	for _, tc := range []struct {
		name, body   string
		wantPointers []string
	}{
		{"no view", `{"name":"X","views":[]}`, []string{"/views"}},
		{"unknown views", `{"name":"X","views":["albums","artists","songs"]}`, []string{"/views/0", "/views/2"}},
		{"default not among the views", `{"name":"X","views":["releases"],"default_view":"discover"}`, []string{"/default_view"}},
		{"unknown default", `{"name":"X","default_view":"albums"}`, []string{"/default_view"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := send(t, r, "POST", "/libraries", tc.body)
			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("expected 422, got %d, body=%s", w.Code, w.Body.String())
			}
			var problem problemjson.ValidationDetails
			if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
				t.Fatal(err)
			}
			pointers := make([]string, 0, len(problem.Errors))
			for _, e := range problem.Errors {
				pointers = append(pointers, e.Pointer)
			}
			if !slices.Equal(pointers, tc.wantPointers) {
				t.Fatalf("error pointers = %v, want %v", pointers, tc.wantPointers)
			}
		})
	}
}

func TestUpdateLibraryViews(t *testing.T) {
	_, s, r := newTestHandler(t)
	lib := &model.Library{Name: "A"} // every view, by the column default
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	got := decodeViews(t, send(t, r, "PUT", "/libraries/"+itoa(lib.ID), `{"name":"A","views":["releases"]}`), http.StatusOK)
	if !slices.Equal(got.Views, []model.LibraryView{model.ViewReleases}) || got.DefaultView != model.ViewReleases {
		t.Fatalf("got views %v opening on %q, want [releases] opening on releases", got.Views, got.DefaultView)
	}
}

// An update that omits views keeps the stored ones, and its default_view is
// judged against them.
func TestUpdateLibraryWithoutViewsKeepsThem(t *testing.T) {
	_, s, r := newTestHandler(t)
	kept := []model.LibraryView{model.ViewArtists, model.ViewReleases}
	lib := &model.Library{Name: "A", Views: kept, DefaultView: model.ViewReleases}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	path := "/libraries/" + itoa(lib.ID)

	got := decodeViews(t, send(t, r, "PUT", path, `{"name":"A","default_view":"releases"}`), http.StatusOK)
	if !slices.Equal(got.Views, kept) || got.DefaultView != model.ViewReleases {
		t.Fatalf("got views %v opening on %q, want %v opening on releases", got.Views, got.DefaultView, kept)
	}
	// "" opens on the first view, as on create.
	got = decodeViews(t, send(t, r, "PUT", path, `{"name":"A"}`), http.StatusOK)
	if !slices.Equal(got.Views, kept) || got.DefaultView != model.ViewArtists {
		t.Fatalf("got views %v opening on %q, want %v opening on artists", got.Views, got.DefaultView, kept)
	}
	if w := send(t, r, "PUT", path, `{"name":"A","default_view":"discover"}`); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a default outside the kept views: expected 422, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestCreateLibraryHideFromArtistIndexRoundTrip(t *testing.T) {
	_, _, r := newTestHandler(t)
	// A hidden library needs at least one filter
	// (TestHideFromArtistIndexNeedsAFilter); carrying one here keeps this test
	// proving what it says — the round trip — rather than tripping over that
	// unrelated rule.
	body := `{"name":"Main","hide_from_artist_index":true,"filters":[{"field":"scan_folder","values":["Music"]}]}`
	if got := decodeViews(t, send(t, r, "POST", "/libraries", body), http.StatusCreated); !got.HideFromArtistIndex {
		t.Fatal("expected hide_from_artist_index=true in response")
	}
}

func TestUpdateLibraryHideFromArtistIndexOmittedPreservesCurrent(t *testing.T) {
	_, s, r := newTestHandler(t)
	lib := &model.Library{Name: "Main", HideFromArtistIndex: true, Filters: []model.LibraryFilter{
		{Field: model.FilterScanFolder, Values: []string{"Music"}},
	}}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	// A write that MENTIONS filters replaces them wholesale
	// (TestUpdateLibraryReplacesFilters), so this hidden library's PUT must keep
	// carrying one — the point of this test is that omitting
	// hide_from_artist_index preserves the CURRENT value, not the unrelated
	// has-a-filter rule.
	body := `{"name":"Updated","filters":[{"field":"scan_folder","values":["Music"]}]}`
	if got := decodeViews(t, send(t, r, "PUT", "/libraries/"+itoa(lib.ID), body), http.StatusOK); !got.HideFromArtistIndex {
		t.Fatal("expected hide_from_artist_index=true (preserved on an omitted key)")
	}
}

// split_views round-trips, and an update that omits it keeps the stored layout.
func TestLibrarySplitViewsRoundTrip(t *testing.T) {
	_, _, r := newTestHandler(t)
	got := decodeViews(t, send(t, r, "POST", "/libraries", `{"name":"Main","split_views":true}`), http.StatusCreated)
	if !got.SplitViews {
		t.Fatal("expected split_views=true in the create response")
	}
	path := "/libraries/" + itoa(got.ID)
	if got = decodeViews(t, send(t, r, "PUT", path, `{"name":"Main"}`), http.StatusOK); !got.SplitViews {
		t.Fatal("expected split_views=true (preserved on an omitted key)")
	}
	if got = decodeViews(t, send(t, r, "PUT", path, `{"name":"Main","split_views":false}`), http.StatusOK); got.SplitViews {
		t.Fatal("expected split_views=false after an explicit false")
	}
}

// assertNoWarningsKey fails if body carries a top-level "warnings" key.
// Warnings is emitted with omitempty, so decoding into a Go slice cannot
// distinguish "absent" from "present but empty" — only a check on the raw
// JSON object can.
func assertNoWarningsKey(t *testing.T, body []byte) {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	if v, ok := raw["warnings"]; ok {
		t.Fatalf(`expected no "warnings" key, got %s`, v)
	}
}

func TestCreateLibraryRoundTripsNormalizedFilters(t *testing.T) {
	_, _, r := newTestHandler(t)
	body := `{"name":"Main","filters":[{"field":"scan_folder","values":[" Music "]},{"field":"format","values":["FLAC"]}]}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
	}
	want := []model.LibraryFilter{
		{Field: model.FilterScanFolder, Values: []string{"Music"}},
		{Field: model.FilterFormat, Values: []string{"flac"}},
	}
	var created struct {
		ID      uint                  `json:"id"`
		Filters []model.LibraryFilter `json:"filters"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(created.Filters, want) {
		t.Fatalf("create response filters = %+v, want %+v", created.Filters, want)
	}
	assertNoWarningsKey(t, w.Body.Bytes())

	req = httptest.NewRequest("GET", "/libraries/"+itoa(created.ID), nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Filters []model.LibraryFilter `json:"filters"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Filters, want) {
		t.Fatalf("get response filters = %+v, want %+v", got.Filters, want)
	}
	assertNoWarningsKey(t, w.Body.Bytes())
}

func TestCreateLibraryWithoutFiltersIsValid(t *testing.T) {
	_, _, r := newTestHandler(t)
	body := `{"name":"Main"}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	filtersRaw, ok := raw["filters"]
	if !ok {
		t.Fatalf(`expected a "filters" key, body=%s`, w.Body.String())
	}
	// Compared as raw JSON text, not decoded into a Go slice: unmarshaling
	// either "null" or "[]" into a []T gives an empty slice in Go, so only
	// the wire text can prove this is really an array and not null.
	if string(filtersRaw) != "[]" {
		t.Fatalf(`expected filters to be the JSON array "[]", got %s`, filtersRaw)
	}
}

func TestCreateLibraryReportsEveryFilterProblem(t *testing.T) {
	_, _, r := newTestHandler(t)
	body := `{"name":"Main","filters":[` +
		`{"field":"bogus","values":["x"]},` +
		`{"field":"scan_folder","values":["Nope"]},` +
		`{"field":"path","values":["relative/dir"]}` +
		`]}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d, body=%s", w.Code, w.Body.String())
	}
	var problem problemjson.ValidationDetails
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	wantPointers := []string{"/filters/0/field", "/filters/1/values/0", "/filters/2/values/0"}
	if len(problem.Errors) != len(wantPointers) {
		t.Fatalf("expected %d errors, got %d: %+v", len(wantPointers), len(problem.Errors), problem.Errors)
	}
	for i, want := range wantPointers {
		if problem.Errors[i].Pointer != want {
			t.Errorf("errors[%d].pointer = %q, want %q", i, problem.Errors[i].Pointer, want)
		}
	}
}

func TestHideFromArtistIndexNeedsAFilter(t *testing.T) {
	_, _, r := newTestHandler(t)

	// No filters at all: refused at /hide_from_artist_index.
	body := `{"name":"Main","hide_from_artist_index":true}`
	req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d, body=%s", w.Code, w.Body.String())
	}
	var problem problemjson.ValidationDetails
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if len(problem.Errors) == 0 || problem.Errors[0].Pointer != "/hide_from_artist_index" {
		t.Fatalf("expected a /hide_from_artist_index field error, got %+v", problem.Errors)
	}

	// One filter: accepted.
	body = `{"name":"Main","hide_from_artist_index":true,"filters":[{"field":"scan_folder","values":["Music"]}]}`
	req = httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
	}
	var created struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	// A PUT that explicitly clears the filters of that hidden library is
	// refused: hide_from_artist_index is omitted here, so its EFFECTIVE value
	// stays the stored true, and a hidden library with no filters would hide
	// every artist.
	body = `{"name":"Main","filters":[]}`
	req = httptest.NewRequest("PUT", "/libraries/"+itoa(created.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d, body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if len(problem.Errors) == 0 || problem.Errors[0].Pointer != "/hide_from_artist_index" {
		t.Fatalf("expected a /hide_from_artist_index field error, got %+v", problem.Errors)
	}

	// The same PUT without a "filters" key keeps the stored filter, and the
	// rule is satisfied by what the library still selects.
	body = `{"name":"Main"}`
	req = httptest.NewRequest("PUT", "/libraries/"+itoa(created.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	// Kept filters are judged, never waved through: hiding a library that has
	// no filters is refused even though the request does not mention any.
	body = `{"name":"Open"}`
	req = httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	body = `{"name":"Open","hide_from_artist_index":true}`
	req = httptest.NewRequest("PUT", "/libraries/"+itoa(created.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d, body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if len(problem.Errors) == 0 || problem.Errors[0].Pointer != "/hide_from_artist_index" {
		t.Fatalf("expected a /hide_from_artist_index field error, got %+v", problem.Errors)
	}
}

func TestUpdateLibraryReplacesFilters(t *testing.T) {
	_, s, r := newTestHandler(t)
	lib := &model.Library{Name: "Main", Filters: []model.LibraryFilter{
		{Field: model.FilterScanFolder, Values: []string{"Music"}},
	}}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}

	body := `{"name":"Main","filters":[{"field":"scan_folder","values":["Books"]}]}`
	req := httptest.NewRequest("PUT", "/libraries/"+itoa(lib.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Filters []model.LibraryFilter `json:"filters"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := []model.LibraryFilter{{Field: model.FilterScanFolder, Values: []string{"Books"}}}
	if !reflect.DeepEqual(got.Filters, want) {
		t.Fatalf("filters = %+v, want %+v", got.Filters, want)
	}

	// An explicit empty array is the way to clear them: the key is mentioned,
	// so the request really does ask for no filters (omitting it keeps them —
	// TestUpdateLibraryWithoutAFiltersKeyKeepsThem).
	body = `{"name":"Main","filters":[]}`
	req = httptest.NewRequest("PUT", "/libraries/"+itoa(lib.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Filters) != 0 {
		t.Fatalf("expected filters cleared, got %+v", got.Filters)
	}
}

// TestUpdateLibraryWithoutAFiltersKeyKeepsThem pins the one asymmetry of the
// write DTO: an update that never mentions "filters" leaves the stored ones
// alone. Omission is the single path in this feature that could fail OPEN —
// a PUT shaped like a plain rename silently widening a curated library to the
// whole catalog — so it is pinned on the response, on a following GET, and on
// track_count, which is what actually proves the stored scope did not change.
func TestUpdateLibraryWithoutAFiltersKeyKeepsThem(t *testing.T) {
	_, s, r := newTestHandler(t)
	db := s.DB()
	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "x"}
	if err := db.Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	for _, track := range []model.Track{
		{AlbumID: album.ID, ScanFolder: "Music", Suffix: "flac", Filename: "1.flac", FilePath: "/music/1.flac"},
		{AlbumID: album.ID, ScanFolder: "Music", Suffix: "mp3", Filename: "2.mp3", FilePath: "/music/2.mp3"},
		{AlbumID: album.ID, ScanFolder: "Books", Suffix: "flac", Filename: "3.flac", FilePath: "/books/3.flac"},
	} {
		if err := db.Create(&track).Error; err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest("POST", "/libraries",
		strings.NewReader(`{"name":"Curated","filters":[{"field":"scan_folder","values":["Music"]},{"field":"format","values":["flac"]}]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
	}
	var created struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	want := []model.LibraryFilter{
		{Field: model.FilterScanFolder, Values: []string{"Music"}},
		{Field: model.FilterFormat, Values: []string{"flac"}},
	}

	type libraryBody struct {
		Filters    []model.LibraryFilter `json:"filters"`
		TrackCount int64                 `json:"track_count"`
	}
	do := func(t *testing.T, method, body string) libraryBody {
		t.Helper()
		var rdr io.Reader
		if body != "" {
			rdr = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, "/libraries/"+itoa(created.ID), rdr)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d, body=%s", method, w.Code, w.Body.String())
		}
		var got libraryBody
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got
	}

	if got := do(t, "GET", ""); got.TrackCount != 1 {
		t.Fatalf("seeded track_count = %d, want 1 (one flac in Music)", got.TrackCount)
	}

	for _, tc := range []struct{ name, body string }{
		{"no filters key", `{"name":"Renamed","default_view":"artists","icon":"heart"}`},
		{"explicit null", `{"name":"Renamed","default_view":"artists","icon":"heart","filters":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := do(t, "PUT", tc.body)
			if !reflect.DeepEqual(got.Filters, want) {
				t.Fatalf("update response filters = %+v, want the stored %+v", got.Filters, want)
			}
			if got.TrackCount != 1 {
				t.Fatalf("update response track_count = %d, want 1 — the scope must not have widened", got.TrackCount)
			}
			got = do(t, "GET", "")
			if !reflect.DeepEqual(got.Filters, want) {
				t.Fatalf("stored filters = %+v, want %+v", got.Filters, want)
			}
			if got.TrackCount != 1 {
				t.Fatalf("stored track_count = %d, want 1", got.TrackCount)
			}
		})
	}
}

// TestRenamingALibraryWithADanglingFilterWorks is why kept filters are not
// re-validated: a library whose scan folder left the config still has to be
// renameable, and it keeps reporting the warning that says so. Sending those
// same values explicitly is a different request — it names an unconfigured
// folder — and is still refused.
func TestRenamingALibraryWithADanglingFilterWorks(t *testing.T) {
	_, s, r := newTestHandler(t)
	lib := &model.Library{Name: "Ghost", Filters: []model.LibraryFilter{
		{Field: model.FilterScanFolder, Values: []string{"Gone"}},
	}}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("PUT", "/libraries/"+itoa(lib.ID), strings.NewReader(`{"name":"Ghost Renamed"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Name     string                `json:"name"`
		Filters  []model.LibraryFilter `json:"filters"`
		Warnings []struct {
			Pointer string `json:"pointer"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "Ghost Renamed" {
		t.Fatalf("name = %q, want the new one", got.Name)
	}
	wantFilters := []model.LibraryFilter{{Field: model.FilterScanFolder, Values: []string{"Gone"}}}
	if !reflect.DeepEqual(got.Filters, wantFilters) {
		t.Fatalf("filters = %+v, want the stored %+v", got.Filters, wantFilters)
	}
	if len(got.Warnings) != 1 || got.Warnings[0].Pointer != "/filters/0/values/0" {
		t.Fatalf("expected the dangling-folder warning to survive the rename, got %+v", got.Warnings)
	}

	req = httptest.NewRequest("PUT", "/libraries/"+itoa(lib.ID),
		strings.NewReader(`{"name":"Ghost Renamed","filters":[{"field":"scan_folder","values":["Gone"]}]}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d, body=%s", w.Code, w.Body.String())
	}
	var problem problemjson.ValidationDetails
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if len(problem.Errors) != 1 || problem.Errors[0].Pointer != "/filters/0/values/0" {
		t.Fatalf("expected one error at /filters/0/values/0, got %+v", problem.Errors)
	}
}

// TestLibraryWarnsAboutAScanFolderThatIsGone stores its library directly
// through the store, bypassing libraryfilter.Validate (which would refuse
// "Gone" outright) — this is what a library looks like after its scan folder
// was removed from the config AFTER the library was saved.
func TestLibraryWarnsAboutAScanFolderThatIsGone(t *testing.T) {
	_, s, r := newTestHandler(t)
	lib := &model.Library{Name: "Ghost", Filters: []model.LibraryFilter{
		{Field: model.FilterScanFolder, Values: []string{"Gone"}},
	}}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/libraries/"+itoa(lib.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Warnings []struct {
			Pointer string `json:"pointer"`
			Detail  string `json:"detail"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Warnings) != 1 || got.Warnings[0].Pointer != "/filters/0/values/0" {
		t.Fatalf("expected one warning at /filters/0/values/0, got %+v", got.Warnings)
	}
	if got.Warnings[0].Detail == "" {
		t.Fatal("expected a non-empty warning detail")
	}

	req = httptest.NewRequest("GET", "/libraries", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var list struct {
		Libraries []struct {
			Warnings []struct {
				Pointer string `json:"pointer"`
			} `json:"warnings"`
		} `json:"libraries"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Libraries) != 1 || len(list.Libraries[0].Warnings) != 1 || list.Libraries[0].Warnings[0].Pointer != "/filters/0/values/0" {
		t.Fatalf("expected the list to report the same warning, got %+v", list.Libraries)
	}
}

func TestTrackCountFollowsTheFilters(t *testing.T) {
	_, s, r := newTestHandler(t)
	db := s.DB()
	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "x"}
	db.Create(&album)
	db.Create(&model.Track{AlbumID: album.ID, ScanFolder: "Music", Suffix: "flac", Filename: "1.flac", FilePath: "/music/1.flac"})
	db.Create(&model.Track{AlbumID: album.ID, ScanFolder: "Music", Suffix: "mp3", Filename: "2.mp3", FilePath: "/music/2.mp3"})
	db.Create(&model.Track{AlbumID: album.ID, ScanFolder: "Books", Suffix: "mp3", Filename: "3.mp3", FilePath: "/books/3.mp3"})

	scoped := &model.Library{Name: "Music FLAC", Filters: []model.LibraryFilter{
		{Field: model.FilterScanFolder, Values: []string{"Music"}},
		{Field: model.FilterFormat, Values: []string{"flac"}},
	}}
	if err := s.CreateLibrary(scoped); err != nil {
		t.Fatal(err)
	}
	whole := &model.Library{Name: "Everything"}
	if err := s.CreateLibrary(whole); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/libraries/"+itoa(scoped.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var got struct {
		TrackCount int64 `json:"track_count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.TrackCount != 1 {
		t.Fatalf("expected track_count 1 for the scoped library, got %d", got.TrackCount)
	}

	req = httptest.NewRequest("GET", "/libraries/"+itoa(whole.ID), nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.TrackCount != 3 {
		t.Fatalf("expected track_count 3 for the filterless library, got %d", got.TrackCount)
	}
}

// TestCreateLibraryWithoutANameIs400 pins the 400-vs-422 mapping at the HTTP
// layer (today only proven at the ValidateName unit level, validate_test.go):
// a missing required field is a malformed request, not a well-formed-but-invalid
// one, whether the key is absent or present-but-blank.
func TestCreateLibraryWithoutANameIs400(t *testing.T) {
	_, _, r := newTestHandler(t)
	for _, body := range []string{`{}`, `{"name":"   "}`} {
		req := httptest.NewRequest("POST", "/libraries", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body=%s: expected 400, got %d, resp=%s", body, w.Code, w.Body.String())
		}
		var problem problemjson.Details
		if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
			t.Fatal(err)
		}
		if slug := problemjson.Slug(problem.Type); slug != "validation_error" {
			t.Fatalf("body=%s: expected slug validation_error, got %q", body, slug)
		}
	}
}
