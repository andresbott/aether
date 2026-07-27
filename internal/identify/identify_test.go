package identify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andresbott/aether/libs/acoustid"
	"github.com/andresbott/aether/libs/fpcalc"
)

// writeFakeFpcalc creates an executable script that echoes a fixed JSON
// payload, standing in for the real binary.
func writeFakeFpcalc(t *testing.T, payload string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fpcalc")
	script := "#!/bin/sh\necho '" + payload + "'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func newAcoustClient(srv *httptest.Server) *acoustid.Client {
	c := acoustid.New("test-key", "test-agent")
	c.BaseURL = srv.URL
	c.Client = srv.Client()
	return c
}

func TestIdentifyFile(t *testing.T) {
	bin := writeFakeFpcalc(t, `{"duration": 123.4, "fingerprint": "ABC123"}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"status": "ok",
			"results": [{
				"score": 0.97,
				"recordings": [{"id": "rec-mbid-1", "title": "Song One"}]
			}]
		}`))
	}))
	defer srv.Close()

	id := New(fpcalc.New(bin), newAcoustClient(srv))
	recs, err := id.IdentifyFile(context.Background(), "/some/file.mp3")
	if err != nil {
		t.Fatalf("IdentifyFile: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 recording, got %d", len(recs))
	}
	if recs[0].MBID != "rec-mbid-1" || recs[0].Title != "Song One" || recs[0].Score != 0.97 {
		t.Fatalf("unexpected recording: %+v", recs[0])
	}
}

func TestIdentifyFileNoMatch(t *testing.T) {
	bin := writeFakeFpcalc(t, `{"duration": 12.3, "fingerprint": "XYZ"}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status": "ok", "results": []}`))
	}))
	defer srv.Close()

	id := New(fpcalc.New(bin), newAcoustClient(srv))
	recs, err := id.IdentifyFile(context.Background(), "/some/file.mp3")
	if err != nil {
		t.Fatalf("IdentifyFile: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("expected no recordings, got %d", len(recs))
	}
}

func TestIdentifyFileFingerprintError(t *testing.T) {
	id := New(fpcalc.New(filepath.Join(t.TempDir(), "does-not-exist")), acoustid.New("k", "ua"))
	_, err := id.IdentifyFile(context.Background(), "/some/file.mp3")
	if err == nil || !strings.Contains(err.Error(), "fingerprint:") {
		t.Fatalf("expected fingerprint error, got %v", err)
	}
}

func TestIdentifyFileLookupError(t *testing.T) {
	bin := writeFakeFpcalc(t, `{"duration": 12.3, "fingerprint": "XYZ"}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	id := New(fpcalc.New(bin), newAcoustClient(srv))
	_, err := id.IdentifyFile(context.Background(), "/some/file.mp3")
	if err == nil || !strings.Contains(err.Error(), "acoustid:") {
		t.Fatalf("expected acoustid error, got %v", err)
	}
}

func TestIdentifyFileWithDurationReturnsFpcalcDuration(t *testing.T) {
	bin := writeFakeFpcalc(t, `{"duration": 245.7, "fingerprint": "ABC123"}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"status": "ok",
			"results": [{
				"score": 0.9,
				"recordings": [{"id": "rec-mbid-1", "title": "Song One"}]
			}]
		}`))
	}))
	defer srv.Close()

	id := New(fpcalc.New(bin), newAcoustClient(srv))
	recs, dur, err := id.IdentifyFileWithDuration(context.Background(), "/some/file.mp3")
	if err != nil {
		t.Fatalf("IdentifyFileWithDuration: %v", err)
	}
	if dur != 245.7 {
		t.Fatalf("expected duration 245.7, got %v", dur)
	}
	if len(recs) != 1 || recs[0].MBID != "rec-mbid-1" {
		t.Fatalf("unexpected recordings: %+v", recs)
	}
}

func TestIdentifyFileWithDurationFingerprintError(t *testing.T) {
	id := New(fpcalc.New(filepath.Join(t.TempDir(), "does-not-exist")), acoustid.New("k", "ua"))
	_, dur, err := id.IdentifyFileWithDuration(context.Background(), "/some/file.mp3")
	if err == nil || !strings.Contains(err.Error(), "fingerprint:") {
		t.Fatalf("expected fingerprint error, got %v", err)
	}
	if dur != 0 {
		t.Fatalf("expected duration 0 on fingerprint error, got %v", dur)
	}
}
