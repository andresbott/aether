package spa

import (
	"mime"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// The web app manifest must not go out as text/plain: browsers ignore a manifest
// served with the wrong type, silently disabling install/standalone mode. Go's
// built-in mime table has no .webmanifest entry, so the package registers one.
func TestWebmanifestExtensionIsRegistered(t *testing.T) {
	got := mime.TypeByExtension(".webmanifest")
	if got != "application/manifest+json" {
		t.Errorf("mime.TypeByExtension(\".webmanifest\") = %q, want application/manifest+json", got)
	}
}

// The registration only matters if it reaches the response, which it does via
// net/http's file server. The embedded SPA is built by `make`, so this exercises
// the same code path over a synthetic FS instead.
func TestManifestServedWithManifestContentType(t *testing.T) {
	fs := fstest.MapFS{
		"manifest.webmanifest": &fstest.MapFile{Data: []byte(`{"name":"Aether"}`)},
	}

	req := httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil)
	rec := httptest.NewRecorder()
	http.FileServerFS(fs).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/manifest+json" {
		t.Errorf("Content-Type = %q, want application/manifest+json", got)
	}
}
