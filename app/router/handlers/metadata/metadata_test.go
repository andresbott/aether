package metadata_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	metaHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	"github.com/andresbott/aether/app/router/handlers/problems"
	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/internal/tags"
	"github.com/go-bumbu/http/problemjson"
	"github.com/gorilla/mux"
	_taglib "go.senan.xyz/taglib"
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

func newTestHandler(t *testing.T, root string) (*mux.Router, scanfolder.Folder) {
	t.Helper()
	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Main", Path: root, FollowSymlinks: true}})
	if err != nil {
		t.Fatal(err)
	}
	folder, _ := set.ByName("Main")
	h := &metaHandler.TagsHandler{Folders: set, Reader: nullReader{}, Problems: problems.New(false)}
	r := mux.NewRouter()
	h.Routes(r)
	return r, folder
}

func TestFolders_ListsImmediateSubdirs(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Beatles"), 0o755); err != nil {
		t.Fatal(err)
	}
	r, folder := newTestHandler(t, root)
	reqURL := "/metadata/folders?scan_folder=" + url.QueryEscape(folder.Name) + "&path="
	req := httptest.NewRequest("GET", reqURL, nil)
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
	r, folder := newTestHandler(t, root)
	reqURL := "/metadata/folders?scan_folder=" + url.QueryEscape(folder.Name) + "&path=../"
	req := httptest.NewRequest("GET", reqURL, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestFolders_UnknownScanFolder404 confirms an unconfigured scan folder name
// answers 404 with a detail that says so, not merely a bare status.
func TestFolders_UnknownScanFolder404(t *testing.T) {
	r, _ := newTestHandler(t, t.TempDir())
	reqURL := "/metadata/folders?scan_folder=" + url.QueryEscape("No Such Folder") + "&path="
	req := httptest.NewRequest("GET", reqURL, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "is not configured") {
		t.Fatalf("expected detail to say the scan folder is not configured, got %s", w.Body.String())
	}
}

// A folder name with a space and a non-ASCII letter must round-trip through the
// query string: names are identifiers, and they are written by humans.
func TestFoldersAddressesAFolderWithANonTrivialName(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Artist"), 0o755); err != nil {
		t.Fatal(err)
	}
	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Música Clásica", Path: root}})
	if err != nil {
		t.Fatal(err)
	}
	r := mux.NewRouter()
	(&metaHandler.TagsHandler{Folders: set, Reader: nullReader{}, Problems: problems.New(false)}).Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/metadata/folders?scan_folder="+url.QueryEscape("Música Clásica"), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Artist") {
		t.Fatalf("got %d %s, want 200 listing Artist", rec.Code, rec.Body.String())
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
	r, folder := newTestHandler(t, root)
	// A search query returns matching folders from anywhere in the scan folder,
	// not just the immediate children the plain listing would return.
	reqURL := "/metadata/folders?scan_folder=" + url.QueryEscape(folder.Name) + "&q=up"
	req := httptest.NewRequest("GET", reqURL, nil)
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

	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Main", Path: root}})
	if err != nil {
		t.Fatal(err)
	}
	folder, _ := set.ByName("Main")
	h := &metaHandler.TagsHandler{Folders: set, Reader: stubTagReader{}, Problems: problems.New(false)}
	r := mux.NewRouter()
	h.Routes(r)

	reqURL := "/metadata/tracks?scan_folder=" + url.QueryEscape(folder.Name) + "&path=alb"
	req := httptest.NewRequest("GET", reqURL, nil)
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
	r, folder := newTestHandler(t, root)
	reqURL := "/metadata/tracks?scan_folder=" + url.QueryEscape(folder.Name) + "&path=../"
	req := httptest.NewRequest("GET", reqURL, nil)
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

	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Main", Path: root}})
	if err != nil {
		t.Fatal(err)
	}
	folder, _ := set.ByName("Main")
	h := &metaHandler.TagsHandler{Folders: set, Reader: nullReader{}, Problems: problems.New(false)}
	r := mux.NewRouter()
	h.Routes(r)

	body := `{
		"scan_folder": "` + folder.Name + `",
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

// warnHandler builds an updateTracks handler over a real fixture copied into the
// scan folder, so a save can partially fail (a missing sibling path) while one real
// file writes.
func warnHandler(t *testing.T) (*mux.Router, scanfolder.Folder) {
	t.Helper()
	root := t.TempDir()
	fx := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(fx); err != nil {
		t.Skipf("no fixture: %v", err)
	}
	copyTestFile(t, fx, filepath.Join(root, "ok.flac"))

	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Main", Path: root}})
	if err != nil {
		t.Fatal(err)
	}
	folder, _ := set.ByName("Main")
	h := &metaHandler.TagsHandler{Folders: set, Reader: nullReader{}, Problems: problems.New(false)}
	r := mux.NewRouter()
	h.Routes(r)
	return r, folder
}

func warnFromUpdate(t *testing.T, r *mux.Router, body string) string {
	t.Helper()
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Warning string `json:"warning,omitempty"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v: %s", err, w.Body.String())
	}
	return resp.Warning
}

