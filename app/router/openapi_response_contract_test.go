package router

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// This file is the response-contract test: it drives real /api/v1 requests
// through the same maximal-router/auth harness openapi_coverage_test.go and
// auth_test.go already establish, and validates each captured response BODY
// against docs/openapi/aether-v1.yaml's schema for that operation and
// status. TestOpenAPICoversAllV1Routes (openapi_coverage_test.go) only
// checks that a (method, path) exists on both sides; spec-lint only checks
// the document is well-formed. Neither ever decodes a real handler response
// and checks its shape against the schema — this file is what closes that
// gap: a field rename, a casing slip, a dropped `required`, or an enum value
// the spec doesn't know about will fail a test here.
//
// Validator choice: github.com/getkin/kin-openapi (openapi3), confirmed by a
// feasibility spike to load and correctly VALIDATE this spec's OpenAPI
// 3.1-only constructs — `type: [string, 'null']` (TokenInfo.lastUsedAt/
// expiresAt, Library.last_scan_started_at, ...), `oneOf: [$ref, {type:
// 'null'}]` (MeResponse.user) and `allOf` composition (ValidationProblem,
// Task.schedule) — accepting a valid body under each shape and REJECTING an
// invalid one (wrong type, missing required field, a oneOf value matching
// neither branch). EnableJSONSchema2020() is passed to VisitJSON per
// kin-openapi's own guidance for 3.1+ documents.

// specDoc parses and OpenAPI-validates docs/openapi/aether-v1.yaml once for
// the whole test binary (loading + validating a ~3700 line document on every
// call would needlessly slow the suite down). Resolved relative to this
// source file, like loadSpecOperations in openapi_coverage_test.go, so it
// does not depend on "go test"'s working directory.
var specDocOnce = sync.OnceValues(func() (*openapi3.T, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, errors.New("could not resolve this test file's own location")
	}
	specPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "docs", "openapi", "aether-v1.yaml")
	doc, err := openapi3.NewLoader().LoadFromFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", specPath, err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("%s failed OpenAPI validation: %w", specPath, err)
	}
	return doc, nil
})

func specDoc(t *testing.T) *openapi3.T {
	t.Helper()
	doc, err := specDocOnce()
	if err != nil {
		t.Fatalf("openapi spec: %v", err)
	}
	return doc
}

// operationByID finds the operation with the given operationId anywhere in
// the spec. docs/openapi/aether-v1.yaml's paths are mount-relative
// (api-conventions.md, "Mount-relative paths") while every request in this
// file goes through the real /api/v1 mount, so looking operations up by
// their stable operationId — rather than reconstructing a mount-relative
// path by hand at each call site — is both less error-prone and self-
// documenting about which spec operation a given request is expected to
// satisfy.
func operationByID(t *testing.T, doc *openapi3.T, id string) *openapi3.Operation {
	t.Helper()
	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			if op.OperationID == id {
				return op
			}
		}
	}
	t.Fatalf("no operation with operationId %q in docs/openapi/aether-v1.yaml", id)
	return nil
}

// contentTypeBase strips any parameters (e.g. "; charset=utf-8") off a
// Content-Type header value, so it can be matched against the bare media
// type keys the spec's `content:` maps use.
func contentTypeBase(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.TrimSpace(ct)
}

// responseSchema resolves the schema docs/openapi/aether-v1.yaml documents
// for operationID's response at status, under contentType. Missing
// operation/status/media-type is a Fatalf, not a skip: this file exists to
// prove response bodies match the spec, so an assertion that silently
// checked nothing would defeat the point.
func responseSchema(t *testing.T, doc *openapi3.T, operationID string, status int, contentType string) *openapi3.Schema {
	t.Helper()
	op := operationByID(t, doc, operationID)
	respRef := op.Responses.Status(status)
	if respRef == nil || respRef.Value == nil {
		t.Fatalf("%s: spec has no documented response for status %d", operationID, status)
	}
	media := respRef.Value.Content.Get(contentType)
	if media == nil || media.Schema == nil || media.Schema.Value == nil {
		t.Fatalf("%s %d: spec documents no %q schema (response Content-Type)", operationID, status, contentType)
	}
	return media.Schema.Value
}

