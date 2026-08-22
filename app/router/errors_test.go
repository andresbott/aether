package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andresbott/aether/app/router/handlers/httperr"
)

// The production middleware wraps any >=400 body in an envelope, shaped by
// mount: the internal admin API (apiV1MountPrefix, "/api/v1") gets a
// problem+json Problem; everything else (chiefly /rest) keeps the legacy
// apiError{error,code} shape unchanged. Our /api/v1 handlers already answer
// JSON (most already problem+json via httperr), so without care the client
// receives an envelope whose "detail" is an escaped JSON *document* — which
// the UI then shows verbatim (the {"error":...,"code":"upstream_error"}
// string users saw on a failed cover search, back when the envelope's own
// shape was that ad hoc object rather than a Problem). These tests pin the
// contract: one envelope per mount, "detail" is a sentence, and the
// handler's own type/slug survives.
//
// GET /api/v1/radiobrowser/search without q is a real registered handler that
// answers a JSON 400 through writeError, so it exercises the whole stack.
const jsonErrPath = "/api/v1/radiobrowser/search"

func newTestRouter(t *testing.T) *MainAppHandler {
	t.Helper()
	h, err := New(Cfg{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h
}

func TestApiErrorBodyIsNotDoubleWrapped(t *testing.T) {
	h := newTestRouter(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, jsonErrPath, nil))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	var body httperr.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not JSON: %s", w.Body.String())
	}
	if body.Detail == "" {
		t.Fatalf(`"detail" is empty: %s`, w.Body.String())
	}
	if strings.Contains(body.Detail, `{`) || strings.Contains(body.Detail, `"`) {
		t.Fatalf("error message contains a nested JSON document: %q", body.Detail)
	}
}

// The handler-authored code (e.g. "validation_error", "upstream_error") must
// reach the client; the middleware must not replace it with a numeric status.
func TestApiErrorKeepsHandlerCode(t *testing.T) {
	h := newTestRouter(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, jsonErrPath, nil))

	var body httperr.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not the expected envelope: %s", w.Body.String())
	}
	if got := httperr.Slug(body.Type); got != "validation_error" {
		t.Errorf("code = %q, want the handler's own code to survive", got)
	}
	if body.Detail != "q is required" {
		t.Errorf("error = %q, want the handler's own message", body.Detail)
	}
}

// Successful responses must pass through untouched — the envelope writer must
// never buffer or rewrite a 200 (audio streaming, task logs, the SPA itself).
func TestSuccessResponsesArePassedThrough(t *testing.T) {
	h := newTestRouter(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body was altered: %s", w.Body.String())
	}
	if body["status"] != "ok" {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

// Subsonic reports its own errors inside a 200 envelope; the error writer must
// not interfere with that surface.
func TestSubsonicErrorEnvelopeIsUntouched(t *testing.T) {
	h := newTestRouter(t)
	w := httptest.NewRecorder()
	// No store configured, so /rest is not registered and this falls to the SPA
	// handler; assert only that we did not turn a non-error into an error.
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/rest/ping.view", nil))
	if w.Code >= 500 {
		t.Fatalf("unexpected server error: %d %s", w.Code, w.Body.String())
	}
}

// Handlers that answer plain text (http.Error) still need an envelope — the
// SPA parses every /api/v1 failure as problem+json, not just the ones a
// handler builds itself via httperr. This exercises the real /api/v1
// catch-all (api_v1.go), which answers unmatched paths with a bare
// http.Error.
func TestPlainTextHandlerErrorsGetProblemJSON(t *testing.T) {
	h := newTestRouter(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/does-not-exist", nil))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	var body httperr.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("plain-text handler error was not wrapped as problem+json: %s", w.Body.String())
	}
	if !strings.Contains(body.Detail, "wrong api call") {
		t.Errorf("detail = %q, want the handler's message", body.Detail)
	}
	if got := httperr.Slug(body.Type); got != "validation_error" {
		t.Errorf("slug = %q, want validation_error (400)", got)
	}
	if body.Instance != "/api/v1/does-not-exist" {
		t.Errorf("instance = %q, want the request path", body.Instance)
	}
}

// A bare http.NotFound (the pictureImage "cell not found" case, or any other
// handler that answers 404 without going through httperr) must come out as
// problem+json too, exercised directly against the middleware rather than a
// specific registered route.
func TestBareNotFoundBecomesProblemJSON(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil)
	jsonErrorEnvelope(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})).ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	var body httperr.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("http.NotFound body was not wrapped as problem+json: %s", w.Body.String())
	}
	if body.Status != http.StatusNotFound {
		t.Errorf("Status = %d, want 404", body.Status)
	}
	if got := httperr.Slug(body.Type); got != "not_found" {
		t.Errorf("slug = %q, want not_found", got)
	}
	if body.Instance != "/api/v1/nope" {
		t.Errorf("instance = %q, want the request path", body.Instance)
	}
}

// A bare plain-text error OUTSIDE the admin API — e.g. one of /rest's three
// raw-status media-serving edge cases (media.go's http.NotFound/http.Error
// calls, for a vanished cover/stream file or no cover source at all) — must
// keep the legacy apiError{error,code} shape, byte-identical to what a
// client received before this middleware ever learned about problem+json:
// /rest must never answer application/problem+json.
func TestBareErrorOutsideApiV1KeepsLegacyShape(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getCoverArt.view", nil)
	jsonErrorEnvelope(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})).ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json (the legacy shape, not problem+json)", ct)
	}
	var body struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not the legacy envelope: %s", w.Body.String())
	}
	if body.Code != "not_found" {
		t.Errorf("code = %q, want not_found", body.Code)
	}
	if body.Error != "404 page not found" {
		t.Errorf("error = %q, want Go's stock http.NotFound message", body.Error)
	}
	// A Problem's own field names must be absent — proves this is genuinely
	// the legacy shape, not a Problem that happened to also decode leniently
	// into the struct above.
	if strings.Contains(w.Body.String(), `"type"`) || strings.Contains(w.Body.String(), `"instance"`) {
		t.Errorf("body looks like a Problem, want the legacy {error,code} shape: %s", w.Body.String())
	}
}
