package router

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
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

// doMint mints for a generic test app instance; tests that care about the
// device identity use doMintDevice.
func doMint(t *testing.T, h *MainAppHandler, attach func(*http.Request)) mintResponse {
	t.Helper()
	return doMintDevice(t, h, attach, "test-browser", "Test Browser")
}

// doMintDevice mints on behalf of one first-party app instance, as the SPA does.
func doMintDevice(t *testing.T, h *MainAppHandler, attach func(*http.Request), deviceID, deviceName string) mintResponse {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"deviceId": deviceID, "deviceName": deviceName})
	if err != nil {
		t.Fatal(err)
	}
	return mintBody(t, h, attach, payload)
}

func mintBody(t *testing.T, h *MainAppHandler, attach func(*http.Request), payload []byte) mintResponse {
	t.Helper()
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", reader)
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

// listTokenDTOs returns what the management endpoint reports for the caller.
func listTokenDTOs(t *testing.T, h *MainAppHandler, attach func(*http.Request)) []tokenDTO {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/tokens", nil)
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list = %d, want 200: %s", w.Code, w.Body.String())
	}
	var body struct {
		Tokens []tokenDTO `json:"tokens"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Tokens
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
	// spa marks it as first-party plumbing; the device scope binds it to one app.
	want := []string{tokensHandler.SPAScope, tokensHandler.DeviceScopePrefix + "test-browser"}
	if !reflect.DeepEqual(info.Scopes, want) {
		t.Errorf("Scopes = %v, want %v", info.Scopes, want)
	}
}

// Minting sweeps the caller's EXPIRED spa tokens (whatever device they came
// from) but leaves other devices' live sessions alone; client tokens survive too.
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
	_, staleRec, err := h.tokens.Mint(usr.ID, "stale-spa", []string{tokensHandler.SPAScope}, &future, pat.HashOnly)
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := db.Table("user_pats").
		Where("token_id = ?", staleRec.TokenID).
		Update("expires_at", past).Error; err != nil {
		t.Fatal(err)
	}
	// A LIVE spa token belonging to ANOTHER app instance: that session stays.
	liveExpiry := time.Now().Add(time.Hour)
	if _, _, err := h.tokens.Mint(usr.ID, "live-spa",
		[]string{tokensHandler.SPAScope, tokensHandler.DeviceScopePrefix + "other-browser"},
		&liveExpiry, pat.HashOnly); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.tokens.Mint(usr.ID, "phone", []string{tokensHandler.ClientScope}, nil, pat.HashOnly); err != nil {
		t.Fatal(err)
	}

	doMintDevice(t, h, attach, "this-browser", "Firefox on Linux")

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
	// stale-spa gone; live-spa, phone and the fresh session remain.
	if len(recs) != 3 {
		t.Fatalf("tokens after mint = %v, want [live-spa phone Firefox on Linux]", names)
	}
}

// The bug this fixes: signing in from a second app instance must not sign the
// first one out. Each identifies itself with its own deviceId, so both sessions
// stay live and both are listed.
func TestMintKeepsOtherDevicesSessions(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	first := doMintDevice(t, h, attach, "browser-a", "Firefox on Linux")
	second := doMintDevice(t, h, attach, "browser-b", "Chrome on Android")

	if _, ok, err := h.tokens.Verify(first.Token); err != nil || !ok {
		t.Fatalf("first browser's token = ok:%v err:%v after the second signed in, want valid", ok, err)
	}
	if _, ok, err := h.tokens.Verify(second.Token); err != nil || !ok {
		t.Fatalf("second browser's token = ok:%v err:%v, want valid", ok, err)
	}

	var sessions []string
	for _, tok := range listTokenDTOs(t, h, attach) {
		if tok.Kind == tokensHandler.KindSession {
			sessions = append(sessions, tok.Name)
		}
	}
	if len(sessions) != 2 {
		t.Fatalf("listed sessions = %v, want one per app instance", sessions)
	}
}

// An app instance holds exactly one session: re-minting from the same device
// supersedes its own previous token instead of piling up rows.
func TestMintReplacesSameDeviceSession(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	old := doMintDevice(t, h, attach, "browser-a", "Firefox on Linux")
	fresh := doMintDevice(t, h, attach, "browser-a", "Firefox on Linux")

	if _, ok, _ := h.tokens.Verify(old.Token); ok {
		t.Error("the device's previous token still verifies after re-minting")
	}
	if _, ok, err := h.tokens.Verify(fresh.Token); err != nil || !ok {
		t.Fatalf("fresh token = ok:%v err:%v, want valid", ok, err)
	}
	var sessions []tokenDTO
	for _, tok := range listTokenDTOs(t, h, attach) {
		if tok.Kind == tokensHandler.KindSession {
			sessions = append(sessions, tok)
		}
	}
	if len(sessions) != 1 || sessions[0].TokenID != fresh.TokenID {
		t.Fatalf("sessions = %+v, want only %s", sessions, fresh.TokenID)
	}
}

// The device name is what the user recognises in the sessions list.
func TestMintNamesSessionAfterDevice(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	doMintDevice(t, h, attach, "browser-a", "Chrome on Android")

	for _, tok := range listTokenDTOs(t, h, attach) {
		if tok.Kind == tokensHandler.KindSession {
			if tok.Name != "Chrome on Android" {
				t.Fatalf("session name = %q, want the device name", tok.Name)
			}
			return
		}
	}
	t.Fatal("no session token listed")
}

// An unnamed device still gets a session, under the generic SPA name.
func TestMintWithoutDeviceNameFallsBackToSPAName(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	doMintDevice(t, h, attach, "browser-a", "")

	for _, tok := range listTokenDTOs(t, h, attach) {
		if tok.Kind == tokensHandler.KindSession {
			if tok.Name != tokensHandler.SPATokenName {
				t.Fatalf("session name = %q, want %q", tok.Name, tokensHandler.SPATokenName)
			}
			return
		}
	}
	t.Fatal("no session token listed")
}

// The deviceId lands in a scope string and decides whose session is superseded,
// so it is mandatory and constrained; anything else is a 400.
func TestMintRejectsMissingOrMalformedDeviceID(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	bodies := []string{
		"",                                  // no body at all
		`{}`,                                // no deviceId
		`{"deviceId":""}`,                   // empty
		`{"deviceId":"has spaces"}`,         // illegal character
		`{"deviceId":"device:with:colons"}`, // would forge a scope
		`{"deviceId":"` + strings.Repeat("a", 65) + `"}`, // over-long
	}
	for _, body := range bodies {
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", reader)
		attach(req)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("mint %q = %d, want 400", body, w.Code)
		}
	}
}

// An over-long device name is truncated rather than rejected: the name is
// cosmetic (pat caps it at 100 runes) and a mint failure would blank the player.
func TestMintTruncatesOverLongDeviceName(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	doMintDevice(t, h, attach, "browser-a", strings.Repeat("x", 300))

	for _, tok := range listTokenDTOs(t, h, attach) {
		if tok.Kind == tokensHandler.KindSession {
			if n := len([]rune(tok.Name)); n != 100 {
				t.Fatalf("session name length = %d, want 100", n)
			}
			return
		}
	}
	t.Fatal("no session token listed")
}

// Repeated boots never lock the user out: each mint frees that device's
// previous spa token before minting the next.
func TestRepeatedMintsNeverExhaustCap(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	usr, err := h.users.GetUserByLogin("bob")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 30; i++ {
		doMintDevice(t, h, attach, "browser-a", "Firefox on Linux")
	}
	recs, err := h.tokens.List(usr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("tokens after 30 mints = %d, want 1", len(recs))
	}
}

// Per-device sessions must not eat the whole per-user token budget: a new app
// instance evicts the least recently seen session once the ceiling is reached,
// so the user's own client PATs stay mintable.
func TestMintEvictsOldestSessionAtCeiling(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	usr, err := h.users.GetUserByLogin("bob")
	if err != nil {
		t.Fatal(err)
	}
	first := doMintDevice(t, h, attach, "browser-0", "browser 0")
	for i := 1; i < tokensHandler.MaxSessionsPerUser+1; i++ {
		doMintDevice(t, h, attach, "browser-"+strconv.Itoa(i), "browser "+strconv.Itoa(i))
	}
	last := doMintDevice(t, h, attach, "browser-last", "browser last")

	recs, err := h.tokens.List(usr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != tokensHandler.MaxSessionsPerUser {
		t.Fatalf("sessions = %d, want the ceiling %d", len(recs), tokensHandler.MaxSessionsPerUser)
	}
	if _, ok, _ := h.tokens.Verify(first.Token); ok {
		t.Error("the oldest session survived past the ceiling")
	}
	if _, ok, err := h.tokens.Verify(last.Token); err != nil || !ok {
		t.Fatalf("newest session = ok:%v err:%v, want valid", ok, err)
	}
}

type tokenDTO struct {
	TokenID    string     `json:"tokenId"`
	Name       string     `json:"name"`
	Kind       string     `json:"kind"`
	Type       string     `json:"type"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
}

// The list carries both token kinds, distinguished by "kind": the user's PATs
// as "client" and live SPA-minted tokens as "session". The secret hash must
// never appear in any response.
func TestListTokensReportsKinds(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")
	doMintDevice(t, h, attach, "browser-a", "Firefox on Linux") // creates a session token

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
	kinds := map[string]string{}
	for _, tok := range list.Tokens {
		kinds[tok.Name] = tok.Kind
	}
	if len(list.Tokens) != 2 {
		t.Fatalf("list = %+v, want the client token and the spa session", list.Tokens)
	}
	if kinds["Symfonium on phone"] != "client" {
		t.Fatalf("client token kind = %q, want client", kinds["Symfonium on phone"])
	}
	if kinds["Firefox on Linux"] != "session" {
		t.Fatalf("spa token kind = %q, want session", kinds["Firefox on Linux"])
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

// newRestAuthRouter builds a native-mode router with the music store attached,
// so /rest is registered and PAT auth can be exercised end to end. Only bob
// exists (a regular user: /rest is not an admin surface).
func newRestAuthRouter(t *testing.T) (*MainAppHandler, *gorm.DB) {
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
	return h, db
}

// restPing calls /rest/ping.view with the given query and returns the envelope.
func restPing(t *testing.T, h *MainAppHandler, query string) errorEnvelopeRouter {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/rest/ping.view?"+query, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var body errorEnvelopeRouter
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s: %v", w.Body.String(), err)
	}
	return body
}

// An EXPIRED token is no longer a credential: /rest answers 44, exactly like a
// forged one. Aged behind the service's back (Mint rejects past expiries).
func TestRestRejectsExpiredToken(t *testing.T) {
	h, db := newRestAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")
	minted := doMint(t, h, attach)

	if got := restPing(t, h, "apiKey="+minted.Token); got.SubsonicResponse.Status != "ok" {
		t.Fatalf("fresh apiKey: status %q, want ok", got.SubsonicResponse.Status)
	}

	past := time.Now().Add(-time.Hour)
	if err := db.Table("user_pats").
		Where("token_id = ?", minted.TokenID).
		Update("expires_at", past).Error; err != nil {
		t.Fatal(err)
	}

	got := restPing(t, h, "apiKey="+minted.Token)
	if got.SubsonicResponse.Error == nil || got.SubsonicResponse.Error.Code != 44 {
		t.Fatalf("expired apiKey: %+v, want code 44", got.SubsonicResponse.Error)
	}
}

// Disabling a user must kill their tokens too, or a revoked account keeps
// streaming: /rest answers 44 for a token whose owner is disabled.
func TestRestRejectsTokenOfDisabledOwner(t *testing.T) {
	h, _ := newRestAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")
	minted := doMint(t, h, attach)

	if got := restPing(t, h, "apiKey="+minted.Token); got.SubsonicResponse.Status != "ok" {
		t.Fatalf("enabled owner: status %q, want ok", got.SubsonicResponse.Status)
	}

	usr, err := h.users.GetUserByLogin("bob")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.users.SetEnabled(usr.ID, false); err != nil {
		t.Fatal(err)
	}

	got := restPing(t, h, "apiKey="+minted.Token)
	if got.SubsonicResponse.Error == nil || got.SubsonicResponse.Error.Code != 44 {
		t.Fatalf("disabled owner: %+v, want code 44", got.SubsonicResponse.Error)
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

func TestCreateUserTokenReturnsCredentialPair(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	body := strings.NewReader(`{"name":"phone","type":"usertoken"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", body)
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	var res struct {
		Token    string `json:"token"`
		TokenID  string `json:"tokenId"`
		Username string `json:"username"`
		Password string `json:"password"`
		Type     string `json:"type"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Type != "usertoken" {
		t.Errorf("type = %q, want usertoken", res.Type)
	}
	if res.Username != res.TokenID || res.Username == "" {
		t.Errorf("username %q must equal tokenId %q", res.Username, res.TokenID)
	}
	if res.Username != strings.ToLower(res.Username) {
		t.Errorf("username %q must be lowercase", res.Username)
	}
	if res.Token != "aether_"+res.Username+"_"+res.Password {
		t.Errorf("token %q must be the joined credential pair", res.Token)
	}
}

func TestCreateApikeyTokenOmitsCredentialPair(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	body := strings.NewReader(`{"name":"scripted"}`) // type omitted = apikey
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", body)
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	var res map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res["type"] != "apikey" {
		t.Errorf("type = %v, want apikey", res["type"])
	}
	if _, has := res["username"]; has {
		t.Error("apikey response must not carry a username field")
	}
	if _, has := res["password"]; has {
		t.Error("apikey response must not carry a password field")
	}
}

func TestCreateTokenRejectsUnknownType(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	body := strings.NewReader(`{"name":"x","type":"banana"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", body)
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestListReportsTokenType(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	for _, payload := range []string{`{"name":"a","type":"apikey"}`, `{"name":"b","type":"usertoken"}`} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(payload))
		attach(req)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %s: status %d", payload, w.Code)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/tokens", nil)
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var res struct {
		Tokens []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	types := map[string]string{}
	for _, tok := range res.Tokens {
		types[tok.Name] = tok.Type
	}
	if types["a"] != "apikey" || types["b"] != "usertoken" {
		t.Errorf("list types = %v", types)
	}
}
