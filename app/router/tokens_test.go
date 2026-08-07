package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	tokensHandler "github.com/andresbott/aether/app/router/handlers/tokens"
)

type mintResponse struct {
	Token     string    `json:"token"`
	TokenID   string    `json:"tokenId"`
	ExpiresAt time.Time `json:"expiresAt"`
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
