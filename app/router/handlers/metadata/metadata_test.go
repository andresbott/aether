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
	"strconv"
	"strings"
	"testing"

	metaHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/mux"
	_taglib "go.senan.xyz/taglib"
	"gorm.io/gorm"
)

type nullReader struct{}

func (nullReader) CanRead(string) bool { return false }
func (nullReader) Read(context.Context, string) (tags.Metadata, error) {
	return tags.Metadata{}, nil
}

// taggedReader reads audio by extension and returns a fixed album artist for
// every file — enough to drive the artist-folder eligibility check and the
// representative-track rescan in the artist-image tests.
type taggedReader struct{ albumArtist string }

func (taggedReader) CanRead(p string) bool {
	e := strings.ToLower(filepath.Ext(p))
	return e == ".flac" || e == ".mp3"
}
func (r taggedReader) Read(context.Context, string) (tags.Metadata, error) {
	return tags.Metadata{AlbumArtist: []string{r.albumArtist}}, nil
}

func newTestHandler(t *testing.T, libRoot string) (*store.Store, *mux.Router, *model.Library) {
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
	h := &metaHandler.Handler{Store: s, Reader: nullReader{}}
	r := mux.NewRouter()
	h.Routes(r)
	return s, r, lib
}

func TestFolders_ListsImmediateSubdirs(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Beatles"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, r, lib := newTestHandler(t, root)
	url := "/metadata/folders?library_id=" + strconv.FormatUint(uint64(lib.ID), 10) + "&path="
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Folders []map[string]any `json:"folders"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if len(body.Folders) != 1 || body.Folders[0]["name"] != "Beatles" {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestFolders_RejectsTraversal(t *testing.T) {
	root := t.TempDir()
	_, r, lib := newTestHandler(t, root)
	url := "/metadata/folders?library_id=" + strconv.FormatUint(uint64(lib.ID), 10) + "&path=../"
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFolders_UnknownLibrary404(t *testing.T) {
	_, r, _ := newTestHandler(t, t.TempDir())
	req := httptest.NewRequest("GET", "/metadata/folders?library_id=999&path=", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestFolders_SearchByQueryFindsDeepMatch(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Alexia dixon", "fire up"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Other", "thing"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, r, lib := newTestHandler(t, root)
	// A search query returns matching folders from anywhere in the library, not
	// just the immediate children the plain listing would return.
	url := "/metadata/folders?library_id=" + strconv.FormatUint(uint64(lib.ID), 10) + "&q=up"
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Folders []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"folders"`
		Truncated bool `json:"truncated"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Folders) != 1 || body.Folders[0].Path != "Alexia dixon/fire up" {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
	if body.Truncated {
		t.Fatalf("did not expect truncation for one match")
	}
}

type stubTagReader struct{}

func (stubTagReader) CanRead(p string) bool {
	return filepath.Ext(p) == ".mp3" || filepath.Ext(p) == ".flac"
}
func (stubTagReader) Read(_ context.Context, p string) (tags.Metadata, error) {
	return tags.Metadata{Title: filepath.Base(p), Artist: []string{"Stub"}, Album: "Alb", Year: 2020}, nil
}

func TestTracks_ListsFilesWithTags(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "alb"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "alb", "01.flac"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "alb", "02.mp3"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "alb", "readme.txt"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	_ = model.Migrate(db)
	s := store.New(db)
	lib := &model.Library{Name: "Main", Path: root}
	_ = s.CreateLibrary(lib)
	h := &metaHandler.Handler{Store: s, Reader: stubTagReader{}}
	r := mux.NewRouter()
	h.Routes(r)

	url := "/metadata/tracks?library_id=" + strconv.FormatUint(uint64(lib.ID), 10) + "&path=alb"
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Tracks []map[string]any `json:"tracks"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if len(body.Tracks) != 2 {
		t.Fatalf("expected 2 rows, got %d: %s", len(body.Tracks), w.Body.String())
	}
	if body.Tracks[0]["path"] != "alb/01.flac" {
		t.Fatalf("first row path unexpected: %v", body.Tracks[0])
	}
}

