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

	"github.com/go-bumbu/userauth/auth/cookieauth"
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
)

// SPATokenName is the fixed name of SPA-minted tokens.
const SPATokenName = "aether-web"

// SPATokenTTL bounds how long a browser session can go without re-minting.
const SPATokenTTL = 48 * time.Hour

type Handler struct {
	Tokens *pat.Service
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

// caller returns the session's user ID; the guard already authenticated it.
func caller(r *http.Request) (string, bool) {
	data, err := cookieauth.CtxGetUserData(r)
	if err != nil || !data.IsAuthenticated {
		return "", false
	}
	return data.UserId, true
}

func hasScope(rec pat.TokenRecord, scope string) bool {
	for _, s := range rec.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// mintSPAToken creates the SPA's short-lived token. It first sweeps the
// caller's expired spa-scoped tokens: repeated boots must not exhaust the
// per-user cap (mint-time-only sweep, see the spec).
func (h *Handler) mintSPAToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := caller(r)
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
		if hasScope(rec, SPAScope) && rec.ExpiresAt != nil && rec.ExpiresAt.Before(now) {
			// Best-effort: a failed sweep must not block the mint.
			_ = h.Tokens.Revoke(userID, rec.TokenID)
		}
	}
	expiresAt := now.Add(SPATokenTTL)
	token, rec, err := h.Tokens.Mint(userID, SPATokenName, []string{SPAScope}, &expiresAt)
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
	default:
		return http.StatusInternalServerError, "internal"
	}
}

// tokenDTO is the management view of a token: metadata only, never the hash.
type tokenDTO struct {
	TokenID    string     `json:"tokenId"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
}

func toDTO(rec pat.TokenRecord) tokenDTO {
	return tokenDTO{
		TokenID:    rec.TokenID,
		Name:       rec.Name,
		CreatedAt:  rec.CreatedAt,
		LastUsedAt: rec.LastUsedAt,
		ExpiresAt:  rec.ExpiresAt,
	}
}

// list returns the caller's user-created PATs. SPA-minted tokens are
// excluded: they are plumbing, not something the user manages.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	userID, ok := caller(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	recs, err := h.Tokens.List(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]tokenDTO, 0, len(recs))
	for _, rec := range recs {
		if hasScope(rec, SPAScope) {
			continue
		}
		out = append(out, toDTO(rec))
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": out})
}

type createInput struct {
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expiresAt"` // optional; nil = never expires
}

// create mints a user-created PAT (scope "client"). The plaintext appears in
// this response and nowhere else.
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	userID, ok := caller(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var in createInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	token, rec, err := h.Tokens.Mint(userID, in.Name, []string{ClientScope}, in.ExpiresAt)
	if err != nil {
		status, code := mapPatError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":     token,
		"tokenId":   rec.TokenID,
		"name":      rec.Name,
		"createdAt": rec.CreatedAt,
		"expiresAt": rec.ExpiresAt,
	})
}

// revoke deletes the caller's token; foreign and absent IDs are both 404
// (the store cannot tell them apart, deliberately).
func (h *Handler) revoke(w http.ResponseWriter, r *http.Request) {
	userID, ok := caller(r)
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