// When an identity-affecting edit (album, album artist, MB release id) writes
// only part of the selection, the remaining files keep the old album identity
// on disk: the scanner sees a split and mints a new album row, stranding the old
// row's manual cover, stars and created_at on a remnant. The user must be warned
// that those may have moved. See internal/scanner/albumcontinuity.go.
func TestUpdateTracks_PartialIdentityEditWarnsAlbumMoved(t *testing.T) {
	r, folder := warnHandler(t)
	body := `{
		"scan_folder": "` + folder.Name + `",
		"paths": ["ok.flac", "missing.flac"],
		"fields": { "album": "New Album Name" }
	}`
	warning := warnFromUpdate(t, r, body)
	if warning == "" {
		t.Fatal("expected a warning that the album cover/stars may have moved, got none")
	}
	if !strings.Contains(strings.ToLower(warning), "album") {
		t.Fatalf("warning should mention the album, got %q", warning)
	}
}

// A partial failure on a non-identity field (title) cannot strand an album, so
// it must not raise the album-moved warning.
func TestUpdateTracks_PartialNonIdentityEditDoesNotWarn(t *testing.T) {
	r, folder := warnHandler(t)
	body := `{
		"scan_folder": "` + folder.Name + `",
		"paths": ["ok.flac", "missing.flac"],
		"fields": { "title": "New Title" }
	}`
	if warning := warnFromUpdate(t, r, body); warning != "" {
		t.Fatalf("a title edit cannot move an album; expected no warning, got %q", warning)
	}
}

// An identity edit where every file wrote is consistent on disk: continuity
// retags the album in place and nothing moves, so there is no warning.
func TestUpdateTracks_CompleteIdentityEditDoesNotWarn(t *testing.T) {
	r, folder := warnHandler(t)
	body := `{
		"scan_folder": "` + folder.Name + `",
		"paths": ["ok.flac"],
		"fields": { "album": "New Album Name" }
	}`
	if warning := warnFromUpdate(t, r, body); warning != "" {
		t.Fatalf("a fully-written album rename should not warn, got %q", warning)
	}
}

