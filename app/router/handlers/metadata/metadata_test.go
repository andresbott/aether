package metadata_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	metaHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/mux"
	_taglib "go.senan.xyz/taglib"
	"gorm.io/gorm"
)

type nullReader struct{}

func (nullReader) CanRead(string) bool                { return false }
func (nullReader) Read(string) (tags.Metadata, error) { return tags.Metadata{}, nil }

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

type stubTagReader struct{}

func (stubTagReader) CanRead(p string) bool {
	return filepath.Ext(p) == ".mp3" || filepath.Ext(p) == ".flac"
}
func (stubTagReader) Read(p string) (tags.Metadata, error) {
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