func TestTracks_RejectsTraversal(t *testing.T) {
	root := t.TempDir()
	_, r, lib := newTestHandler(t, root)
	url := "/metadata/tracks?library_id=" + strconv.FormatUint(uint64(lib.ID), 10) + "&path=../"
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func taglibWrite(path string, tm map[string][]string) error { return _taglib.WriteTags(path, tm, 0) }
func taglibReadTags(path string) (map[string][]string, error) {
	return _taglib.ReadTags(path)
}

func copyTestFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, in, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateTracks_PartialFailureCollected(t *testing.T) {
	root := t.TempDir()
	fx := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(fx); err != nil {
		t.Skipf("no fixture: %v", err)
	}
	dst := filepath.Join(root, "ok.flac")
	copyTestFile(t, fx, dst)

	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	_ = model.Migrate(db)
	s := store.New(db)
	lib := &model.Library{Name: "Main", Path: root}
	_ = s.CreateLibrary(lib)
	h := &metaHandler.Handler{Store: s, Reader: nullReader{}}
	r := mux.NewRouter()
	h.Routes(r)

	body := `{
		"library_id": ` + strconv.FormatUint(uint64(lib.ID), 10) + `,
		"paths": ["ok.flac", "missing.flac"],
		"fields": { "title": "New Title" }
	}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Results []struct {
			Path  string `json:"path"`
			OK    bool   `json:"ok"`
			Error string `json:"error,omitempty"`
		} `json:"results"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.Results))
	}
	gotByPath := map[string]bool{}
	for _, row := range resp.Results {
		gotByPath[row.Path] = row.OK
	}
	if !gotByPath["ok.flac"] {
		t.Fatalf("ok.flac should have succeeded: %+v", resp.Results)
	}
	if gotByPath["missing.flac"] {
		t.Fatalf("missing.flac should have failed: %+v", resp.Results)
	}
}

// Every row failing is still a processed batch, not a transport failure: the
// status stays 200 and each row carries its own error, exactly as rawTags
// does. Before this rule the handler flipped to 500 with the identical body,
// which made axios throw and lose the per-row detail in the SPA.
func TestUpdateTracks_AllRowsFailStill200(t *testing.T) {
	root := t.TempDir()
	_, r, lib := newTestHandler(t, root)

	body := `{
		"library_id": ` + strconv.FormatUint(uint64(lib.ID), 10) + `,
		"paths": ["missing-a.flac", "missing-b.flac"],
		"fields": { "title": "New Title" }
	}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 even when every row failed, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected application/json, got %q", ct)
	}
	var resp struct {
		Results []struct {
			Path  string `json:"path"`
			OK    bool   `json:"ok"`
			Error string `json:"error,omitempty"`
		} `json:"results"`
		Rescan *json.RawMessage `json:"rescan"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v: %s", err, w.Body.String())
	}
	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d: %s", len(resp.Results), w.Body.String())
	}
	for _, row := range resp.Results {
		if row.OK {
			t.Fatalf("%s should have failed: %+v", row.Path, resp.Results)
		}
		if row.Error == "" {
			t.Fatalf("%s failed without an error message: %+v", row.Path, resp.Results)
		}
	}
	// Nothing was written, so there is nothing to re-index and no rescan report.
	if resp.Rescan != nil {
		t.Fatalf("expected no rescan when no file was written, got %s", string(*resp.Rescan))
	}
}

func TestUpdateTracks_RejectsTraversalPerPath(t *testing.T) {
	root := t.TempDir()
	_, r, lib := newTestHandler(t, root)
	body := `{"library_id": ` + strconv.FormatUint(uint64(lib.ID), 10) + `, "paths": ["../escape.mp3"], "fields": {"title": "x"}}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateTracks_OnlyProvidedFieldsWritten(t *testing.T) {
	root := t.TempDir()
	fx := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(fx); err != nil {
		t.Skipf("no fixture: %v", err)
	}
	dst := filepath.Join(root, "a.flac")
	copyTestFile(t, fx, dst)
	_ = taglibWrite(dst, map[string][]string{"TITLE": {"Original"}, "ALBUM": {"Old"}})

	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	_ = model.Migrate(db)
	s := store.New(db)
	lib := &model.Library{Name: "Main", Path: root}
	_ = s.CreateLibrary(lib)
	h := &metaHandler.Handler{Store: s, Reader: nullReader{}}
	r := mux.NewRouter()
	h.Routes(r)

	body := `{
		"library_id": ` + strconv.FormatUint(uint64(lib.ID), 10) + `,
		"paths": ["a.flac"],
		"fields": { "album": "New" }
	}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	got, err := taglibReadTags(dst)
	if err != nil {
		t.Fatal(err)
	}
	if got["ALBUM"][0] != "New" {
		t.Fatalf("album should be 'New', got %v", got["ALBUM"])
	}
	if got["TITLE"][0] != "Original" {
		t.Fatalf("title should be preserved, got %v", got["TITLE"])
	}
}