// assertJSONResponse validates w's JSON body against the schema
// docs/openapi/aether-v1.yaml declares for operationID's response at status.
// It also pins the Content-Type actually sent to a JSON media type, since a
// schema match against the wrong media type (e.g. a plain-json body that
// should have been problem+json) would prove nothing.
func assertJSONResponse(t *testing.T, doc *openapi3.T, operationID string, status int, w *httptest.ResponseRecorder) {
	t.Helper()
	ct := contentTypeBase(w.Header().Get("Content-Type"))
	if ct != "application/json" && ct != "application/problem+json" {
		t.Fatalf("%s %d: Content-Type = %q, want application/json or application/problem+json",
			operationID, status, w.Header().Get("Content-Type"))
	}
	schema := responseSchema(t, doc, operationID, status, ct)

	var data any
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatalf("%s %d: response body is not JSON: %v\nbody: %s", operationID, status, err, w.Body.String())
	}
	if err := schema.VisitJSON(data, openapi3.EnableJSONSchema2020()); err != nil {
		t.Errorf("%s %d: response body does not match its documented schema:\n%v\nbody: %s",
			operationID, status, err, w.Body.String())
	}
}

// mustJSON marshals v for use as a request body; failures are a test setup
// bug (a value that cannot marshal), not something under test.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	return b
}

// newContractTaskRouter builds a native-auth MainAppHandler with the task
// runner and schedule store both wired, via withTaskRunner (auth_test.go) —
// the one piece newNativeAuthRouter otherwise leaves unset, and attachApiV1
// (api_v1.go) gates the entire /tasks group on a non-nil task runner —
// logged in as the sole admin user so every call site here can attach the
// session immediately. Runner tasks are never actually started (no
// Runner.Start()/RegisterTask call): AddRun only enqueues by name
// (internal/taskrunner, github.com/go-bumbu/tempo's TaskQueue.Add), so
// triggerTask's immediate HTTP response — the only thing this file asserts
// about it — needs no registered task function.
func newContractTaskRouter(t *testing.T) (*MainAppHandler, func(*http.Request)) {
	t.Helper()
	h, _ := newNativeAuthRouter(t, withTaskRunner)
	_, attach := doLogin(t, h, "alice", "secret")
	return h, attach
}

// --- Bootstrap: health, version, me (anonymous and authenticated) ---

func TestContractBootstrapResponses(t *testing.T) {
	doc := specDoc(t)
	h, _ := newNativeAuthRouter(t)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /health = %d, want 200: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "getHealth", http.StatusOK, w)

	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/version", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /version = %d, want 200: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "getVersion", http.StatusOK, w)

	// Anonymous: MeResponse.user must validate as null (the oneOf's second
	// branch, `{type: 'null'}` — OpenAPI 3.1 syntax).
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /me (anonymous) = %d, want 200: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "getMe", http.StatusOK, w)

	// Authenticated: MeResponse.user must validate as a populated MeUser (the
	// oneOf's first branch, a $ref).
	_, attach := doLogin(t, h, "alice", "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	attach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /me (admin session) = %d, want 200: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "getMe", http.StatusOK, w)
}

// --- Error shapes: 401, 403, 404, 400, 422 ---

func TestContractErrorShapes(t *testing.T) {
	doc := specDoc(t)
	h, _ := newNativeAuthRouter(t)

	// 401: no session at all on an admin-gated route. sessionGuard
	// (api_v1.go) answers this before the listUsers handler ever runs, but
	// the spec documents 401 on the operation itself.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/users", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GET /users without session = %d, want 401: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "listUsers", http.StatusUnauthorized, w)

	// 403: authenticated but not admin.
	_, bobAttach := doLogin(t, h, "bob", "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	bobAttach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("GET /users as non-admin = %d, want 403: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "listUsers", http.StatusForbidden, w)

	_, adminAttach := doLogin(t, h, "alice", "secret")

	// 404: a library id that does not exist.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/libraries/999999", nil)
	adminAttach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET /libraries/999999 = %d, want 404: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "getLibrary", http.StatusNotFound, w)

	// 400: malformed JSON body (a request that never gets far enough to be
	// "well-formed but invalid").
	req = httptest.NewRequest(http.MethodPost, "/api/v1/libraries", strings.NewReader(`{`))
	adminAttach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("POST /libraries with malformed JSON = %d, want 400: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "createLibrary", http.StatusBadRequest, w)

	// 422: well-formed but invalid — a name over LibraryCreateRequest's
	// 200-char maxLength — itemising /name via ValidationProblem.
	body := mustJSON(t, map[string]any{"name": strings.Repeat("x", 201), "path": t.TempDir()})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/libraries", bytes.NewReader(body))
	adminAttach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST /libraries with an over-long name = %d, want 422: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "createLibrary", http.StatusUnprocessableEntity, w)
}

