package metadata

import (
	"encoding/json"
	"net/http"
)

// writeJSON writes a JSON body with the given status. Shared by every metadata
// editor handler.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// codeFor maps an HTTP status to the error code used in the problem body of the
// non-selection endpoints (the ones that resolve a library from the query
// string rather than through resolveSelection).
func codeFor(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "validation_error"
	case http.StatusNotFound:
		return "not_found"
	default:
		return "internal"
	}
}