func TestUpdateTracks_AlbumReleaseIDsWritten(t *testing.T) {
	root := t.TempDir()
	fx := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(fx); err != nil {
		t.Skipf("no fixture: %v", err)
	}
	dst := filepath.Join(root, "a.flac")
	copyTestFile(t, fx, dst)
	_ = taglibWrite(dst, map[string][]string{"ALBUM": {"Keep"}})

	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	_ = model.Migrate(db)
	s := store.New(db)
	lib := &model.Library{Name: "Main", Path: root}
	_ = s.CreateLibrary(lib)
	h := &metaHandler.Handler{Store: s, Reader: nullReader{}}
	r := mux.NewRouter()
	h.Routes(r)

	body := `{
		"library_id": ` + strconv.FormatUint(uint64(lib.ID), 10) + `,
		"paths": ["a.flac"],
		"fields": { "mb_release_id": "rel-uuid", "mb_release_group_id": "rg-uuid" }
	}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	got, err := taglibReadTags(dst)
	if err != nil {
		t.Fatal(err)
	}
	if got["MUSICBRAINZ_ALBUMID"][0] != "rel-uuid" {
		t.Fatalf("release id unexpected: %v", got["MUSICBRAINZ_ALBUMID"])
	}
	if got["MUSICBRAINZ_RELEASEGROUPID"][0] != "rg-uuid" {
		t.Fatalf("release-group id unexpected: %v", got["MUSICBRAINZ_RELEASEGROUPID"])
	}
	// Album name must be left intact when only IDs are sent.
	if got["ALBUM"][0] != "Keep" {
		t.Fatalf("album should be preserved, got %v", got["ALBUM"])
	}
}

func TestUpdateTracks_GenresAndTrackNumberWritten(t *testing.T) {
	root := t.TempDir()
	fx := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(fx); err != nil {
		t.Skipf("no fixture: %v", err)
	}
	dst := filepath.Join(root, "a.flac")
	copyTestFile(t, fx, dst)

	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	_ = model.Migrate(db)
	s := store.New(db)
	lib := &model.Library{Name: "Main", Path: root}
	_ = s.CreateLibrary(lib)
	h := &metaHandler.Handler{Store: s, Reader: nullReader{}}
	r := mux.NewRouter()
	h.Routes(r)

	body := `{
		"library_id": ` + strconv.FormatUint(uint64(lib.ID), 10) + `,
		"paths": ["a.flac"],
		"fields": { "genres": ["Rock", "Jazz"], "track_number": 7 }
	}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	got, err := taglibReadTags(dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(got["GENRE"]) != 2 || got["GENRE"][0] != "Rock" || got["GENRE"][1] != "Jazz" {
		t.Fatalf("genres unexpected: %v", got["GENRE"])
	}
	if got["TRACKNUMBER"][0] != "7" {
		t.Fatalf("track number unexpected: %v", got["TRACKNUMBER"])
	}
}

func TestUpdateTracks_ArtistMBID_AlignsPerTrack(t *testing.T) {
	root := t.TempDir()
	fx := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(fx); err != nil {
		t.Skipf("no fixture: %v", err)
	}
	dst1 := filepath.Join(root, "t1.flac")
	dst2 := filepath.Join(root, "t2.flac")
	copyTestFile(t, fx, dst1)
	copyTestFile(t, fx, dst2)
	if err := taglibWrite(dst1, map[string][]string{"ARTIST": {"Daft Punk"}}); err != nil {
		t.Fatal(err)
	}
	if err := taglibWrite(dst2, map[string][]string{"ARTIST": {"Daft Punk", "Pharrell"}}); err != nil {
		t.Fatal(err)
	}

	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	_ = model.Migrate(db)
	s := store.New(db)
	lib := &model.Library{Name: "Main", Path: root}
	_ = s.CreateLibrary(lib)
	h := &metaHandler.Handler{Store: s, Reader: tags.TaglibReader{}}
	r := mux.NewRouter()
	h.Routes(r)

	body := `{
		"library_id": ` + strconv.FormatUint(uint64(lib.ID), 10) + `,
		"paths": ["t1.flac", "t2.flac"],
		"fields": { "artist_mbids": {"Daft Punk": "id-dp", "Pharrell": "id-ph"} }
	}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	got1, err := taglibReadTags(dst1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got1["MUSICBRAINZ_ARTISTID"]) != 1 || got1["MUSICBRAINZ_ARTISTID"][0] != "id-dp" {
		t.Fatalf("t1 MUSICBRAINZ_ARTISTID unexpected: %v", got1["MUSICBRAINZ_ARTISTID"])
	}

	got2, err := taglibReadTags(dst2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got2["MUSICBRAINZ_ARTISTID"]) != 2 || got2["MUSICBRAINZ_ARTISTID"][0] != "id-dp" || got2["MUSICBRAINZ_ARTISTID"][1] != "id-ph" {
		t.Fatalf("t2 MUSICBRAINZ_ARTISTID unexpected: %v", got2["MUSICBRAINZ_ARTISTID"])
	}
}

func TestUpdateTracks_MalformedJSON(t *testing.T) {
	_, r, _ := newTestHandler(t, t.TempDir())
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// A single request may not both rename an artist field and set its MusicBrainz
// IDs: the MB-ID map is keyed by the current names, so writing new names in the
// same request would produce a positionally-misaligned tag. The handler rejects
// the whole request so a corrupt tag is never written; the user saves them
// separately.
func TestUpdateTracks_RejectsArtistRenameWithMBID(t *testing.T) {
	_, r, lib := newTestHandler(t, t.TempDir())
	body := `{
		"library_id": ` + strconv.FormatUint(uint64(lib.ID), 10) + `,
		"paths": ["a.flac"],
		"fields": { "artists": ["New Name"], "artist_mbids": {"Old Name": "056e4f3e-d505-4dad-8ec1-d04f521cbb56"} }
	}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateTracks_RejectsAlbumArtistRenameWithMBID(t *testing.T) {
	_, r, lib := newTestHandler(t, t.TempDir())
	body := `{
		"library_id": ` + strconv.FormatUint(uint64(lib.ID), 10) + `,
		"paths": ["a.flac"],
		"fields": { "album_artists": ["New"], "album_artist_mbids": {"Old": "056e4f3e-d505-4dad-8ec1-d04f521cbb56"} }
	}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// fakeRescanner records what the handler asked to re-index. By default it
// reports every path re-indexed; stats overrides that with a partial or
// erroring result.
type fakeRescanner struct {
	calls [][]string
	libs  []uint
	err   error
	// stats, when set, replaces the default "everything was processed" report.
	stats *scanner.ScanStats
}

func (f *fakeRescanner) RescanPaths(_ context.Context, libraryID uint, absPaths []string) (scanner.ScanStats, error) {
	f.calls = append(f.calls, absPaths)
	f.libs = append(f.libs, libraryID)
	if f.stats != nil {
		return *f.stats, f.err
	}
	return scanner.ScanStats{TracksProcessed: len(absPaths)}, f.err
}

// rescanTestHandler builds a handler over a real in-memory store whose library
// root is a temp dir holding one writable flac fixture.
func rescanTestHandler(t *testing.T, rs *fakeRescanner) (*mux.Router, *model.Library) {
	t.Helper()
	root := t.TempDir()
	fx := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(fx); err != nil {
		t.Skipf("no fixture: %v", err)
	}
	copyTestFile(t, fx, filepath.Join(root, "ok.flac"))

	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	_ = model.Migrate(db)
	s := store.New(db)
	lib := &model.Library{Name: "Main", Path: root}
	_ = s.CreateLibrary(lib)
	h := &metaHandler.Handler{Store: s, Reader: nullReader{}, Rescan: rs}
	r := mux.NewRouter()
	h.Routes(r)
	return r, lib
}

// rescanResponse is the slice of an update response the rescan assertions need.
type rescanResponse struct {
	Results []struct {
		OK bool `json:"ok"`
	} `json:"results"`
	Rescan *struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	} `json:"rescan"`
}

// putTitle saves a title to ok.flac and decodes the response, asserting the
// write itself succeeded with a 200.
func putTitle(t *testing.T, r *mux.Router, libID uint) rescanResponse {
	t.Helper()
	body := `{"library_id": ` + strconv.FormatUint(uint64(libID), 10) +
		`, "paths": ["ok.flac"], "fields": {"title": "T"}}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// The tags are already on disk; a failed re-index must not fail the write.
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp rescanResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || !resp.Results[0].OK {
		t.Fatalf("expected the write to succeed: %s", w.Body.String())
	}
	return resp
}

// wideReader stands in for the editor's tag reader, which is deliberately wider
// than the scanner's admission rules: it accepts extensions the scanner does not
// index (.oga, .mpc, ... per internal/tags/ffprobe.go) and knows nothing about
// the library's exclude patterns.
type wideReader struct{}

func (wideReader) CanRead(p string) bool {
	switch filepath.Ext(p) {
	case ".flac", ".mp3", ".oga":
		return true
	}
	return false
}

func (wideReader) Read(_ context.Context, p string) (tags.Metadata, error) {
	return tags.Metadata{
		Title:       filepath.Base(p),
		Artist:      []string{"A"},
		AlbumArtist: []string{"A"},
		Album:       "Alb",
	}, nil
}

// realRescanHandler wires a *real* scanner.Scanner into the handler, so the
// admission rules under test are the scanner's own rather than a fake's.
func realRescanHandler(t *testing.T, root string, excludes []string) (*mux.Router, *model.Library) {
	t.Helper()
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := store.New(db)
	excludeJSON := ""
	if len(excludes) > 0 {
		b, _ := json.Marshal(excludes)
		excludeJSON = string(b)
	}
	lib := &model.Library{Name: "Main", Path: root, ExcludePatterns: excludeJSON}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	h := &metaHandler.Handler{
		Store:  s,
		Reader: wideReader{},
		Rescan: scanner.New(scanner.Config{}, s, wideReader{}),
	}
	r := mux.NewRouter()
	h.Routes(r)
	return r, lib
}

// The regression this guards: RescanPaths deliberately skips paths the library
// does not cover, and the editor's file listing is wider than the scanner's
// admission on purpose (it ignores excludes and reads extra extensions). A save
// that hands over such paths is entirely correct, so it must report rescan
// ok:true — measuring the shortfall against the raw path count instead of
// against the admitted count warns the user about a save that worked.
func TestUpdateTracks_InadmissiblePathsDoNotFailTheRescan(t *testing.T) {
	fx := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(fx); err != nil {
		t.Skipf("no fixture: %v", err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Artist", "Live"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Artist", "Album"), 0o755); err != nil {
		t.Fatal(err)
	}
	// One good track, one under an excluded ancestor directory, one with an
	// extension the editor reads but the scanner does not index.
	copyTestFile(t, fx, filepath.Join(root, "Artist/Album/01.flac"))
	copyTestFile(t, fx, filepath.Join(root, "Artist/Live/01.flac"))
	copyTestFile(t, fx, filepath.Join(root, "Artist/Album/02.oga"))

	r, lib := realRescanHandler(t, root, []string{"^Live$"})

	body := `{"library_id": ` + strconv.FormatUint(uint64(lib.ID), 10) + `, "paths": [` +
		`"Artist/Album/01.flac", "Artist/Live/01.flac", "Artist/Album/02.oga"` +
		`], "fields": {"genres": ["Rock"]}}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp rescanResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	for _, row := range resp.Results {
		if !row.OK {
			t.Fatalf("every write should have succeeded: %s", w.Body.String())
		}
	}
	if resp.Rescan == nil || !resp.Rescan.OK {
		t.Fatalf("a correct save must not warn about the index: %+v", resp.Rescan)
	}
}

// A re-index that indexed fewer files than were written must not claim success:
// the response's contract is "rescan.ok means the library index is current".
func TestUpdateTracks_PartialRescanReportsNotOK(t *testing.T) {
	rs := &fakeRescanner{stats: &scanner.ScanStats{TracksProcessed: 0}}
	r, lib := rescanTestHandler(t, rs)
	resp := putTitle(t, r, lib.ID)
	if resp.Rescan == nil || resp.Rescan.OK {
		t.Fatalf("expected rescan not ok, got %+v", resp.Rescan)
	}
	if !strings.Contains(resp.Rescan.Error, "0 of 1") {
		t.Fatalf("expected the shortfall to be named, got %q", resp.Rescan.Error)
	}
}

func TestUpdateTracks_RescanErrorsReportNotOK(t *testing.T) {
	rs := &fakeRescanner{stats: &scanner.ScanStats{
		TracksProcessed: 1,
		Errors:          []error{errors.New(`read tags "ok.flac": broken`)},
	}}
	r, lib := rescanTestHandler(t, rs)
	resp := putTitle(t, r, lib.ID)
	if resp.Rescan == nil || resp.Rescan.OK {
		t.Fatalf("expected rescan not ok, got %+v", resp.Rescan)
	}
	if !strings.Contains(resp.Rescan.Error, "read tags") {
		t.Fatalf("expected the tag-read error to be reported, got %q", resp.Rescan.Error)
	}
}

// Many per-file errors are summarised, not concatenated: the message ends up in
// a toast.
func TestUpdateTracks_ManyRescanErrorsAreSummarised(t *testing.T) {
	rs := &fakeRescanner{stats: &scanner.ScanStats{
		TracksProcessed: 1,
		Errors: []error{
			errors.New("first failure"),
			errors.New("second failure"),
			errors.New("third failure"),
		},
	}}
	r, lib := rescanTestHandler(t, rs)
	resp := putTitle(t, r, lib.ID)
	if resp.Rescan == nil || resp.Rescan.OK {
		t.Fatalf("expected rescan not ok, got %+v", resp.Rescan)
	}
	if !strings.Contains(resp.Rescan.Error, "3 files could not be re-indexed") ||
		!strings.Contains(resp.Rescan.Error, "first failure") {
		t.Fatalf("expected a count plus the first error, got %q", resp.Rescan.Error)
	}
	if strings.Contains(resp.Rescan.Error, "third failure") {
		t.Fatalf("expected the tail of the errors to be summarised away, got %q", resp.Rescan.Error)
	}
}

func TestUpdateTracks_RescansWrittenPaths(t *testing.T) {
	root := t.TempDir()
	fx := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(fx); err != nil {
		t.Skipf("no fixture: %v", err)
	}
	dst := filepath.Join(root, "ok.flac")
	copyTestFile(t, fx, dst)

	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	_ = model.Migrate(db)
	s := store.New(db)
	lib := &model.Library{Name: "Main", Path: root}
	_ = s.CreateLibrary(lib)
	rs := &fakeRescanner{}
	h := &metaHandler.Handler{Store: s, Reader: nullReader{}, Rescan: rs}
	r := mux.NewRouter()
	h.Routes(r)

	body := `{
		"library_id": ` + strconv.FormatUint(uint64(lib.ID), 10) + `,
		"paths": ["ok.flac", "missing.flac"],
		"fields": { "title": "New Title" }
	}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if len(rs.calls) != 1 {
		t.Fatalf("expected one rescan call, got %d", len(rs.calls))
	}
	// Only the path that was actually written is re-indexed.
	if len(rs.calls[0]) != 1 || rs.calls[0][0] != dst {
		t.Fatalf("unexpected rescan paths: %v", rs.calls[0])
	}
	if rs.libs[0] != lib.ID {
		t.Fatalf("expected library %d, got %d", lib.ID, rs.libs[0])
	}

	var resp struct {
		Rescan *struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		} `json:"rescan"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Rescan == nil || !resp.Rescan.OK {
		t.Fatalf("expected rescan ok, got %+v", resp.Rescan)
	}
}

func TestUpdateTracks_RescanFailureStillSucceeds(t *testing.T) {
	rs := &fakeRescanner{err: errors.New("db is on fire")}
	r, lib := rescanTestHandler(t, rs)
	resp := putTitle(t, r, lib.ID)
	if resp.Rescan == nil || resp.Rescan.OK || resp.Rescan.Error != "db is on fire" {
		t.Fatalf("expected the rescan error to be reported, got %+v", resp.Rescan)
	}
}

func TestUpdateTracks_NoRescannerOmitsTheField(t *testing.T) {
	root := t.TempDir()
	fx := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(fx); err != nil {
		t.Skipf("no fixture: %v", err)
	}
	copyTestFile(t, fx, filepath.Join(root, "ok.flac"))
	_, r, lib := newTestHandler(t, root)

	body := `{"library_id": ` + strconv.FormatUint(uint64(lib.ID), 10) +
		`, "paths": ["ok.flac"], "fields": {"title": "T"}}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if bytes.Contains(w.Body.Bytes(), []byte(`"rescan"`)) {
		t.Fatalf("expected no rescan field without a rescanner: %s", w.Body.String())
	}
}
