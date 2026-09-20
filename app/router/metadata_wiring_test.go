package router

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/scanfolder"
	"gorm.io/gorm"
)

// This test exists because of a mutation the whole suite survived: deleting
// `scanFolders: cfg.ScanFolders,` from New (main.go) left `go test ./app/router/`
// green, even though every metadata-editor request would then answer 404 "scan
// folder is not configured". The response-contract test only checked status +
// schema (an empty list validates), and nothing drove a /api/v0/metadata/*
// request through router.New at all. So this file pins the wiring itself: each
// request below answers differently when the configured set reaches the handler
// than when it does not.
func TestMetadataHandlersAreWiredToTheConfiguredScanFolders(t *testing.T) {
	doc := specDoc(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Artist"), 0o750); err != nil {
		t.Fatal(err)
	}
	// Non-ASCII on purpose: the name travels percent-encoded in the query and
	// must come back out of url.Values matching the configured one byte for byte.
	const folderName = "Música"
	h, _ := newNativeAuthRouter(t, func(t *testing.T, cfg *Cfg, _ *gorm.DB) {
		t.Helper()
		set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: folderName, Path: root}})
		if err != nil {
			t.Fatal(err)
		}
		cfg.ScanFolders = set
		// The metadata routes are mounted only when a tag reader is configured.
		cfg.TagReader = noopTagReader{}
	})
	_, attach := doLogin(t, h, "alice", "secret")

	get := func(t *testing.T, target string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, target, nil)
		attach(req)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}

	// TagsHandler: the configured folder is browsable, and its contents come
	// from the configured root.
	t.Run("folders lists the configured root", func(t *testing.T) {
		w := get(t, "/api/v0/metadata/folders?scan_folder="+url.QueryEscape(folderName))
		if w.Code != http.StatusOK {
			t.Fatalf("GET /metadata/folders = %d, want 200: %s", w.Code, w.Body.String())
		}
		assertJSONResponse(t, doc, "listMetadataFolders", http.StatusOK, w)
		var body struct {
			Folders []struct {
				Name string `json:"name"`
			} `json:"folders"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decoding the body: %v: %s", err, w.Body.String())
		}
		if len(body.Folders) != 1 || body.Folders[0].Name != "Artist" {
			t.Fatalf("folders = %+v, want the one sub-directory of the configured root", body.Folders)
		}
	})

	t.Run("an unconfigured name is 404", func(t *testing.T) {
		w := get(t, "/api/v0/metadata/folders?scan_folder="+url.QueryEscape("No Such Folder"))
		if w.Code != http.StatusNotFound {
			t.Fatalf("GET /metadata/folders for an unknown name = %d, want 404: %s", w.Code, w.Body.String())
		}
		assertJSONResponse(t, doc, "listMetadataFolders", http.StatusNotFound, w)
		assertDetailContains(t, w, "is not configured")
	})

	t.Run("no scan_folder at all is 400", func(t *testing.T) {
		w := get(t, "/api/v0/metadata/folders")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("GET /metadata/folders without scan_folder = %d, want 400: %s", w.Code, w.Body.String())
		}
		assertJSONResponse(t, doc, "listMetadataFolders", http.StatusBadRequest, w)
		assertDetailContains(t, w, "scan_folder required")
	})

	// ImagesHandler is mounted separately from TagsHandler, so it needs its own
	// request: `{eligible:false}` is the honest answer for a folder with no
	// tagged albums under it, and it is only reachable once the folder resolved.
	t.Run("artist-folder resolves the same folder", func(t *testing.T) {
		w := get(t, "/api/v0/metadata/artist-folder?scan_folder="+url.QueryEscape(folderName)+"&path=Artist")
		if w.Code != http.StatusOK {
			t.Fatalf("GET /metadata/artist-folder = %d, want 200: %s", w.Code, w.Body.String())
		}
		assertJSONResponse(t, doc, "getArtistFolder", http.StatusOK, w)
	})

	// The same missing-field mapping must hold in every transport, not only the
	// query string: a form endpoint answers 400 too. setArtistImage checks
	// `path` BEFORE `scan_folder`, so path must be non-empty or this asserts the
	// wrong 400.
	t.Run("a form without scan_folder is 400", func(t *testing.T) {
		body := &bytes.Buffer{}
		form := multipart.NewWriter(body)
		if err := form.WriteField("path", "Artist"); err != nil {
			t.Fatal(err)
		}
		if err := form.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v0/metadata/artist-image", body)
		req.Header.Set("Content-Type", form.FormDataContentType())
		attach(req)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("POST /metadata/artist-image without scan_folder = %d, want 400: %s", w.Code, w.Body.String())
		}
		assertJSONResponse(t, doc, "setArtistImage", http.StatusBadRequest, w)
		assertDetailContains(t, w, "scan_folder required")
	})

	// IdentifyHandler is the third handler holding the set, and its wiring is
	// NOT observable here: identify and identify-album both answer 503
	// identify_unavailable before they look at scan_folder, and this router has
	// no *identify.Identifier (one needs fpcalc). Rather than contort the test
	// into configuring a fingerprinter, the point is recorded: if that
	// precedence ever changes, add an unknown-folder → 404 case here.
}

// assertDetailContains checks the problem+json body's human-readable detail.
// The router runs in ModeDev in tests, so detail is present and unmasked.
func assertDetailContains(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var problem struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decoding the problem body: %v: %s", err, w.Body.String())
	}
	if !strings.Contains(problem.Detail, want) {
		t.Fatalf("detail = %q, want it to contain %q", problem.Detail, want)
	}
}
