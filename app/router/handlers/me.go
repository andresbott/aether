package handlers

import (
	"encoding/json"
	"net/http"
)

// meUser identifies the authenticated user. Always nil today: no request
// carries identity until sessions land, so /me answers identically for
// everyone. The field is in the contract now so the SPA can code against the
// final shape.
type meUser struct {
	Login string `json:"login"`
}

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
	User       *meUser    `json:"user"`
	Features   meFeatures `json:"features"`
}

// MeHandler is the SPA's bootstrap endpoint (GET /api/v1/me, see
// docs/agents/authentication.md): it reports the active auth method, the
// caller's identity (null until sessions exist) and the features this server
// exposes, so the UI adapts without build-time configuration. It is
// deliberately public — the SPA needs it before any login.
func MeHandler(authMethod string, userManagement bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(meResponse{
			AuthMethod: authMethod,
			Features:   meFeatures{UserManagement: userManagement},
		})
	})
}