// --- Libraries: create-then-list, the Library/LibraryList envelopes ---

func TestContractLibrariesCreateAndList(t *testing.T) {
	doc := specDoc(t)
	h, _ := newNativeAuthRouter(t)
	_, adminAttach := doLogin(t, h, "alice", "secret")

	body := mustJSON(t, map[string]any{"name": "Contract Test Library", "path": t.TempDir()})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/libraries", bytes.NewReader(body))
	adminAttach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /libraries = %d, want 201: %s", w.Code, w.Body.String())
	}
	// The freshly created Library exercises last_scan_started_at:null — one
	// of the spec's `type: [string, 'null']` fields.
	assertJSONResponse(t, doc, "createLibrary", http.StatusCreated, w)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/libraries", nil)
	adminAttach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /libraries = %d, want 200: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "listLibraries", http.StatusOK, w)
}

// --- Users: create-then-list, the User/UserList envelopes ---

func TestContractUsersCreateAndList(t *testing.T) {
	doc := specDoc(t)
	h, _ := newNativeAuthRouter(t)
	_, adminAttach := doLogin(t, h, "alice", "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(`{"login":"contractuser","password":"s3cret-password"}`))
	adminAttach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /users = %d, want 201: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "createUser", http.StatusCreated, w)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	adminAttach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /users = %d, want 200: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "listUsers", http.StatusOK, w)
}

// --- Tokens: the trickiest conditional shapes (apikey vs usertoken,
// nullable lastUsedAt/expiresAt, camelCase) ---

func TestContractTokensCreateAndList(t *testing.T) {
	doc := specDoc(t)
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	// createToken, default type "apikey": CreateTokenResult without
	// username/password (present only for "usertoken").
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(`{"name":"contract-apikey"}`))
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /auth/tokens (apikey) = %d, want 201: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "createToken", http.StatusCreated, w)

	// createToken, type "usertoken": the conditional username/password
	// branch, plus expiresAt:null (omitted in the request).
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(`{"name":"contract-usertoken","type":"usertoken"}`))
	attach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /auth/tokens (usertoken) = %d, want 201: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "createToken", http.StatusCreated, w)

	// mintToken: the camelCase SPA session/device shape.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", strings.NewReader(`{"deviceId":"contract-device"}`))
	attach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /auth/token = %d, want 201: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "mintToken", http.StatusCreated, w)

	// listTokens: both the two just-created client PATs and the live mint
	// session, exercising TokenInfo's nullable lastUsedAt (never used yet)
	// and expiresAt (the apikey token never expires) side by side with the
	// mint session's populated ones.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/tokens", nil)
	attach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /auth/tokens = %d, want 200: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "listTokens", http.StatusOK, w)
}

