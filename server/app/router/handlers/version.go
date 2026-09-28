package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/andresbott/aether/app/metainfo"
)

// VersionHandler reports the build information baked into the binary so the UI
// can display it. Values come from ldflags at release time.
func VersionHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"version":    metainfo.Version,
			"build_time": metainfo.BuildTime,
			"commit":     metainfo.ShaVer,
		})
	})
}
