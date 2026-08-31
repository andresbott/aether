// Package auth mounts the native-login endpoints on /api/v1: a JSON login
// that establishes the aether session cookie and a logout that clears it.
// Only registered with auth method "native" — with "none" there is no user
// store, no session manager, and no login to offer. See
// docs/agents/authentication.md (mode: builtin).
package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/andresbott/aether/app/router/handlers/httperr"
	usersHandler "github.com/andresbott/aether/app/router/handlers/users"
	"github.com/go-bumbu/userauth"
	"github.com/go-bumbu/userauth/auth/cookieauth"
	loginflow "github.com/go-bumbu/userauth/flow/login"
	loginhandlers "github.com/go-bumbu/userauth/flow/login/handlers"
	"github.com/go-bumbu/userauth/service/password"
	"github.com/go-bumbu/userauth/service/pat"
	"github.com/go-bumbu/userauth/service/throttle"
	"github.com/go-bumbu/userauth/service/user"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	Users     *user.Service
	Passwords *password.Service
	Sessions  *cookieauth.Manager
	Tokens    *pat.Service
	// Guard throttles the login flow per login identifier (brute-force
	// backoff). Nil leaves login unguarded.
	Guard loginflow.Guard
	// Reauth throttles the change-password re-verification per user, so a
	// stolen session cannot brute-force the current password through the
	// change-password endpoint. Nil leaves that check unthrottled.
	Reauth *throttle.Backoff
	Logger *slog.Logger
}

// reauthMethod is the throttle bucket for change-password re-verification,
// separate from the login bucket so the two backoffs never share a budget.
const reauthMethod = "reauth"

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
			Methods: []loginflow.Method{loginflow.PasswordMethod{Users: h.Users, Password: h.Passwords}},
			Policy:  loginflow.RequireAny(loginflow.Chain{loginflow.MethodPassword}),
			Session: h.Sessions, // single-factor policy: no attempt store needed
			Guard:   h.Guard,    // nil leaves login unguarded; set to throttle brute force
			Logger:  h.Logger,
		},
		Logger: h.Logger,
	}
	r.Path("/auth/login").Methods(http.MethodPost).Handler(j.LoginHandler())
	r.Path("/auth/logout").Methods(http.MethodPost).Handler(h.logoutHandler())
	r.Path("/auth/password").Methods(http.MethodPut).Handler(h.changePasswordHandler())
}

// changePasswordRequest is the body of PUT /auth/password. The confirm-new
// check is a frontend concern; the server only needs the two values.
type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// changePasswordHandler lets the signed-in user change their own password.
// Session-tier (any role): it acts only on the caller's own credential, so it
// is not admin-gated. The current password is re-verified — a live session is
// not by itself authority to change the credential that mints it — with
// brute-force backoff so the endpoint cannot become a password oracle for a
// stolen session. On success the caller's own session cookie is cleared, so a
// password change signs this device out and the user signs back in with the
// new password. (Aether's sessions are stateless encrypted cookies, so
// SetPasswordHash's server-side revocation is a no-op and OTHER devices' cookies
// cannot be forced to expire early — see docs/agents/authentication.md.)
func (h *Handler) changePasswordHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := cookieauth.CtxGetUserData(r)
		if err != nil || !data.IsAuthenticated {
			// The session guard should have caught this; belt and braces.
			httperr.Write(w, r, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}

		var in changePasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			httperr.Write(w, r, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
			return
		}
		if strings.TrimSpace(in.CurrentPassword) == "" {
			httperr.Write(w, r, http.StatusBadRequest, "validation_error", "current password is required")
			return
		}
		if err := usersHandler.ValidPassword(in.NewPassword); err != nil {
			if errors.Is(err, usersHandler.ErrPasswordTooLong) {
				httperr.WriteValidation(w, r, err.Error(), httperr.FieldError{Pointer: "/newPassword", Detail: err.Error()})
				return
			}
			httperr.Write(w, r, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		usr, err := h.Users.GetUser(data.UserId)
		if err != nil {
			httperr.Write(w, r, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}

		// Re-verify the current password with brute-force backoff. Reproduces
		// what the userauth reauth.Guard does (unreleased in the pinned
		// version) over the throttle.Backoff this codebase already uses for
		// login: throttle first, verify only if allowed, record the outcome so
		// repeated wrong guesses escalate the delay.
		ok, retryAfter, err := h.verifyCurrent(usr, in.CurrentPassword)
		if err != nil {
			h.logger().Error("auth: could not verify the current password", "user", usr.ID, "error", err)
			httperr.Write(w, r, http.StatusInternalServerError, "internal", "could not verify the current password")
			return
		}
		if !ok {
			if retryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Round(time.Second)/time.Second)))
				httperr.Write(w, r, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later")
				return
			}
			// 403, not 401: the session is valid — the re-auth check failed.
			// A 401 here would read as a lost session and sign the caller out
			// of the SPA (the client treats any /api/v1 401 as session expiry);
			// 401 stays reserved for the guard's genuine no-session case.
			httperr.Write(w, r, http.StatusForbidden, "reauth_failed", "current password is incorrect")
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), usersHandler.BcryptDifficulty)
		if err != nil {
			httperr.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		if err := h.Users.SetPasswordHash(data.UserId, string(hash)); err != nil {
			httperr.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
			return
		}

		// Clear the caller's session cookie so the 204 itself means "signed
		// out" — the user re-logs in with the new password.
		if err := h.Sessions.LogoutUser(r, w); err != nil {
			h.logger().Warn("auth: could not clear the session cookie after a password change",
				"user", data.UserId, "error", err)
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// verifyCurrent re-verifies secret against usr, throttled per user. It returns
// (true, 0, nil) on a match, (false, >0, nil) when throttled (the verifier was
// not called), (false, 0, nil) on a wrong secret, and a non-nil error on an
// unusable hash or throttle-store failure (failing closed). With a nil Reauth
// the check runs unthrottled.
func (h *Handler) verifyCurrent(usr userauth.User, secret string) (ok bool, retryAfter time.Duration, err error) {
	if h.Reauth == nil {
		matched, verr := h.Passwords.Verify(usr, secret)
		return matched, 0, verr
	}
	allowed, wait, err := h.Reauth.Allow(usr.ID, reauthMethod)
	if err != nil {
		return false, 0, err
	}
	if !allowed {
		return false, wait, nil
	}
	matched, verr := h.Passwords.Verify(usr, secret)
	if verr != nil {
		// An unusable stored hash is not proof of ownership: record it as a
		// failure so it cannot hand out un-throttled retries, and surface it.
		if ferr := h.Reauth.Fail(usr.ID, reauthMethod); ferr != nil {
			return false, 0, ferr
		}
		return false, 0, verr
	}
	if !matched {
		if ferr := h.Reauth.Fail(usr.ID, reauthMethod); ferr != nil {
			// Cannot record the failure: fail closed rather than un-throttle.
			return false, 0, ferr
		}
		return false, 0, nil
	}
	if cerr := h.Reauth.Success(usr.ID, reauthMethod); cerr != nil {
		// A failure to clear the counter must not deny a verified credential.
		h.logger().Warn("auth: could not clear the re-auth failure counter", "user", usr.ID, "error", cerr)
	}
	return true, 0, nil
}

func (h *Handler) logger() *slog.Logger {
	if h.Logger != nil {
		return h.Logger
	}
	return slog.New(slog.DiscardHandler)
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
