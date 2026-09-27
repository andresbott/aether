package subsonic

import "net/http"

func (h *Handler) ping(w http.ResponseWriter, r *http.Request) {
	writeResponse(w, nil)
}

func (h *Handler) getLicense(w http.ResponseWriter, r *http.Request) {
	writeResponse(w, map[string]any{
		"license": map[string]any{
			"valid": true,
		},
	})
}
