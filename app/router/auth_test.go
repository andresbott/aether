package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andresbott/aether/app/router/handlers/httperr"
	usersHandler "github.com/andresbott/aether/app/router/handlers/users"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/taskrunner"
	"github.com/glebarez/sqlite"
	"github.com/go-bumbu/userauth/auth/cookieauth"
	"github.com/go-bumbu/userauth/service/pat"
	"github.com/go-bumbu/userauth/userstore/userdb"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// routerOpt configures an optional piece of newNativeAuthRouter's Cfg that
// most callers don't need — today, only the task-runner group does (see
// withTaskRunner).
type routerOpt func(t *testing.T, cfg *Cfg, db *gorm.DB)

// withTaskRunner wires a task runner and schedule store into Cfg — the one
// piece newNativeAuthRouter otherwise leaves unset, and attachApiV1
// (api_v1.go) gates the entire /tasks group on a non-nil task runner. Runner
// tasks are never actually started (no Runner.Start()/RegisterTask call):
// AddRun only enqueues by name (internal/taskrunner, github.com/go-bumbu/
// tempo's TaskQueue.Add), so a triggered task's immediate HTTP response
// needs no registered task function.
func withTaskRunner(t *testing.T, cfg *Cfg, db *gorm.DB) {
	t.Helper()
	runner, err := taskrunner.NewRunner(taskrunner.Cfg{DB: db})
	if err != nil {
		t.Fatal(err)
	}
	scheduleStore, err := taskrunner.NewScheduleStore(db)
	if err != nil {
		t.Fatal(err)
	}
	cfg.TaskRunner = runner
	cfg.ScheduleStore = scheduleStore
}

// newNativeAuthRouter builds a router in the shape native mode always has in
// production: a user store AND a cookie session manager, so the /api/v1
// session guard is installed. Admin alice/secret and regular user bob/secret
// exist. opts wires additional optional Cfg pieces some callers need (see
// withTaskRunner); most callers pass none.
func newNativeAuthRouter(t *testing.T, opts ...routerOpt) (*MainAppHandler, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	users, err := userdb.New(db, userdb.Opts{BcryptDifficulty: bcrypt.MinCost, DefaultEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := users.CreateUser(userdb.User{LoginID: "alice", Pw: "secret", Enabled: true, Groups: []string{usersHandler.AdminGroup}}); err != nil {
		t.Fatal(err)
	}
	if err := users.Create("bob", "secret"); err != nil {
		t.Fatal(err)
	}
	cookieStore, err := cookieauth.NewCookieStore(make([]byte, 64), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	cookieStore.Options.Secure = false
	sessions, err := cookieauth.New(cookieauth.Cfg{
		Store:         cookieStore,
		SessionDur:    time.Hour,
		MaxSessionDur: 24 * time.Hour,
		AllowRenew:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := pat.NewAESGCMCipher(bytes.Repeat([]byte{0x42}, 32), "k1")
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := pat.NewService(users.PATStore(), users, pat.Opts{Prefix: "aether", Cipher: cipher})
	if err != nil {
		t.Fatal(err)
	}
	cfg := Cfg{
		AuthMethod: "native",
		Users:      users,
		Sessions:   sessions,
		Tokens:     tokens,
		Store:      store.New(db),
		DataDir:    t.TempDir(),
	}
	for _, opt := range opts {
		opt(t, &cfg, db)
	}
	h, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return h, db
}

// doLogin posts credentials and returns the response; on success the session
// cookies are copied onto every request made through the returned attach func.
func doLogin(t *testing.T, h *MainAppHandler, username, password string) (*httptest.ResponseRecorder, func(r *http.Request)) {
	t.Helper()
	body := strings.NewReader(`{"username":"` + username + `","password":"` + password + `","sessionRenew":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	cookies := w.Result().Cookies()
	return w, func(r *http.Request) {
		for _, c := range cookies {
			r.AddCookie(c)
		}
	}
}

func TestLoginSetsSessionCookie(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	w, _ := doLogin(t, h, "alice", "secret")

	if w.Code != http.StatusOK {
		t.Fatalf("login = %d, want 200: %s", w.Code, w.Body.String())
	}
	var body struct {
		Done bool `json:"done"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || !body.Done {
		t.Fatalf("login body = %s, want done:true", w.Body.String())
	}
	if len(w.Result().Cookies()) == 0 {
		t.Fatal("login did not set a session cookie")
	}
}

// Unknown user, disabled user and wrong password must all answer the same
// uniform 401 — the flow engine guarantees it, this pins the wiring.
func TestLoginRejectsBadCredentials(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	for _, tc := range []struct{ user, pw string }{
		{"alice", "wrong"},
		{"nobody", "secret"},
	} {
		w, _ := doLogin(t, h, tc.user, tc.pw)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("login %s/%s = %d, want 401", tc.user, tc.pw, w.Code)
		}
	}
}

func TestSessionGuardBlocksApiV1(t *testing.T) {
	h, _ := newNativeAuthRouter(t)

	// Without a session, a protected route answers 401 as a problem+json
	// envelope — sessionGuard builds it directly via httperr.Write.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/users", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/users without session = %d, want 401: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
	var envelope httperr.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || httperr.Slug(envelope.Type) != "unauthorized" {
		t.Errorf("401 body = %s, want a Problem with slug unauthorized", w.Body.String())
	}

	// The public bootstrap set stays reachable.
	for _, path := range []string{"/api/v1/me", "/api/v1/health", "/api/v1/version"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Errorf("GET %s without session = %d, want 200 (public)", path, w.Code)
		}
	}

	// With an admin session the same protected route answers.
	_, attach := doLogin(t, h, "alice", "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	attach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/users with admin session = %d, want 200: %s", w.Code, w.Body.String())
	}
}

// Everything /api/v1 protects is server administration, so a valid session
// without the admin role answers 403 — authenticated is not enough.
func TestSessionGuardRequiresAdmin(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	for _, path := range []string{"/api/v1/users", "/api/v1/libraries", "/api/v1/tasks"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		attach(req)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("GET %s as regular user = %d, want 403: %s", path, w.Code, w.Body.String())
		}
	}

	// The public bootstrap set stays reachable for non-admins; /me still
	// reports who they are.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/me as regular user = %d, want 200", w.Code)
	}
	var body struct {
		User *struct {
			Login string `json:"login"`
			Role  string `json:"role"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("/me body is not JSON: %s", w.Body.String())
	}
	if body.User == nil || body.User.Role != "user" {
		t.Fatalf("/me user = %+v, want role user", body.User)
	}
}

// /me reports the session's identity so the SPA knows who is logged in, and
// goes back to null after logout.
func TestMeReflectsSession(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "alice", "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var body struct {
		User *struct {
			Login string `json:"login"`
			Role  string `json:"role"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("/me body is not JSON: %s", w.Body.String())
	}
	if body.User == nil || body.User.Login != "alice" || body.User.Role != "admin" {
		t.Fatalf("/me user = %+v, want alice with role admin", body.User)
	}

	// Logout clears the session; the cleared cookie replaces the old one.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	attach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("logout = %d, want 200: %s", w.Code, w.Body.String())
	}
	loggedOut := w.Result().Cookies()

	req = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	for _, c := range loggedOut {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	body.User = nil
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("/me body is not JSON: %s", w.Body.String())
	}
	if body.User != nil {
		t.Fatalf("/me user after logout = %v, want null", body.User)
	}
}

// A disabled user must not be able to log in even with the right password.
func TestLoginRejectsDisabledUser(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	usr, err := h.users.GetUserByLogin("alice")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.users.SetEnabled(usr.ID, false); err != nil {
		t.Fatal(err)
	}
	w, _ := doLogin(t, h, "alice", "secret")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("login disabled user = %d, want 401", w.Code)
	}
}