// TestCreateTokenResultRejectsCredentialTypeMismatch proves the
// CreateTokenResult oneOf split (aether-v1.yaml) is an enforced "iff", not
// merely documented prose: a synthetic "apikey" body wrongly carrying
// username/password is REJECTED (the gap #5 closed — the apikey oneOf
// variant now forbids them via not/anyOf over required, not just omits them
// from its own properties), a synthetic "usertoken" body missing them is
// REJECTED, and a clean body of either shape PASSES. Validates directly
// against the schema with the exact same call assertJSONResponse makes
// (schema.VisitJSON + EnableJSONSchema2020()), so this exercises the actual
// validator/dialect real responses are checked against — kin-openapi's
// EnableJSONSchema2020 path compiles the schema with
// github.com/santhosh-tekuri/jsonschema/v6 (a JSON Schema 2020-12 engine
// that does evaluate if/then/else and every XOF keyword), not the legacy
// pre-3.1 visitJSON/visitXOFOperations code path, which is only a fallback
// used when that compilation itself fails.
func TestCreateTokenResultRejectsCredentialTypeMismatch(t *testing.T) {
	doc := specDoc(t)
	schema := responseSchema(t, doc, "createToken", http.StatusCreated, "application/json")

	valid := func(name string, body map[string]any) {
		t.Run(name, func(t *testing.T) {
			if err := schema.VisitJSON(body, openapi3.EnableJSONSchema2020()); err != nil {
				t.Fatalf("expected a valid CreateTokenResult, got: %v\nbody: %+v", err, body)
			}
		})
	}
	invalid := func(name string, body map[string]any) {
		t.Run(name, func(t *testing.T) {
			if err := schema.VisitJSON(body, openapi3.EnableJSONSchema2020()); err == nil {
				t.Fatalf("expected CreateTokenResult validation to reject this body, but it passed: %+v", body)
			}
		})
	}

	valid("apikey without credentials passes", map[string]any{
		"token": "aether-abc123", "tokenId": "abc123", "name": "contract-test",
		"kind": "client", "type": "apikey",
		"createdAt": "2026-01-01T00:00:00Z", "expiresAt": nil,
	})
	invalid("apikey wrongly carrying username/password is rejected", map[string]any{
		"token": "aether-abc123", "tokenId": "abc123", "name": "contract-test",
		"kind": "client", "type": "apikey",
		"createdAt": "2026-01-01T00:00:00Z", "expiresAt": nil,
		"username": "abc123", "password": "secret",
	})
	valid("usertoken with both credentials passes", map[string]any{
		"token": "aether-abc123", "tokenId": "abc123", "name": "contract-test",
		"kind": "client", "type": "usertoken",
		"createdAt": "2026-01-01T00:00:00Z", "expiresAt": nil,
		"username": "abc123", "password": "secret",
	})
	invalid("usertoken missing username/password is rejected", map[string]any{
		"token": "aether-abc123", "tokenId": "abc123", "name": "contract-test",
		"kind": "client", "type": "usertoken",
		"createdAt": "2026-01-01T00:00:00Z", "expiresAt": nil,
	})
}

// --- Tasks: the flattened-embed + wrapped-vs-bare Task shapes, an empty
// envelope before any run, and the populated one after ---

func TestContractTasksListUpsertGetTriggerAndExecutions(t *testing.T) {
	doc := specDoc(t)
	h, attach := newContractTaskRouter(t)

	// listTasks: the TaskList envelope, entries with no schedule configured
	// yet (Task.schedule entirely absent, per its omitempty allOf).
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /tasks = %d, want 200: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "listTasks", http.StatusOK, w)

	// listTaskExecutions before any run: the empty-list envelope.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tasks/executions", nil)
	attach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /tasks/executions (before any run) = %d, want 200: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "listTaskExecutions", http.StatusOK, w)

	// upsertTaskSchedule: bare Task, now WITH its schedule (TaskSchedule via
	// allOf) — the trickiest shape in this group.
	req = httptest.NewRequest(http.MethodPut, "/api/v1/tasks/scan", strings.NewReader(`{"cron_expression":"0 0 3 * * *"}`))
	attach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT /tasks/scan = %d, want 200: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "upsertTaskSchedule", http.StatusOK, w)

	// getTask: bare Task, reading the schedule just saved back.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tasks/scan", nil)
	attach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /tasks/scan = %d, want 200: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "getTask", http.StatusOK, w)

	// triggerTask: 202 + TriggerTaskResult's execution_id.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/tasks/scan/trigger", nil)
	attach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("POST /tasks/scan/trigger = %d, want 202: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "triggerTask", http.StatusAccepted, w)

	// listTaskExecutions again: now populated by the triggered run, and
	// TaskExecution's ended_at is still its Go zero-time value (the run was
	// never started: no Runner.Start() in newContractTaskRouter).
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tasks/executions", nil)
	attach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /tasks/executions (after trigger) = %d, want 200: %s", w.Code, w.Body.String())
	}
	assertJSONResponse(t, doc, "listTaskExecutions", http.StatusOK, w)
}
