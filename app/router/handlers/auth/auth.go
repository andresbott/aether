// Package auth mounts the native-login endpoints on /api/v1: a JSON login
// that establishes the aether session cookie and a logout that clears it.
// Only registered with auth method "native" — with "none" there is no user
// store, no session manager, and no login to offer. See
// docs/agents/authentication.md (mode: builtin).
package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-bumbu/userauth/auth/cookieauth"
	loginflow "github.com/go-bumbu/userauth/flow/login"
	loginhandlers "github.com/go-bumbu/userauth/flow/login/handlers"
	"github.com/go-bumbu/userauth/service/pat"
	"github.com/go-bumbu/userauth/userstore/userdb"
	"github.com/gorilla/mux"
)

type Handler struct {
	Users    *userdb.Store
	Sessions *cookieauth.Manager
	Tokens   *pat.Service
	Logger   *slog.Logger
}

// Routes mounts POST /auth/login and POST /auth/logout.
//
// Login is the library's JSON transport over a password-only flow: it takes
// {username, password, sessionRenew} and answers {done:true} with the session
// cookie set, or a uniform 401 for every credential-shaped failure (unknown
// user, disabled user, wrong password — deliberately indistinguishable).
// sessionRenew is the "remember me" bit: it opts the session into rolling
// expiry renewal instead of a fixed window.
func (h *Handler) Routes(r *mux.Router) {
	j := &loginhandlers.JSON{
		Flow: &loginflow.Flow{
			Users:   h.Users,
			Methods: []loginflow.Method{loginflow.PasswordMethod{Users: h.Users}},
			Policy:  loginflow.RequireAny(loginflow.Chain{loginflow.MethodPassword}),
			Session: h.Sessions, // single-factor policy: no attempt store needed
			Logger:  h.Logger,
		},
		Logger: h.Logger,
	}
	r.Path("/auth/login").Methods(http.MethodPost).Handler(j.LoginHandler())
	r.Path("/auth/logout").Methods(http.MethodPost).Handler(h.logoutHandler())
}

// logoutHandler clears the session like the library's LogoutHandler, but
// first best-effort revokes the SPA's short-lived token when the client
// names it ({tokenId} body). The revocation must never fail the logout:
// the session is being destroyed either way.
func (h *Handler) logoutHandler() http.Handler {
	inner := cookieauth.LogoutHandler(h.Sessions, "")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.Tokens != nil && r.Body != nil {
			var body struct {
				TokenID string `json:"tokenId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.TokenID != "" {
				if data, err := h.Sessions.GetSessData(r); err == nil && data.IsAuthenticated {
					_ = h.Tokens.Revoke(data.UserId, body.TokenID)
				}
			}
		}
		inner.ServeHTTP(w, r)
	})
}
