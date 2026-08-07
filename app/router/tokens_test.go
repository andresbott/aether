package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tokensHandler "github.com/andresbott/aether/app/router/handlers/tokens"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"github.com/go-bumbu/userauth/auth/cookieauth"
	"github.com/go-bumbu/userauth/service/pat"
	"github.com/go-bumbu/userauth/userstore/userdb"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type mintResponse struct {
	Token     string    `json:"token"`
	TokenID   string    `json:"tokenId"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type errorEnvelopeRouter struct {
	SubsonicResponse struct {
		Status string `json:"status"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	} `json:"subsonic-response"`
}

func doMint(t *testing.T, h *MainAppHandler, attach func(*http.Request)) mintResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", nil)
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("mint = %d, want 201: %s", w.Code, w.Body.String())
	}
	var body mintResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("mint body: %v", err)
	}
	return body
}

// The mint endpoint requires a session but NOT the admin role: tokens are a
// per-user concern. Anonymous callers get 401 like any guarded route.
func TestMintRequiresSessionNotAdmin(t *testing.T) {
	h, _ := newNativeAuthRouter(t)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("mint without session = %d, want 401", w.Code)
	}

	_, attach := doLogin(t, h, "bob", "secret") // bob is a regular user
	body := doMint(t, h, attach)
	if body.Token == "" || body.TokenID == "" {
		t.Fatalf("mint body = %+v, want token and tokenId", body)
	}
	if until := time.Until(body.ExpiresAt); until < 47*time.Hour || until > 49*time.Hour {
		t.Errorf("expiresAt %v, want ~48h from now", body.ExpiresAt)
	}
}

// The minted token verifies against the pat service and asserts the caller's
// login (owners on /rest are login strings).
func TestMintedTokenVerifies(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")
	body := doMint(t, h, attach)

	info, ok, err := h.tokens.Verify(body.Token)
	if err != nil || !ok {
		t.Fatalf("Verify = ok:%v err:%v, want valid", ok, err)
	}
	if info.LoginID != "bob" {
		t.Errorf("LoginID = %q, want bob", info.LoginID)
	}
	if len(info.Scopes) != 1 || info.Scopes[0] != tokensHandler.SPAScope {
		t.Errorf("Scopes = %v, want [spa]", info.Scopes)
	}
}

// Minting sweeps the caller's EXPIRED spa-scoped tokens so repeated boots
// cannot exhaust the per-user cap. Live spa tokens and client tokens survive.
func TestMintSweepsExpiredSpaTokens(t *testing.T) {
	h, db := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	usr, err := h.users.GetUserByLogin("bob")
	if err != nil {
		t.Fatal(err)
	}
	// Mint rejects past expiries, so mint a live spa token and age it into
	// the past behind the service's back (user_pats is userdb's PAT table).
	future := time.Now().Add(time.Minute)
	_, staleRec, err := h.tokens.Mint(usr.ID, "stale-spa", []string{tokensHandler.SPAScope}, &future)
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := db.Table("user_pats").
		Where("token_id = ?", staleRec.TokenID).
		Update("expires_at", past).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.tokens.Mint(usr.ID, "phone", []string{tokensHandler.ClientScope}, nil); err != nil {
		t.Fatal(err)
	}

	doMint(t, h, attach)

	recs, err := h.tokens.List(usr.ID)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range recs {
		names = append(names, r.Name)
		if r.Name == "stale-spa" {
			t.Fatal("expired spa token survived the mint sweep")
		}
	}
	// stale-spa gone; phone and the fresh aether-web remain.
	if len(recs) != 2 {
		t.Fatalf("tokens after mint = %v, want [phone aether-web]", names)
	}
}

