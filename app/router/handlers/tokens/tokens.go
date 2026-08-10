// Package tokens exposes the session-scoped token endpoints on /api/v1: the
// SPA's short-lived mint plus CRUD for user-created PATs (Task 2). All
// endpoints operate on the CALLER's tokens only — identity comes from the
// session the /api/v1 guard validated, never from the request. See
// docs/agents/authentication.md.
package tokens

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-bumbu/userauth/service/pat"
	"github.com/gorilla/mux"
)

// Token classification rides the pat library's opaque Scopes field.
const (
	// SPAScope marks tokens the SPA mints for itself: short-lived, hidden
	// from the management UI, swept at mint time.
	SPAScope = "spa"
	// ClientScope marks user-created PATs for third-party Subsonic clients.
	ClientScope = "client"
	// UserTokenScope additionally marks recoverable PATs presented as a virtual
	// username + password pair (Subsonic t+s clients). Tokens without it are
	// hash-only apikeys.
	UserTokenScope = "usertoken"
)

// SPATokenName is the fixed name of SPA-minted tokens.
const SPATokenName = "aether-web"

// SPATokenTTL bounds how long a browser session can go without re-minting.
const SPATokenTTL = 48 * time.Hour

type Handler struct {
	Tokens *pat.Service
	// Caller resolves the request's user ID from whatever identity the
	// /api/v1 guard established (session cookie or proxy headers). The
	// handler itself never branches on the auth mode — it trusts the
	// middleware identity and nothing else (docs/agents/authentication.md).
	Caller func(r *http.Request) (userID string, ok bool)
}

func (h *Handler) Routes(r *mux.Router) {
	r.Path("/auth/token").Methods(http.MethodPost).HandlerFunc(h.mintSPAToken)
	r.Path("/auth/tokens").Methods(http.MethodGet).HandlerFunc(h.list)
	r.Path("/auth/tokens").Methods(http.MethodPost).HandlerFunc(h.create)
	r.Path("/auth/tokens/{tokenId}").Methods(http.MethodDelete).HandlerFunc(h.revoke)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, apiError{Error: msg, Code: code})
}

// caller returns the guard-established user ID via the injected resolver.
func (h *Handler) caller(r *http.Request) (string, bool) {
	if h.Caller == nil {
		return "", false
	}
	return h.Caller(r)
}

func hasScope(rec pat.TokenRecord, scope string) bool {
	for _, s := range rec.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// mintSPAToken creates the SPA's short-lived token. It first sweeps ALL of the
// caller's spa-scoped tokens, expired or live: the SPA holds exactly one, and
// this mint supersedes it, so keeping the old one around only eats into the
// per-user cap. That bounds spa tokens at ~1/user and removes the lockout where
// repeated boots eventually answered 409 ErrTooManyTokens (mint-time-only
// sweep, see the spec).
func (h *Handler) mintSPAToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	recs, err := h.Tokens.List(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	now := time.Now()
	for _, rec := range recs {
		if hasScope(rec, SPAScope) {
			// Best-effort: a failed sweep must not block the mint.
			_ = h.Tokens.Revoke(userID, rec.TokenID)
		}
	}
	expiresAt := now.Add(SPATokenTTL)
	token, rec, err := h.Tokens.Mint(userID, SPATokenName, []string{SPAScope}, &expiresAt, pat.HashOnly)
	if err != nil {
		status, code := mapPatError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":     token,
		"tokenId":   rec.TokenID,
		"expiresAt": rec.ExpiresAt,
	})
}

func mapPatError(err error) (status int, code string) {
	switch {
	case errors.Is(err, pat.ErrInvalidName), errors.Is(err, pat.ErrInvalidExpiry):
		return http.StatusBadRequest, "validation_error"
	case errors.Is(err, pat.ErrTooManyTokens):
		return http.StatusConflict, "too_many_tokens"
	case errors.Is(err, pat.ErrTokenNotFound):
		return http.StatusNotFound, "not_found"
	case errors.Is(err, pat.ErrNoCipher):
		return http.StatusNotImplemented, "usertoken_unavailable"
	default:
		return http.StatusInternalServerError, "internal"
	}
}