// A session whose user has been deleted is worthless: /me answers null and
// the SPA shows the login view.
func TestSessionOfDeletedUserIsAnonymous(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "alice", "secret")
	usr, err := h.users.GetUserByLogin("alice")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.users.Delete(usr.ID); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var body struct {
		User any `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("/me body is not JSON: %s", w.Body.String())
	}
	if body.User != nil {
		t.Fatalf("/me user for deleted account = %v, want null", body.User)
	}
}

// Disabling a user is aether's kill-switch, so it must also close the sessions
// that user already holds — otherwise an open admin session survives the disable
// and can re-enable the account through the very API it should have lost. The
// proxy-header guard has always enforced this (TestProxyDisabledUser); the
// native guard must match.
func TestSessionGuardBlocksDisabledUser(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "alice", "secret")

	// Sanity: the session works before the disable, so a later 401 is caused by
	// the disable and not by a broken login.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("enabled admin session = %d before the disable, test setup is broken", w.Code)
	}

	usr, err := h.users.GetUserByLogin("alice")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.users.SetEnabled(usr.ID, false); err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	attach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("disabled user on an admin route = %d, want 403", w.Code)
	}

	// The session-scoped tier is guarded too: a disabled user must not be able
	// to mint a fresh /rest token and keep streaming.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", nil)
	attach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("disabled user minting a token = %d, want 403", w.Code)
	}
}

// /me is public-tier, so it resolves independently of the guard: a disabled
// user's session must read as anonymous there rather than 403, which is what
// makes the SPA fall back to the login view.
func TestMeIsAnonymousForDisabledUser(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "alice", "secret")
	usr, err := h.users.GetUserByLogin("alice")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.users.SetEnabled(usr.ID, false); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/me = %d, want 200", w.Code)
	}
	var body struct {
		User any `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("/me body is not JSON: %s", w.Body.String())
	}
	if body.User != nil {
		t.Fatalf("/me user for disabled account = %v, want null", body.User)
	}
}

// Native mode without Tokens silently opens /rest (nil resolver → auth "none").
// Fail closed instead: New errors when AuthMethod is native and Tokens is nil.
func TestNativeModeRequiresTokens(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	users, err := userdb.New(db, userdb.Opts{BcryptDifficulty: bcrypt.MinCost, DefaultEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	store, err := cookieauth.NewCookieStore(make([]byte, 64), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	store.Options.Secure = false
	sessions, err := cookieauth.New(cookieauth.Cfg{
		Store:         store,
		SessionDur:    time.Hour,
		MaxSessionDur: 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = New(Cfg{AuthMethod: "native", Users: users, Sessions: sessions, Tokens: nil})
	if err == nil {
		t.Fatal("New with native + nil Tokens succeeded, want error")
	}
}
