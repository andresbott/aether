// Package auth mounts the native-login endpoints on /api/v1: a JSON login
// that establishes the aether session cookie and a logout that clears it.
// Only registered with auth method "native" — with "none" there is no user
// store, no session manager, and no login to offer. See
// docs/agents/authentication.md (mode: builtin).
package auth

import (
	"log/slog"
	"net/http"

	"github.com/go-bumbu/userauth/auth/cookieauth"
	loginflow "github.com/go-bumbu/userauth/flow/login"
	loginhandlers "github.com/go-bumbu/userauth/flow/login/handlers"
	"github.com/go-bumbu/userauth/userstore/userdb"
	"github.com/gorilla/mux"
)

type Handler struct {
	Users    *userdb.Store
	Sessions *cookieauth.Manager
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
	r.Path("/auth/logout").Methods(http.MethodPost).Handler(cookieauth.LogoutHandler(h.Sessions, ""))
}
