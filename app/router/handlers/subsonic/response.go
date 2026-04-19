package subsonic

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	apiVersion    = "1.16.1"
	serverType    = "Aether"
	serverVersion = "0.1.0"
)

type errorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func writeResponse(w http.ResponseWriter, data map[string]any) {
	resp := map[string]any{
		"status":        "ok",
		"version":       apiVersion,
		"type":          serverType,
		"serverVersion": serverVersion,
		"openSubsonic":  true,
	}
	for k, v := range data {
		resp[k] = v
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"subsonic-response": resp})
}

func writeError(w http.ResponseWriter, code int, message string) {
	resp := map[string]any{
		"status":        "failed",
		"version":       apiVersion,
		"type":          serverType,
		"serverVersion": serverVersion,
		"openSubsonic":  true,
		"error":         errorBody{Code: code, Message: message},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"subsonic-response": resp})
}

func writeErrorf(w http.ResponseWriter, code int, format string, args ...any) {
	writeError(w, code, fmt.Sprintf(format, args...))
}
