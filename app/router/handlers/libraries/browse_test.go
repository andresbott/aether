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
