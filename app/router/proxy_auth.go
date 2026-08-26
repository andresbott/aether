package router

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"github.com/andresbott/aether/app/router/handlers"
	"github.com/andresbott/aether/app/router/handlers/httperr"
	usersHandler "github.com/andresbott/aether/app/router/handlers/users"
	"github.com/go-bumbu/userauth"
	"github.com/go-bumbu/userauth/auth/headerauth"
	"github.com/go-bumbu/userauth/userstore/userdb"
)

// proxyIdentity is the resolved identity of a header-authenticated request:
// the JIT-provisioned user row plus the role derived live from the proxy's
// groups header (the IdP is authoritative; DB groups are not consulted).
type proxyIdentity struct {
	UserID string
	Login  string
	Role   string
}

type proxyIdentityCtxKey struct{}

// ctxGetProxyIdentity reads the identity headerGuard stashed on the request.
func ctxGetProxyIdentity(r *http.Request) (proxyIdentity, bool) {
	id, ok := r.Context().Value(proxyIdentityCtxKey{}).(proxyIdentity)
	return id, ok
}

// resolveProxyIdentity validates the proxy headers and resolves the caller to
// a user row, provisioning one on first sight of a new login. It returns
// (nil, nil) when the request carries no trusted identity and an error only
// for infrastructure failures; a disabled user resolves with Role untouched —
// the caller decides (the guard 403s, /me reports nothing).
func (h *MainAppHandler) resolveProxyIdentity(w http.ResponseWriter, r *http.Request) (*proxyIdentity, *userauth.User, error) {
	ok, _ := h.headerAuth.HandleAuth(w, r)
	if !ok {
		return nil, nil, nil
	}
	data, err := headerauth.CtxGetRequestData(r)
	if err != nil {
		return nil, nil, nil
	}
	usr, err := h.jitUser(data.UserName)
	if err != nil {
		return nil, nil, err
	}
	role := usersHandler.RoleUser
	for _, g := range data.Groups {
		if g == h.adminGroup {
			role = usersHandler.RoleAdmin
			break
		}
	}
	// Mirror the header-derived role into the DB groups. The IdP stays
	// authoritative on /api/v1 (the guard uses the live role above, never the
	// DB) — but /rest is proxy-bypassed and carries no identity headers, so
	// its admin check (restAdminChecker) can only consult the DB. Written only
	// on change, so the steady state costs one read per request.
	stored, err := usersHandler.RoleOf(h.users, usr.ID)
	if err != nil {
		return nil, nil, err
	}
	if stored != role {
		if err := usersHandler.SetRole(h.users, usr.ID, role); err != nil {
			return nil, nil, err
		}
	}
	return &proxyIdentity{UserID: usr.ID, Login: usr.LoginID, Role: role}, &usr, nil
}

// jitUser resolves a login to a user row, creating it on first sight. The
// row exists so PATs (and any future per-user state keyed on User.ID) have an
// owner the pat service can verify; its password is a random throwaway — in
// proxy mode nothing ever authenticates against it.
func (h *MainAppHandler) jitUser(login string) (userauth.User, error) {
	usr, err := h.users.GetUserByLogin(login)
	if err == nil {
		return usr, nil
	}
	if !errors.Is(err, userauth.ErrUserNotFound) {
		return userauth.User{}, err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return userauth.User{}, fmt.Errorf("generate placeholder password: %w", err)
	}
	createErr := h.users.CreateUser(userdb.User{
		LoginID: login,
		Pw:      hex.EncodeToString(secret),
		Enabled: true,
	})
	// Two concurrent first requests race on the unique login; whoever loses
	// just reads the winner's row.
	usr, err = h.users.GetUserByLogin(login)
	if err != nil {
		if createErr != nil {
			return userauth.User{}, fmt.Errorf("provision user %q: %w", login, createErr)
		}
		return userauth.User{}, err
	}
	return usr, nil
}

// headerGuard is the proxy-header counterpart of sessionGuard, enforcing the
// same three tiers on /api/v1: public bootstrap, header-authenticated
// (personal token mint + CRUD), and admin default. Identity comes exclusively
// from the trusted proxy's headers; the login/logout endpoints and users CRUD
// are not mounted in this mode.
func (h *MainAppHandler) headerGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if apiV1PublicPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		id, usr, err := h.resolveProxyIdentity(w, r)
		if err != nil {
			h.logger.Error("proxy auth: identity resolution failed", "err", err)
			httperr.Write(w, r, http.StatusInternalServerError, "internal", httperr.TitleFor("internal"), "internal error")
			return
		}
		if id == nil {
			httperr.Write(w, r, http.StatusUnauthorized, "unauthorized", httperr.TitleFor("unauthorized"), "authentication required")
			return
		}
		// The DB Enabled flag is aether's kill-switch: it blocks a user the
		// proxy still authenticates (pat.Verify enforces the same on /rest).
		if !usr.Enabled {
			httperr.Write(w, r, http.StatusForbidden, "forbidden", httperr.TitleFor("forbidden"), "user is disabled")
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), proxyIdentityCtxKey{}, *id))
		if apiV1SessionPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if id.Role != usersHandler.RoleAdmin {
			httperr.Write(w, r, http.StatusForbidden, "forbidden", httperr.TitleFor("forbidden"), "admin privileges required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// proxyMeIdentity resolves the /me caller from the proxy headers. /me is
// public-tier so it resolves independently of the guard's context stash;
// a disabled or absent identity reports as anonymous.
func (h *MainAppHandler) proxyMeIdentity(w http.ResponseWriter, r *http.Request) *handlers.MeUser {
	id, usr, err := h.resolveProxyIdentity(w, r)
	if err != nil || id == nil || !usr.Enabled {
		return nil
	}
	return &handlers.MeUser{Login: id.Login, Role: id.Role}
}

// proxyCaller adapts the guard's context identity to the tokens handler's
// resolver seam.
func proxyCaller(r *http.Request) (string, bool) {
	id, ok := ctxGetProxyIdentity(r)
	if !ok {
		return "", false
	}
	return id.UserID, true
}
