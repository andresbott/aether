package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The production middleware wraps any >=400 body in a JSON envelope. Our
// /api/v1 handlers already answer JSON, so without care the client receives an
// envelope whose "error" is an escaped JSON *document* — which the UI then shows
// verbatim (the {"error":...,"code":"upstream_error"} string users saw on a
// failed cover search). These tests pin the contract: one envelope, "error" is
// a sentence, and the handler's own code survives.
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
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not JSON: %s", w.Body.String())
	}
	msg, ok := body["error"].(string)
	if !ok {
		t.Fatalf(`"error" is not a string: %s`, w.Body.String())
	}
	if strings.Contains(msg, `{`) || strings.Contains(msg, `"`) {
		t.Fatalf("error message contains a nested JSON document: %q", msg)
	}
}

// The handler-authored code (e.g. "validation_error", "upstream_error") must
// reach the client; the middleware must not replace it with a numeric status.
func TestApiErrorKeepsHandlerCode(t *testing.T) {
	h := newTestRouter(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, jsonErrPath, nil))

	var body struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not the expected envelope: %s", w.Body.String())
	}
	if body.Code != "validation_error" {
		t.Errorf("code = %q, want the handler's own code to survive", body.Code)
	}
	if body.Error != "q is required" {
		t.Errorf("error = %q, want the handler's own message", body.Error)
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

// Handlers that answer plain text (http.Error) still need the JSON envelope —
// the SPA parses every /api/v1 failure as JSON.
func TestPlainTextHandlerErrorsStillGetJSONEnvelope(t *testing.T) {
	h := newTestRouter(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/does-not-exist", nil))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", w.Code, w.Body.String())
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("plain-text handler error was not wrapped as JSON: %s", w.Body.String())
	}
	if !strings.Contains(body.Error, "wrong api call") {
		t.Errorf("error = %q, want the handler's message", body.Error)
	}
}