// Every row failing is still a processed batch, not a transport failure: the
// status stays 200 and each row carries its own error, exactly as rawTags
// does. Before this rule the handler flipped to 500 with the identical body,
// which made axios throw and lose the per-row detail in the SPA.
func TestUpdateTracks_AllRowsFailStill200(t *testing.T) {
	root := t.TempDir()
	r, folder := newTestHandler(t, root)

	body := `{
		"scan_folder": "` + folder.Name + `",
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
		Reindex *json.RawMessage `json:"reindex"`
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
	// Nothing was written, so there is nothing to re-index and no reindex report.
	if resp.Reindex != nil {
		t.Fatalf("expected no reindex when no file was written, got %s", string(*resp.Reindex))
	}
}

func TestUpdateTracks_RejectsTraversalPerPath(t *testing.T) {
	root := t.TempDir()
	r, folder := newTestHandler(t, root)
	body := `{"scan_folder": "` + folder.Name + `", "paths": ["../escape.mp3"], "fields": {"title": "x"}}`
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

	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Main", Path: root}})
	if err != nil {
		t.Fatal(err)
	}
	folder, _ := set.ByName("Main")
	h := &metaHandler.TagsHandler{Folders: set, Reader: nullReader{}, Problems: problems.New(false)}
	r := mux.NewRouter()
	h.Routes(r)

	body := `{
		"scan_folder": "` + folder.Name + `",
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

	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Main", Path: root}})
	if err != nil {
		t.Fatal(err)
	}
	folder, _ := set.ByName("Main")
	h := &metaHandler.TagsHandler{Folders: set, Reader: nullReader{}, Problems: problems.New(false)}
	r := mux.NewRouter()
	h.Routes(r)

	body := `{
		"scan_folder": "` + folder.Name + `",
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

	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Main", Path: root}})
	if err != nil {
		t.Fatal(err)
	}
	folder, _ := set.ByName("Main")
	h := &metaHandler.TagsHandler{Folders: set, Reader: nullReader{}, Problems: problems.New(false)}
	r := mux.NewRouter()
	h.Routes(r)

	body := `{
		"scan_folder": "` + folder.Name + `",
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

func TestUpdateTracks_ReleaseTypesWritten(t *testing.T) {
	root := t.TempDir()
	fx := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(fx); err != nil {
		t.Skipf("no fixture: %v", err)
	}
	dst := filepath.Join(root, "a.flac")
	copyTestFile(t, fx, dst)

	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Main", Path: root}})
	if err != nil {
		t.Fatal(err)
	}
	folder, _ := set.ByName("Main")
	h := &metaHandler.TagsHandler{Folders: set, Reader: nullReader{}, Problems: problems.New(false)}
	r := mux.NewRouter()
	h.Routes(r)

	body := `{
		"scan_folder": "` + folder.Name + `",
		"paths": ["a.flac"],
		"fields": { "release_types": ["Album", "Compilation"] }
	}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Written to the MusicBrainz key the readers prefer, not RELEASETYPE.
	got, err := taglibReadTags(dst)
	if err != nil {
		t.Fatal(err)
	}
	rt := got["MUSICBRAINZ_ALBUMTYPE"]
	if len(rt) != 2 || rt[0] != "Album" || rt[1] != "Compilation" {
		t.Fatalf("release types unexpected: %v", rt)
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

	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Main", Path: root}})
	if err != nil {
		t.Fatal(err)
	}
	folder, _ := set.ByName("Main")
	h := &metaHandler.TagsHandler{Folders: set, Reader: tags.TaglibReader{}, Problems: problems.New(false)}
	r := mux.NewRouter()
	h.Routes(r)

	body := `{
		"scan_folder": "` + folder.Name + `",
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
	r, _ := newTestHandler(t, t.TempDir())
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// An empty selection is well-formed but invalid: like every other
// paths[]-accepting endpoint (shared checkPaths), it answers a 422 itemising
// /paths, not a 400. The field validation still runs first, so a non-empty
// fields object is supplied to reach the selection check.
func TestUpdateTracks_EmptySelectionIs422(t *testing.T) {
	r, folder := newTestHandler(t, t.TempDir())
	body := `{
		"scan_folder": "` + folder.Name + `",
		"paths": [],
		"fields": { "title": "x" }
	}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for empty paths, got %d: %s", w.Code, w.Body.String())
	}
	var validation problemjson.ValidationDetails
	if err := json.Unmarshal(w.Body.Bytes(), &validation); err != nil {
		t.Fatal(err)
	}
	if len(validation.Errors) == 0 || validation.Errors[0].Pointer != "/paths" {
		t.Fatalf("expected a /paths field error, got %+v", validation.Errors)
	}
}

// A single request may not both rename an artist field and set its MusicBrainz
// IDs: the MB-ID map is keyed by the current names, so writing new names in the
// same request would produce a positionally-misaligned tag. The handler rejects
// the whole request so a corrupt tag is never written; the user saves them
// separately.
func TestUpdateTracks_RejectsArtistRenameWithMBID(t *testing.T) {
	r, folder := newTestHandler(t, t.TempDir())
	body := `{
		"scan_folder": "` + folder.Name + `",
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
	r, folder := newTestHandler(t, t.TempDir())
	body := `{
		"scan_folder": "` + folder.Name + `",
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

// fakeReindexer records what the handler asked to re-index. It returns a
// canned execution id (defaulting to "exec-1"), or the configured error.
type fakeReindexer struct {
	calls   [][]string
	folders []string
	id      string
	err     error
}

func (f *fakeReindexer) EnqueueReindex(_ context.Context, scanFolder string, absPaths []string) (string, error) {
	f.calls = append(f.calls, absPaths)
	f.folders = append(f.folders, scanFolder)
	if f.id == "" {
		f.id = "exec-1"
	}
	return f.id, f.err
}

// reindexTestHandler builds a handler over a real scan folder whose root is a
// temp dir holding one writable flac fixture.
func reindexTestHandler(t *testing.T, rx *fakeReindexer) (*mux.Router, scanfolder.Folder) {
	t.Helper()
	root := t.TempDir()
	fx := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(fx); err != nil {
		t.Skipf("no fixture: %v", err)
	}
	copyTestFile(t, fx, filepath.Join(root, "ok.flac"))

	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Main", Path: root}})
	if err != nil {
		t.Fatal(err)
	}
	folder, _ := set.ByName("Main")
	h := &metaHandler.TagsHandler{Folders: set, Reader: nullReader{}, Reindex: rx, Problems: problems.New(false)}
	r := mux.NewRouter()
	h.Routes(r)
	return r, folder
}

// reindexResponse is the slice of an update response the reindex assertions
// need.
type reindexResponse struct {
	Results []struct {
		OK bool `json:"ok"`
	} `json:"results"`
	Reindex *struct {
		ExecutionID string `json:"execution_id"`
	} `json:"reindex"`
}

// putTitle saves a title to ok.flac and decodes the response, asserting the
// write itself succeeded with a 200.
func putTitle(t *testing.T, r *mux.Router, scanFolder string) reindexResponse {
	t.Helper()
	body := `{"scan_folder": "` + scanFolder + `", "paths": ["ok.flac"], "fields": {"title": "T"}}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// The tags are already on disk; a failed enqueue must not fail the write.
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp reindexResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || !resp.Results[0].OK {
		t.Fatalf("expected the write to succeed: %s", w.Body.String())
	}
	return resp
}

func TestUpdateTracks_ReindexesWrittenPaths(t *testing.T) {
	root := t.TempDir()
	fx := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(fx); err != nil {
		t.Skipf("no fixture: %v", err)
	}
	dst := filepath.Join(root, "ok.flac")
	copyTestFile(t, fx, dst)

	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Main", Path: root}})
	if err != nil {
		t.Fatal(err)
	}
	folder, _ := set.ByName("Main")
	rx := &fakeReindexer{}
	h := &metaHandler.TagsHandler{Folders: set, Reader: nullReader{}, Reindex: rx, Problems: problems.New(false)}
	r := mux.NewRouter()
	h.Routes(r)

	body := `{
		"scan_folder": "` + folder.Name + `",
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

	if len(rx.calls) != 1 {
		t.Fatalf("expected one reindex call, got %d", len(rx.calls))
	}
	// Only the path that was actually written is re-indexed.
	if len(rx.calls[0]) != 1 || rx.calls[0][0] != dst {
		t.Fatalf("unexpected reindex paths: %v", rx.calls[0])
	}
	if rx.folders[0] != folder.Name {
		t.Fatalf("expected scan folder %q, got %q", folder.Name, rx.folders[0])
	}

	var resp struct {
		Reindex *struct {
			ExecutionID string `json:"execution_id"`
		} `json:"reindex"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Reindex == nil || resp.Reindex.ExecutionID != "exec-1" {
		t.Fatalf("expected a reindex execution id, got %+v", resp.Reindex)
	}
}

// A failure to enqueue is never fatal to the write — the file already landed
// on disk — and enqueueReindex swallows it silently (see rescan.go), so the
// response simply carries no "reindex" field.
func TestUpdateTracks_ReindexEnqueueFailureStillSucceeds(t *testing.T) {
	rx := &fakeReindexer{err: errors.New("db is on fire")}
	r, folder := reindexTestHandler(t, rx)
	resp := putTitle(t, r, folder.Name)
	if resp.Reindex != nil {
		t.Fatalf("expected no reindex reference when enqueue fails, got %+v", resp.Reindex)
	}
}

func TestUpdateTracks_NoReindexerOmitsTheField(t *testing.T) {
	root := t.TempDir()
	fx := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(fx); err != nil {
		t.Skipf("no fixture: %v", err)
	}
	copyTestFile(t, fx, filepath.Join(root, "ok.flac"))
	r, folder := newTestHandler(t, root)

	body := `{"scan_folder": "` + folder.Name + `", "paths": ["ok.flac"], "fields": {"title": "T"}}`
	req := httptest.NewRequest("PUT", "/metadata/tracks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if bytes.Contains(w.Body.Bytes(), []byte(`"reindex"`)) {
		t.Fatalf("expected no reindex field without a reindexer: %s", w.Body.String())
	}
}
