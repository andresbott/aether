package handlers

import (
	"encoding/json"
	"net/http"
)

// MeUser identifies the authenticated user. Nil when the request carries no
// valid session (or the auth method is "none", where nothing ever does).
type MeUser struct {
	Login string `json:"login"`
	// Role is the user's vertical, "admin" or "user" (see handlers/users).
	// The SPA gates the administration UI on it; the real enforcement is the
	// admin guard on /api/v1.
	Role string `json:"role"`
}

// MeIdentity resolves the caller's identity from the request, or nil when it
// is anonymous. It gets the ResponseWriter so a session-backed implementation
// can renew the rolling cookie expiry on the SPA's bootstrap call.
type MeIdentity func(w http.ResponseWriter, r *http.Request) *MeUser

// meFeatures lists the server capabilities the SPA gates UI on. Only
// capabilities that depend on server configuration belong here — the SPA
// cannot know them at build time.
type meFeatures struct {
	// UserManagement is true when the native users CRUD is mounted on
	// /api/v1 (auth method "native").
	UserManagement bool `json:"userManagement"`
}

type meResponse struct {
	AuthMethod string     `json:"authMethod"`
	User       *MeUser    `json:"user"`
	Features   meFeatures `json:"features"`
}

// MeHandler is the SPA's bootstrap endpoint (GET /api/v1/me, see
// docs/agents/authentication.md): it reports the active auth method, the
// caller's identity and the features this server exposes, so the UI adapts
// without build-time configuration. It is deliberately public — the SPA needs
// it before any login; identity may be nil (auth method "none", or no
// session) and the SPA reacts by showing the login view.
func MeHandler(authMethod string, userManagement bool, identity MeIdentity) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var user *MeUser
		if identity != nil {
			user = identity(w, r)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(meResponse{
			AuthMethod: authMethod,
			User:       user,
			Features:   meFeatures{UserManagement: userManagement},
		})
	})
}
