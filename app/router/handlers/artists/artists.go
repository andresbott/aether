package artists

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/andresbott/aether/app/tasks"
	"github.com/andresbott/aether/internal/artistimage"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// Searcher searches MusicBrainz for artists and releases by name. Satisfied by
// *artistimage.MusicBrainzSearch.
type Searcher interface {
	Search(ctx context.Context, query string, limit int) ([]artistimage.Candidate, error)
	SearchRelease(ctx context.Context, query string, limit int) ([]artistimage.ReleaseCandidate, error)
}

type Handler struct {
	Store   *store.Store
	Assets  *assetstore.Store
	Fetcher tasks.Fetcher // nil when no image-provider API key is configured
	Search  Searcher
}

func (h *Handler) Routes(r *mux.Router) {
	r.Path("/musicbrainz/search").Methods(http.MethodGet).HandlerFunc(h.searchMusicBrainz)
	r.Path("/musicbrainz/search/releases").Methods(http.MethodGet).HandlerFunc(h.searchMusicBrainzReleases)
	r.Path("/artists/{id:[0-9]+}/mbid").Methods(http.MethodGet).HandlerFunc(h.getMBID)
	r.Path("/artists/{id:[0-9]+}/mbid").Methods(http.MethodPut).HandlerFunc(h.setMBID)
}

type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, apiError{Error: msg, Code: code})
}

func parseID(r *http.Request) (uint, error) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, errors.New("invalid id")
	}
	return uint(id), nil
}

func mapStoreError(err error) (status int, code string) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return http.StatusNotFound, "not_found"
	}
	return http.StatusInternalServerError, "internal"
}

// parseSearchParams reads and validates the shared MusicBrainz search query
// params (q required, limit optional positive int capped at 25 defaulting to
// 10). It writes the error response itself and returns ok=false on failure.
func parseSearchParams(w http.ResponseWriter, r *http.Request) (q string, limit int, ok bool) {
	q = strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "q is required")
		return "", 0, false
	}
	limit = 10
	if l := r.URL.Query().Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "validation_error", "limit must be a positive integer")
			return "", 0, false
		}
		if n > 25 {
			n = 25
		}
		limit = n
	}
	return q, limit, true
}

func (h *Handler) searchMusicBrainz(w http.ResponseWriter, r *http.Request) {
	q, limit, ok := parseSearchParams(w, r)
	if !ok {
		return
	}
	results, err := h.Search.Search(r.Context(), q, limit)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream_error", "musicbrainz search failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *Handler) searchMusicBrainzReleases(w http.ResponseWriter, r *http.Request) {
	q, limit, ok := parseSearchParams(w, r)
	if !ok {
		return
	}
	results, err := h.Search.SearchRelease(r.Context(), q, limit)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream_error", "musicbrainz search failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

type mbidResponse struct {
	MBArtistID string `json:"mbArtistId"`
}

func (h *Handler) getMBID(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	artist, _, err := h.Store.GetArtist(id)
	if err != nil {
		status, code := mapStoreError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mbidResponse{MBArtistID: artist.MBArtistID})
}

var mbidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type setMBIDRequest struct {
	MBID string `json:"mbid"`
}

type setMBIDResponse struct {
	MBArtistID   string  `json:"mbArtistId"`
	ImageFetched bool    `json:"imageFetched"`
	FetchError   *string `json:"fetchError"`
}

func (h *Handler) setMBID(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	var in setMBIDRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	if in.MBID != "" && !mbidRe.MatchString(in.MBID) {
		writeError(w, http.StatusBadRequest, "validation_error", "mbid must be a valid MusicBrainz identifier")
		return
	}
	artist, _, err := h.Store.GetArtist(id)
	if err != nil {
		status, code := mapStoreError(err)
		writeError(w, status, code, err.Error())
		return
	}
	if err := h.Store.SetArtistMBID(id, in.MBID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	artist.MBArtistID = in.MBID

	resp := setMBIDResponse{MBArtistID: in.MBID}
	if in.MBID != "" {
		resp.ImageFetched, resp.FetchError = h.fetchArtistImage(r.Context(), *artist)
	}
	writeJSON(w, http.StatusOK, resp)
}

// fetchArtistImage fetches and stores the artist image, returning whether an
// image was stored and a human-readable error message when the fetch could not
// complete.
func (h *Handler) fetchArtistImage(ctx context.Context, artist model.Artist) (bool, *string) {
	if h.Fetcher == nil {
		msg := "artist image fetching is not configured"
		return false, &msg
	}
	stored, ferr := tasks.FetchAndStoreArtistImage(ctx, h.Store, h.Assets, h.Fetcher, artist)
	if ferr != nil {
		msg := ferr.Error()
		return stored, &msg
	}
	if !stored {
		msg := "no image found for this artist"
		return stored, &msg
	}
	return stored, nil
}
