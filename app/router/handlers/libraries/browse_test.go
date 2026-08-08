package libraries_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestBrowseListsSubfolders(t *testing.T) {
	_, _, r := newTestHandler(t)
	dir := t.TempDir()
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
	if body.Path != dir {
		t.Fatalf("expected path %q, got %q", dir, body.Path)
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

func TestBrowseHiddenFolders(t *testing.T) {
	_, _, r := newTestHandler(t)
	dir := t.TempDir()
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

func TestBrowseRejectsBadPaths(t *testing.T) {
	_, _, r := newTestHandler(t)
	for _, p := range []string{"relative/path", "/nonexistent-aether-browse-xyz"} {
		req := httptest.NewRequest("GET", "/libraries/browse?path="+url.QueryEscape(p), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("path %q: expected 400, got %d", p, w.Code)
		}
	}
}

func TestBrowseListsSymlinkedFolders(t *testing.T) {
	_, _, r := newTestHandler(t)
	dir := t.TempDir()
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
	// The link is reported as typed, so the picker stores the symlink path.
	if link.Path != filepath.Join(dir, "linked") {
		t.Fatalf("expected unresolved path, got %q", link.Path)
	}
	if body.Folders[1].Name != "real" || body.Folders[1].IsSymlink {
		t.Fatalf("real should be a plain folder: %+v", body.Folders[1])
	}
}

func TestBrowseFollowsSymlinkedFolderOnExpand(t *testing.T) {
	_, _, r := newTestHandler(t)
	dir := t.TempDir()
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
	_, _, r := newTestHandler(t)
	req := httptest.NewRequest("GET", "/libraries/browse?path=/&show_hidden=maybe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}
