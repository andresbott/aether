package handlers

import (
	"encoding/json"
	"net/http"
)

// AuthInfoHandler reports the active authentication method ("none" or
// "native") so the SPA can gate auth-dependent UI (the Users settings section,
// the future login view) without build-time configuration.
func AuthInfoHandler(method string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"method": method,
		})
	})
}
