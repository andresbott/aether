package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/go-bumbu/userauth/auth/headerauth"
	"github.com/go-bumbu/userauth/service/user"
	"gorm.io/gorm"
)

const testAdminGroup = "aether-admin"

// newProxyAuthRouter builds a router in the shape proxy-header mode always
// has in production: a user store and PAT service (shared with native) plus
// the header handler — no sessions, no login endpoints, no users CRUD.
func newProxyAuthRouter(t *testing.T, trusted []netip.Prefix) (*MainAppHandler, *user.Service) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	users, _, tokens := newTestAuthStores(t, db, nil)
	headerAuth := headerauth.New(headerauth.Cfg{
		UserHeader:     "Remote-User",
		GroupsHeader:   "Remote-Groups",
		ParseGroups:    true,
		TrustedProxies: trusted,
	})
	h, err := New(Cfg{
		AuthMethod: "proxy-header",
		Users:      users,
		Tokens:     tokens,
		HeaderAuth: headerAuth,
		AdminGroup: testAdminGroup,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h, users
}

// proxyReq builds a request carrying proxy-asserted identity headers.
func proxyReq(method, path, user, groups string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	if user != "" {
		r.Header.Set("Remote-User", user)
	}
	if groups != "" {
		r.Header.Set("Remote-Groups", groups)
	}
	return r
}

// proxyMintReq builds the spa-token mint the SPA sends in proxy mode: a
// proxy-authenticated request naming the browser it mints for.
func proxyMintReq(user string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token",
		strings.NewReader(`{"deviceId":"test-browser","deviceName":"Test Browser"}`))
	r.Header.Set("Remote-User", user)
	return r
}

func TestHeaderGuardTiers(t *testing.T) {
	h, _ := newProxyAuthRouter(t, nil)

	// Public bootstrap set stays reachable without identity headers.
	for _, path := range []string{"/api/v1/me", "/api/v1/health", "/api/v1/version"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Errorf("GET %s without headers = %d, want 200 (public)", path, w.Code)
		}
	}

	// Protected route without identity headers answers 401.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/libraries", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/libraries without headers = %d, want 401: %s", w.Code, w.Body.String())
	}

	// Session-scoped tier: any authenticated identity may mint tokens.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, proxyMintReq("bob"))
	if w.Code != http.StatusCreated {
		t.Fatalf("mint as regular user = %d, want 201: %s", w.Code, w.Body.String())
	}

	// Admin default: a non-admin identity answers 403...
	w = httptest.NewRecorder()
	h.ServeHTTP(w, proxyReq(http.MethodGet, "/api/v1/tasks", "bob", "some-group"))
	if w.Code != http.StatusForbidden {
		t.Fatalf("GET /api/v1/tasks as regular user = %d, want 403: %s", w.Code, w.Body.String())
	}
	// ...and membership in the admin group grants access. (No task runner is
	// wired in this test router, so the route is absent and the guard passing
	// shows as the 400 catch-all rather than 401/403.)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, proxyReq(http.MethodGet, "/api/v1/tasks", "alice", "other,"+testAdminGroup))
	if w.Code == http.StatusForbidden || w.Code == http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/tasks as admin = %d, want the guard to pass: %s", w.Code, w.Body.String())
	}
}

// The login/logout endpoints and the users CRUD are native-only; in proxy
// mode they are not mounted, so they fall through to the 400 catch-all
// (never 200, never a 5xx panic).
func TestProxyModeMountsNoNativeEndpoints(t *testing.T) {
	h, _ := newProxyAuthRouter(t, nil)
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/auth/login"},
		{http.MethodPost, "/api/v1/auth/logout"},
		{http.MethodGet, "/api/v1/users"},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, proxyReq(tc.method, tc.path, "alice", testAdminGroup))
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s %s in proxy mode = %d, want 400 (not mounted)", tc.method, tc.path, w.Code)
		}
	}
}