type tokenDTO struct {
	TokenID    string     `json:"tokenId"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
}

// The list shows only user-created PATs: spa-scoped tokens are SPA plumbing.
// The secret hash must never appear in any response.
func TestListTokensExcludesSpaScoped(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")
	doMint(t, h, attach) // creates an aether-web spa token

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens",
		strings.NewReader(`{"name":"Symfonium on phone"}`))
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create = %d, want 201: %s", w.Code, w.Body.String())
	}
	var created struct {
		Token   string `json:"token"`
		TokenID string `json:"tokenId"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || created.Token == "" {
		t.Fatalf("create body = %s, want plaintext token", w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/tokens", nil)
	attach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list = %d, want 200: %s", w.Code, w.Body.String())
	}
	var list struct {
		Tokens []tokenDTO `json:"tokens"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Tokens) != 1 || list.Tokens[0].Name != "Symfonium on phone" {
		t.Fatalf("list = %+v, want only the client token", list.Tokens)
	}
	if strings.Contains(w.Body.String(), "secretHash") || strings.Contains(w.Body.String(), "SecretHash") {
		t.Fatal("token list leaks the secret hash")
	}
}

// Revocation is owner-scoped: alice cannot delete bob's token, and the store
// answers identically for absent and foreign IDs (404).
func TestRevokeTokenOwnerScoped(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, bobAttach := doLogin(t, h, "bob", "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens",
		strings.NewReader(`{"name":"phone"}`))
	bobAttach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var created struct {
		TokenID string `json:"tokenId"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	_, aliceAttach := doLogin(t, h, "alice", "secret")
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/auth/tokens/"+created.TokenID, nil)
	aliceAttach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("foreign revoke = %d, want 404", w.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/auth/tokens/"+created.TokenID, nil)
	bobAttach(req)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("own revoke = %d, want 204", w.Code)
	}
}

// Create validates input: empty name 400, past expiry 400.
func TestCreateTokenValidation(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	for _, body := range []string{
		`{"name":""}`,
		`{"name":"x","expiresAt":"2020-01-01T00:00:00Z"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(body))
		attach(req)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("create %s = %d, want 400", body, w.Code)
		}
	}
}

// Logout revokes the spa token the SPA hands it, so a stolen in-memory token
// dies with the session. Best-effort: logout succeeds regardless.
func TestLogoutRevokesSpaToken(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")
	minted := doMint(t, h, attach)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout",
		strings.NewReader(`{"tokenId":"`+minted.TokenID+`"}`))
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("logout = %d, want 200: %s", w.Code, w.Body.String())
	}

	if _, ok, _ := h.tokens.Verify(minted.Token); ok {
		t.Fatal("spa token still valid after logout")
	}
}

// Logout without a body (or with a bogus tokenId) still succeeds.
func TestLogoutWithoutTokenStillWorks(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("logout without body = %d, want 200", w.Code)
	}
}

// End to end: login → mint → call /rest with apiKey. Pins the full wiring:
// guard tier, mint, resolver, error codes 43/44/40.
func TestRestAuthenticatesWithApiKey(t *testing.T) {
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
	if err := users.Create("bob", "secret"); err != nil {
		t.Fatal(err)
	}
	cookieStore, err := cookieauth.NewCookieStore(make([]byte, 64), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	cookieStore.Options.Secure = false
	sessions, err := cookieauth.New(cookieauth.Cfg{Store: cookieStore, SessionDur: time.Hour, MaxSessionDur: 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := pat.NewService(users.PATStore(), users, pat.Opts{Prefix: "aether"})
	if err != nil {
		t.Fatal(err)
	}
	h, err := New(Cfg{AuthMethod: "native", Users: users, Sessions: sessions, Tokens: tokens,
		Store: store.New(db), DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	_, attach := doLogin(t, h, "bob", "secret")
	minted := doMint(t, h, attach)

	get := func(url string) errorEnvelopeRouter {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		var body errorEnvelopeRouter
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("body %s: %v", w.Body.String(), err)
		}
		return body
	}

	if got := get("/rest/ping.view?apiKey=" + minted.Token); got.SubsonicResponse.Status != "ok" {
		t.Fatalf("valid apiKey: status %q, want ok", got.SubsonicResponse.Status)
	}
	if got := get("/rest/ping.view?apiKey=aether_bogus_bogus"); got.SubsonicResponse.Error == nil || got.SubsonicResponse.Error.Code != 44 {
		t.Fatalf("invalid apiKey: %+v, want code 44", got.SubsonicResponse.Error)
	}
	if got := get("/rest/ping.view?apiKey=" + minted.Token + "&u=bob"); got.SubsonicResponse.Error == nil || got.SubsonicResponse.Error.Code != 43 {
		t.Fatalf("mixed auth: %+v, want code 43", got.SubsonicResponse.Error)
	}
	if got := get("/rest/ping.view"); got.SubsonicResponse.Error == nil || got.SubsonicResponse.Error.Code != 40 {
		t.Fatalf("no credentials: %+v, want code 40", got.SubsonicResponse.Error)
	}
}

// Verifier I/O error (e.g. database unreachable) returns Subsonic code 0, not
// 44, and must not panic when Logger is nil (New defaults to discard handler).
func TestRestVerifierIOErrorReturnsCode0(t *testing.T) {
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
	if err := users.Create("bob", "secret"); err != nil {
		t.Fatal(err)
	}
	cookieStore, err := cookieauth.NewCookieStore(make([]byte, 64), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	cookieStore.Options.Secure = false
	sessions, err := cookieauth.New(cookieauth.Cfg{Store: cookieStore, SessionDur: time.Hour, MaxSessionDur: 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := pat.NewService(users.PATStore(), users, pat.Opts{Prefix: "aether"})
	if err != nil {
		t.Fatal(err)
	}
	// Build router with NO logger to ensure nil-logger safety.
	h, err := New(Cfg{AuthMethod: "native", Users: users, Sessions: sessions, Tokens: tokens,
		Store: store.New(db), DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	_, attach := doLogin(t, h, "bob", "secret")
	minted := doMint(t, h, attach)

	// Close the underlying database to force a PAT Verify failure.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	_ = sqlDB.Close()

	req := httptest.NewRequest(http.MethodGet, "/rest/ping.view?apiKey="+minted.Token, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var body errorEnvelopeRouter
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s: %v", w.Body.String(), err)
	}
	if body.SubsonicResponse.Error == nil || body.SubsonicResponse.Error.Code != 0 {
		t.Fatalf("I/O error: %+v, want code 0", body.SubsonicResponse.Error)
	}
}

// The apiKey masking middleware replaces the value with *** in RequestURI
// (which the logger sees), while leaving r.URL intact so handlers still parse it.
func TestRestApiKeyMaskedInLogs(t *testing.T) {
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
	if err := users.Create("bob", "secret"); err != nil {
		t.Fatal(err)
	}
	cookieStore, err := cookieauth.NewCookieStore(make([]byte, 64), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	cookieStore.Options.Secure = false
	sessions, err := cookieauth.New(cookieauth.Cfg{Store: cookieStore, SessionDur: time.Hour, MaxSessionDur: 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := pat.NewService(users.PATStore(), users, pat.Opts{Prefix: "aether"})
	if err != nil {
		t.Fatal(err)
	}
	h, err := New(Cfg{AuthMethod: "native", Users: users, Sessions: sessions, Tokens: tokens,
		Store: store.New(db), DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	_, attach := doLogin(t, h, "bob", "secret")
	minted := doMint(t, h, attach)

	// Wrap the handler to capture RequestURI as it passes through middleware.
	var capturedURI string
	maskedHandler, _ := New(Cfg{AuthMethod: "native", Users: users, Sessions: sessions, Tokens: tokens,
		Store: store.New(db), DataDir: t.TempDir()})
	// Inject a test middleware at the end to capture RequestURI after masking.
	maskedHandler.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedURI = r.RequestURI
			next.ServeHTTP(w, r)
		})
	})

	reqURL := "/rest/ping.view?apiKey=" + minted.Token
	req := httptest.NewRequest(http.MethodGet, reqURL, nil)
	req.RequestURI = reqURL
	w := httptest.NewRecorder()
	maskedHandler.ServeHTTP(w, req)

	// RequestURI should have *** instead of the real key.
	if !strings.Contains(capturedURI, "apiKey=***") {
		t.Errorf("RequestURI = %q, want apiKey=***", capturedURI)
	}
	if strings.Contains(capturedURI, minted.Token) {
		t.Errorf("RequestURI = %q, contains unmasked token", capturedURI)
	}

	// The handler should authenticate successfully (r.URL intact).
	var body errorEnvelopeRouter
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s: %v", w.Body.String(), err)
	}
	if body.SubsonicResponse.Status != "ok" {
		t.Fatalf("masked apiKey auth: status %q, want ok", body.SubsonicResponse.Status)
	}
}
