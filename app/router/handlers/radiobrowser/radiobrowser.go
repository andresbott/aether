// Package radiobrowser exposes admin-only proxy endpoints for the community
// radio-browser.info directory: searching internet radio stations by name and
// fetching a station favicon. These live under /api/v1 (server-management /
// import tooling); the stations themselves are created via the OpenSubsonic
// /rest/ API.
package radiobrowser

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/andresbott/aether/app/router/handlers/httperr"
	"github.com/andresbott/aether/internal/radiobrowser"
	"github.com/gorilla/mux"
)

// Searcher searches radio-browser.info and proxies station favicons. Satisfied
// by *radiobrowser.Client.
type Searcher interface {
	Search(ctx context.Context, query string, limit int) ([]radiobrowser.Station, error)
	FetchFavicon(ctx context.Context, faviconURL string) ([]byte, string, error)
}

type Handler struct {
	Client Searcher
}

func (h *Handler) Routes(r *mux.Router) {
	r.Path("/radiobrowser/search").Methods(http.MethodGet).HandlerFunc(h.searchStations)
	r.Path("/radiobrowser/favicon").Methods(http.MethodGet).HandlerFunc(h.getFavicon)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	httperr.Write(w, r, status, code, httperr.TitleFor(code), msg)
}

// searchStations proxies GET /radiobrowser/search?q=&limit= to the upstream
// station search. q is required; limit defaults to 10 and is capped at 25.
func (h *Handler) searchStations(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, r, http.StatusBadRequest, "validation_error", "q is required")
		return
	}
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil || n <= 0 {
			writeError(w, r, http.StatusBadRequest, "validation_error", "limit must be a positive integer")
			return
		}
		if n > 25 {
			n = 25
		}
		limit = n
	}
	results, err := h.Client.Search(r.Context(), q, limit)
	if err != nil {
		httperr.WriteUpstream(w, r, err, "The station directory could not be reached. Try again in a moment.")
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// getFavicon proxies GET /radiobrowser/favicon?url= to fetch a station favicon
// server-side, returning the image bytes. Only PNG/JPEG favicons succeed (see
// radiobrowser.Client.FetchFavicon); anything else is reported as an upstream
// error so the caller can skip the cover.
func (h *Handler) getFavicon(w http.ResponseWriter, r *http.Request) {
	u := strings.TrimSpace(r.URL.Query().Get("url"))
	if u == "" {
		writeError(w, r, http.StatusBadRequest, "validation_error", "url is required")
		return
	}
	data, contentType, err := h.Client.FetchFavicon(r.Context(), u)
	if err != nil {
		httperr.WriteUpstream(w, r, err, "The station logo could not be downloaded.")
		return
	}
	// data is validated PNG/JPEG bytes (FetchFavicon sniffs them); serve it with
	// the sniffed content type and nosniff so a browser can't reinterpret it.
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data) //nolint:gosec // G705: validated image bytes served with a non-HTML content type + nosniff
}
