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
