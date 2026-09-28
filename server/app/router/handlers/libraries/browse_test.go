package libraries_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
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

// newBrowseTestHandler builds a handler with two configured scan folders: a
// real "Music" root under t.TempDir(), and an "Offline" root that is never
// created on disk — so a test can prove the empty-path root listing touches
// no filesystem at all (see browse's doc comment). Returns the router and
// both roots' absolute paths.
func newBrowseTestHandler(t *testing.T) (r *mux.Router, musicRoot, offlineRoot string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	musicRoot = t.TempDir()
	offlineRoot = filepath.Join(t.TempDir(), "not-mounted")
	folders, err := scanfolder.NewSet([]scanfolder.Folder{
		{Name: "Music", Path: musicRoot},
		{Name: "Offline", Path: offlineRoot},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := &libraries.Handler{Store: store.New(db), Folders: folders, Problems: problems.New(false)}
	mr := mux.NewRouter()
	h.Routes(mr)
	return mr, musicRoot, offlineRoot
}

// TestBrowseListsScanFolderRootsWithNoPath is the proof for the no-probe
// requirement: the Offline root does not exist on disk at all, yet it is
// still listed, unerrored, because the empty-path branch reads h.Folders
// from memory and never stats a root.
func TestBrowseListsScanFolderRootsWithNoPath(t *testing.T) {
	r, musicRoot, offlineRoot := newBrowseTestHandler(t)
	if _, err := os.Stat(offlineRoot); err == nil {
		t.Fatal("test setup bug: the offline root must not exist on disk")
	}

	req := httptest.NewRequest("GET", "/libraries/browse", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Path    string `json:"path"`
		Folders []struct {
			Name          string `json:"name"`
			Path          string `json:"path"`
			HasSubfolders bool   `json:"has_subfolders"`
			IsSymlink     bool   `json:"is_symlink"`
		} `json:"folders"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Path != "" {
		t.Fatalf("expected the empty path echoed back, got %q", body.Path)
	}
	if len(body.Folders) != 2 {
		t.Fatalf("expected the 2 configured roots, got %+v", body.Folders)
	}
	want := []struct{ name, path string }{{"Music", musicRoot}, {"Offline", offlineRoot}}
	for i, wf := range want {
		f := body.Folders[i]
		if f.Name != wf.name || f.Path != wf.path {
			t.Fatalf("folders[%d] = %+v, want name=%q path=%q (name order)", i, f, wf.name, wf.path)
		}
		if !f.HasSubfolders || f.IsSymlink {
			t.Fatalf("folders[%d] = %+v, want has_subfolders=true is_symlink=false (assumed, never probed)", i, f)
		}
	}
}

// TestBrowseListsSubfolders uses the root itself as the requested path,
// covering the "equal to a root" side of containment.
func TestBrowseListsSubfolders(t *testing.T) {
	r, musicRoot, _ := newBrowseTestHandler(t)
	dir := musicRoot
	if err := os.MkdirAll(filepath.Join(dir, "beta", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a-file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/libraries/browse?path="+url.QueryEscape(dir), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Path    string `json:"path"`
		Folders []struct {
			Name          string `json:"name"`
			Path          string `json:"path"`
			HasSubfolders bool   `json:"has_subfolders"`
		} `json:"folders"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Path != filepath.Clean(dir) {
		t.Fatalf("expected path %q, got %q", filepath.Clean(dir), body.Path)
	}
	if len(body.Folders) != 2 {
		t.Fatalf("expected 2 folders (files excluded), got %+v", body.Folders)
	}
	if body.Folders[0].Name != "alpha" || body.Folders[1].Name != "beta" {
		t.Fatalf("expected sorted [alpha beta], got %+v", body.Folders)
	}
	if body.Folders[0].Path != filepath.Join(dir, "alpha") {
		t.Fatalf("expected absolute child path, got %q", body.Folders[0].Path)
	}
	if body.Folders[0].HasSubfolders || !body.Folders[1].HasSubfolders {
		t.Fatalf("has_subfolders wrong: %+v", body.Folders)
	}
}

// TestBrowseHiddenFolders browses a directory nested one level under the
// root, covering the "under a root" (not merely "equal to") side of
// containment.
func TestBrowseHiddenFolders(t *testing.T) {
	r, musicRoot, _ := newBrowseTestHandler(t)
	dir := filepath.Join(musicRoot, "hidden-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "visible"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}

	names := func(query string) []string {
		req := httptest.NewRequest("GET", "/libraries/browse?path="+url.QueryEscape(dir)+query, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("query %q: expected 200, got %d, body=%s", query, w.Code, w.Body.String())
		}
		var body struct {
			Folders []struct {
				Name string `json:"name"`
			} `json:"folders"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(body.Folders))
		for _, f := range body.Folders {
			out = append(out, f.Name)
		}
		return out
	}

	if got := names(""); len(got) != 1 || got[0] != "visible" {
		t.Fatalf("default should hide dot-folders, got %v", got)
	}
	got := names("&show_hidden=true")
	if len(got) != 2 || got[0] != ".hidden" || got[1] != "visible" {
		t.Fatalf("show_hidden=true should list both, got %v", got)
	}
	if got := names("&show_hidden=false"); len(got) != 1 || got[0] != "visible" {
		t.Fatalf("show_hidden=false should hide dot-folders, got %v", got)
	}
}

func TestBrowseRejectsRelativePath(t *testing.T) {
	r, _, _ := newBrowseTestHandler(t)
	req := httptest.NewRequest("GET", "/libraries/browse?path="+url.QueryEscape("relative/path"), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

// TestBrowseRejectsPathOutsideEveryRoot covers both an unconfigured absolute
// path and "/" itself: neither is spelled under a configured root, so both
// are now refused at 422 (itemising /path) rather than the old "not a
// readable directory" 400 — the picker is confined, not merely checking the
// path is syntactically a directory.
func TestBrowseRejectsPathOutsideEveryRoot(t *testing.T) {
	r, _, _ := newBrowseTestHandler(t)
	for _, p := range []string{"/", "/nonexistent-aether-browse-xyz"} {
		t.Run(p, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/libraries/browse?path="+url.QueryEscape(p), nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("path %q: expected 422, got %d, body=%s", p, w.Code, w.Body.String())
			}
			var problem problemjson.ValidationDetails
			if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
				t.Fatal(err)
			}
			if len(problem.Errors) != 1 || problem.Errors[0].Pointer != "/path" {
				t.Fatalf("path %q: expected one /path field error, got %+v", p, problem.Errors)
			}
		})
	}
}

// TestBrowseRejectsMissingPathInsideRoot proves containment is checked
// before the stat, not instead of it: a path lexically under a root but
// missing on disk is still 400 ("not a readable directory"), not 422 — the
// pre-existing stat check is unchanged for a path that passes containment.
func TestBrowseRejectsMissingPathInsideRoot(t *testing.T) {
	r, musicRoot, _ := newBrowseTestHandler(t)
	p := filepath.Join(musicRoot, "does-not-exist")
	req := httptest.NewRequest("GET", "/libraries/browse?path="+url.QueryEscape(p), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (inside a root but missing on disk), got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestBrowseListsSymlinkedFolders(t *testing.T) {
	r, musicRoot, _ := newBrowseTestHandler(t)
	dir := filepath.Join(musicRoot, "symlink-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := os.Mkdir(filepath.Join(target, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	req := httptest.NewRequest("GET", "/libraries/browse?path="+url.QueryEscape(dir), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Folders []struct {
			Name          string `json:"name"`
			Path          string `json:"path"`
			HasSubfolders bool   `json:"has_subfolders"`
			IsSymlink     bool   `json:"is_symlink"`
		} `json:"folders"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Folders) != 2 {
		t.Fatalf("expected [linked real], got %+v", body.Folders)
	}
	link := body.Folders[0]
	if link.Name != "linked" || !link.IsSymlink || !link.HasSubfolders {
		t.Fatalf("linked should be an expandable symlink: %+v", link)
	}
	// The link is reported as typed, so the picker stores the symlink path,
	// even though it resolves to a directory OUTSIDE every configured root —
	// containment is checked lexically, on the path as spelled, not on where
	// a symlink inside it leads.
	if link.Path != filepath.Join(dir, "linked") {
		t.Fatalf("expected unresolved path, got %q", link.Path)
	}
	if body.Folders[1].Name != "real" || body.Folders[1].IsSymlink {
		t.Fatalf("real should be a plain folder: %+v", body.Folders[1])
	}
}

func TestBrowseFollowsSymlinkedFolderOnExpand(t *testing.T) {
	r, musicRoot, _ := newBrowseTestHandler(t)
	dir := filepath.Join(musicRoot, "symlink-expand-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := os.Mkdir(filepath.Join(target, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	req := httptest.NewRequest("GET", "/libraries/browse?path="+url.QueryEscape(link), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Path    string `json:"path"`
		Folders []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"folders"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Path != link {
		t.Fatalf("expected the symlink path echoed back, got %q", body.Path)
	}
	if len(body.Folders) != 1 || body.Folders[0].Name != "child" {
		t.Fatalf("expected the target's child listed, got %+v", body.Folders)
	}
	// Children stay under the symlink, so paths keep the admin's chosen prefix.
	if body.Folders[0].Path != filepath.Join(link, "child") {
		t.Fatalf("expected child under the symlink, got %q", body.Folders[0].Path)
	}
}

func TestBrowseRejectsBadShowHidden(t *testing.T) {
	r, _, _ := newBrowseTestHandler(t)
	req := httptest.NewRequest("GET", "/libraries/browse?path=/&show_hidden=maybe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}