func TestHeaderGuardIgnoresUntrustedPeer(t *testing.T) {
	// httptest.NewRequest sets RemoteAddr to 192.0.2.1:1234; trust only
	// 10.0.0.0/8 so the request's identity headers must be ignored.
	h, _ := newProxyAuthRouter(t, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})

	w := httptest.NewRecorder()
	h.ServeHTTP(w, proxyReq(http.MethodGet, "/api/v1/libraries", "alice", testAdminGroup))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("spoofed identity from untrusted peer = %d, want 401: %s", w.Code, w.Body.String())
	}

	// /me reports anonymous rather than the spoofed identity.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, proxyReq(http.MethodGet, "/api/v1/me", "alice", testAdminGroup))
	var body struct {
		User any `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("/me body is not JSON: %s", w.Body.String())
	}
	if body.User != nil {
		t.Fatalf("/me user from untrusted peer = %v, want null", body.User)
	}
}

// First sight of a new identity provisions a user row; the second request
// resolves to the same row (no duplicates).
func TestHeaderGuardProvisionsUserOnFirstSight(t *testing.T) {
	h, users := newProxyAuthRouter(t, nil)

	if _, err := users.GetUserByLogin("carol"); err == nil {
		t.Fatal("carol exists before her first request")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, proxyReq(http.MethodGet, "/api/v1/me", "carol", ""))
	if w.Code != http.StatusOK {
		t.Fatalf("first /me = %d, want 200", w.Code)
	}
	first, err := users.GetUserByLogin("carol")
	if err != nil {
		t.Fatalf("carol was not provisioned: %v", err)
	}
	if !first.Enabled {
		t.Fatal("provisioned user is not enabled")
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, proxyMintReq("carol"))
	if w.Code != http.StatusCreated {
		t.Fatalf("mint after provisioning = %d, want 201: %s", w.Code, w.Body.String())
	}
	second, err := users.GetUserByLogin("carol")
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("second request resolved to a different user: %s != %s", second.ID, first.ID)
	}
}

// The DB Enabled flag is aether's kill-switch: a disabled user stays locked
// out even while the proxy still authenticates them.
func TestHeaderGuardBlocksDisabledUser(t *testing.T) {
	h, users := newProxyAuthRouter(t, nil)

	// Provision dave, then disable him.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, proxyReq(http.MethodGet, "/api/v1/me", "dave", ""))
	usr, err := users.GetUserByLogin("dave")
	if err != nil {
		t.Fatal(err)
	}
	if err := users.SetEnabled(usr.ID, false); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, proxyMintReq("dave"))
	if w.Code != http.StatusForbidden {
		t.Fatalf("mint as disabled user = %d, want 403: %s", w.Code, w.Body.String())
	}

	// /me reports anonymous for a disabled identity.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, proxyReq(http.MethodGet, "/api/v1/me", "dave", ""))
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

// /me reports identity, role from the groups header, the proxy-header method
// and userManagement=false (users live at the proxy's IdP).
func TestProxyMeReportsIdentityAndFeatures(t *testing.T) {
	h, _ := newProxyAuthRouter(t, nil)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, proxyReq(http.MethodGet, "/api/v1/me", "alice", testAdminGroup))
	var body struct {
		AuthMethod string `json:"authMethod"`
		User       *struct {
			Login string `json:"login"`
			Role  string `json:"role"`
		} `json:"user"`
		Features struct {
			UserManagement bool `json:"userManagement"`
		} `json:"features"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("/me body is not JSON: %s", w.Body.String())
	}
	if body.AuthMethod != "proxy-header" {
		t.Errorf("authMethod = %q, want proxy-header", body.AuthMethod)
	}
	if body.User == nil || body.User.Login != "alice" || body.User.Role != "admin" {
		t.Errorf("/me user = %+v, want alice with role admin", body.User)
	}
	if body.Features.UserManagement {
		t.Error("features.userManagement = true, want false in proxy mode")
	}

	// Role is read live from the headers: the same user without the admin
	// group is a regular user on the very next request.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, proxyReq(http.MethodGet, "/api/v1/me", "alice", "listeners"))
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("/me body is not JSON: %s", w.Body.String())
	}
	if body.User == nil || body.User.Role != "user" {
		t.Errorf("/me without admin group = %+v, want role user", body.User)
	}
}

// A token minted through the header-authorized endpoint must verify on /rest
// via apiKey, and /rest must keep ignoring identity headers entirely.
func TestProxyMintedTokenWorksOnRest(t *testing.T) {
	h, _ := newProxyAuthRouter(t, nil)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, proxyMintReq("erin"))
	if w.Code != http.StatusCreated {
		t.Fatalf("mint = %d, want 201: %s", w.Code, w.Body.String())
	}
	var mint struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &mint); err != nil || mint.Token == "" {
		t.Fatalf("mint body = %s, want a token", w.Body.String())
	}

	info, ok, err := h.tokens.Verify(mint.Token)
	if err != nil || !ok {
		t.Fatalf("Verify(minted token) = %v, %v, want ok", ok, err)
	}
	if info.LoginID != "erin" {
		t.Fatalf("token resolves to %q, want erin", info.LoginID)
	}

	// Identity headers alone never authenticate /rest: without apiKey the
	// resolver answers Subsonic error 40 regardless of the headers.
	resolver := h.patIdentityResolver()
	if resolver == nil {
		t.Fatal("patIdentityResolver is nil with Tokens set")
	}
	r := proxyReq(http.MethodGet, "/rest/ping", "erin", testAdminGroup)
	if login, code := resolver(r); login != "" || code != 40 {
		t.Fatalf("resolver with headers but no apiKey = (%q, %d), want (\"\", 40)", login, code)
	}
}
