package identify

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/upstream"
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

// The chain that made writeUpstreamErr reachable for the real resolver: an
// AcoustID failure must arrive as an *upstream.Error so callers can classify it
// with errors.As instead of matching error text.
func TestIdentifyFileClassifiesAcoustIDFailuresAsUpstream(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body       string
		wantKind   upstream.Kind
		wantStatus int
	}{
		{"rate limited", http.StatusTooManyRequests, `{"status":"error","error":{"message":"rate limit"}}`,
			upstream.KindRateLimited, http.StatusTooManyRequests},
		{"server error", http.StatusBadGateway, `{"status":"error"}`,
			upstream.KindUnavailable, http.StatusBadGateway},
		{"rejected", http.StatusBadRequest, `{"status":"error","error":{"message":"invalid API key"}}`,
			upstream.KindRejected, http.StatusBadGateway},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := writeFakeFpcalc(t, `{"duration": 12.3, "fingerprint": "XYZ"}`)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			id := New(fpcalc.New(bin), newAcoustClient(srv))
			_, _, err := id.IdentifyFileWithDuration(context.Background(), "/some/file.mp3")
			if err == nil {
				t.Fatal("expected an error")
			}
			var uerr *upstream.Error
			if !errors.As(err, &uerr) {
				t.Fatalf("expected errors.As to reach *upstream.Error, got %v", err)
			}
			if uerr.Kind != tc.wantKind {
				t.Fatalf("expected kind %v, got %v", tc.wantKind, uerr.Kind)
			}
			if got := upstream.HTTPStatus(err); got != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, got)
			}
			// UserMessage must name the service and stay free of Go error text.
			if msg := upstream.UserMessage(err, "fallback"); !strings.Contains(msg, "AcoustID") {
				t.Fatalf("expected AcoustID named in the user message, got %q", msg)
			}
		})
	}
}

// An unreachable service (no listener at all) is a transport failure, not a
// status-code failure, and must classify as such.
func TestIdentifyFileClassifiesAnUnreachableServiceAsUpstream(t *testing.T) {
	bin := writeFakeFpcalc(t, `{"duration": 12.3, "fingerprint": "XYZ"}`)
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	client := newAcoustClient(srv)
	// Close the server so the next request is refused: an outage without a
	// response, exactly what a down AcoustID looks like.
	srv.Close()

	id := New(fpcalc.New(bin), client)
	_, _, err := id.IdentifyFileWithDuration(context.Background(), "/some/file.mp3")
	var uerr *upstream.Error
	if !errors.As(err, &uerr) {
		t.Fatalf("expected errors.As to reach *upstream.Error, got %v", err)
	}
	if uerr.Kind != upstream.KindUnreachable && uerr.Kind != upstream.KindTimeout {
		t.Fatalf("expected an unreachable/timeout kind, got %v", uerr.Kind)
	}
}

// A fingerprint failure is a FILE problem, not an outage: it must not be
// classified as upstream, or one unreadable file would fail the whole request.
func TestIdentifyFileDoesNotClassifyFingerprintFailuresAsUpstream(t *testing.T) {
	id := New(fpcalc.New(filepath.Join(t.TempDir(), "does-not-exist")), acoustid.New("k", "ua"))
	_, _, err := id.IdentifyFileWithDuration(context.Background(), "/some/file.mp3")
	if err == nil {
		t.Fatal("expected an error")
	}
	var uerr *upstream.Error
	if errors.As(err, &uerr) {
		t.Fatalf("a fingerprint failure must not be an upstream error, got %v", uerr)
	}
}

// A cancelled caller context is the caller's business, not an AcoustID outage.
func TestIdentifyFileDoesNotClassifyContextCancellationAsUpstream(t *testing.T) {
	bin := writeFakeFpcalc(t, `{"duration": 12.3, "fingerprint": "XYZ"}`)
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	id := New(fpcalc.New(bin), newAcoustClient(srv))
	_, _, err := id.IdentifyFileWithDuration(ctx, "/some/file.mp3")
	if err == nil {
		t.Fatal("expected an error")
	}
	var uerr *upstream.Error
	if errors.As(err, &uerr) {
		t.Fatalf("a cancelled context must not be an upstream error, got %v", uerr)
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