// Token kinds as the management endpoints report them.
const (
	// KindSession is a first-party token the SPA minted for itself.
	KindSession = "session"
	// KindClient is a user-created PAT for third-party Subsonic clients.
	KindClient = "client"
)

// Token types as the management endpoints report them.
const (
	// TypeAPIKey is a hash-only PAT presented whole via the apiKey param.
	TypeAPIKey = "apikey"
	// TypeUserToken is a recoverable PAT presented as virtual username
	// (the tokenId) + password (the secret) by Subsonic t+s clients.
	TypeUserToken = "usertoken"
)

// tokenDTO is the management view of a token: metadata only, never the hash.
type tokenDTO struct {
	TokenID    string     `json:"tokenId"`
	Name       string     `json:"name"`
	Kind       string     `json:"kind"`
	Type       string     `json:"type"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
}

func toDTO(rec pat.TokenRecord, kind string) tokenDTO {
	typ := TypeAPIKey
	if hasScope(rec, UserTokenScope) {
		typ = TypeUserToken
	}
	return tokenDTO{
		TokenID:    rec.TokenID,
		Name:       rec.Name,
		Kind:       kind,
		Type:       typ,
		CreatedAt:  rec.CreatedAt,
		LastUsedAt: rec.LastUsedAt,
		ExpiresAt:  rec.ExpiresAt,
	}
}

// list returns the caller's tokens: user-created PATs (kind "client") plus
// live first-party SPA tokens (kind "session"), so the UI can show both and
// tell them apart. Expired session tokens are dropped — they are already
// superseded plumbing, not something the user should see.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	recs, err := h.Tokens.List(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	now := time.Now()
	out := make([]tokenDTO, 0, len(recs))
	for _, rec := range recs {
		kind := KindClient
		if hasScope(rec, SPAScope) {
			if rec.ExpiresAt != nil && !rec.ExpiresAt.After(now) {
				continue
			}
			kind = KindSession
		}
		out = append(out, toDTO(rec, kind))
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": out})
}

type createInput struct {
	Name      string     `json:"name"`
	Type      string     `json:"type"`      // "apikey" (default) or "usertoken"
	ExpiresAt *time.Time `json:"expiresAt"` // optional; nil = never expires
}

// create mints a user-created PAT (scope "client"). The plaintext appears in
// this response and nowhere else.
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var in createInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	if in.Type == "" {
		in.Type = TypeAPIKey
	}
	if in.Type != TypeAPIKey && in.Type != TypeUserToken {
		writeError(w, http.StatusBadRequest, "validation_error", "type must be \"apikey\" or \"usertoken\"")
		return
	}
	scopes := []string{ClientScope}
	storage := pat.HashOnly
	if in.Type == TypeUserToken {
		scopes = append(scopes, UserTokenScope)
		storage = pat.Recoverable
	}
	token, rec, err := h.Tokens.Mint(userID, in.Name, scopes, in.ExpiresAt, storage)
	if err != nil {
		status, code := mapPatError(err)
		writeError(w, status, code, err.Error())
		return
	}
	body := map[string]any{
		"token":     token,
		"tokenId":   rec.TokenID,
		"name":      rec.Name,
		"kind":      KindClient,
		"type":      in.Type,
		"createdAt": rec.CreatedAt,
		"expiresAt": rec.ExpiresAt,
	}
	if in.Type == TypeUserToken {
		// The credential pair third-party apps consume: the tokenId doubles
		// as a virtual username, the secret as the password. ParseToken
		// splits the same plaintext Mint just returned.
		_, secret, ok := pat.ParseToken("aether", token)
		if !ok {
			writeError(w, http.StatusInternalServerError, "internal", "minted token failed to parse")
			return
		}
		body["username"] = rec.TokenID
		body["password"] = secret
	}
	writeJSON(w, http.StatusCreated, body)
}

// revoke deletes the caller's token; foreign and absent IDs are both 404
// (the store cannot tell them apart, deliberately).
func (h *Handler) revoke(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if err := h.Tokens.Revoke(userID, mux.Vars(r)["tokenId"]); err != nil {
		status, code := mapPatError(err)
		writeError(w, status, code, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
