package artists

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/andresbott/aether/app/tasks"
	"github.com/andresbott/aether/internal/artistimage"
	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/upstream"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// Searcher searches MusicBrainz for artists and releases by name. Satisfied by
// *artistimage.MusicBrainzSearch.
type Searcher interface {
	Search(ctx context.Context, query string, limit int) ([]artistimage.Candidate, error)
	SearchRelease(ctx context.Context, query string, limit int) ([]artistimage.ReleaseCandidate, error)
	ReleaseGroupGenres(ctx context.Context, mbid string) ([]string, error)
}

// Fetcher lists and downloads artist images. Satisfied by *artistimage.Chain.
// Fetch is the one-shot auto-pick used by setMBID; List/Download drive the
// manual gallery. nil when no image-provider API key is configured.
type Fetcher interface {
	Fetch(ctx context.Context, mbid string) ([]byte, string, error)
	List(ctx context.Context, mbid string) ([]artistimage.ImageCandidate, error)
	Download(ctx context.Context, providerName, url string) ([]byte, string, error)
}

type Handler struct {
	Store   *store.Store
	Assets  *assetstore.Store
	Fetcher Fetcher // nil when no image-provider API key is configured
	Search  Searcher
}

func (h *Handler) Routes(r *mux.Router) {
	r.Path("/musicbrainz/search").Methods(http.MethodGet).HandlerFunc(h.searchMusicBrainz)
	r.Path("/musicbrainz/search/releases").Methods(http.MethodGet).HandlerFunc(h.searchMusicBrainzReleases)
	r.Path("/musicbrainz/release-groups/{mbid}/genres").Methods(http.MethodGet).HandlerFunc(h.releaseGroupGenres)
	r.Path("/artists/{id:[0-9]+}/mbid").Methods(http.MethodGet).HandlerFunc(h.getMBID)
	r.Path("/artists/{id:[0-9]+}/mbid").Methods(http.MethodPut).HandlerFunc(h.setMBID)
	r.Path("/artists/{id:[0-9]+}/image-source").Methods(http.MethodGet).HandlerFunc(h.getImageSource)
	// Manual online image search: list every candidate portrait for a
	// MusicBrainz artist (no artist context needed), then store a chosen URL
	// for a specific artist. Registered before the {id} route above would
	// shadow it — "image-candidates" is not a numeric id, so the {id:[0-9]+}
	// pattern already excludes it.
	r.Path("/artists/image-candidates").Methods(http.MethodGet).HandlerFunc(h.imageCandidates)
	r.Path("/artists/{id:[0-9]+}/image-from-search").Methods(http.MethodPut).HandlerFunc(h.setImageFromSearch)
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

// writeUpstreamErr reports a failed call to an external service. The body
// carries the upstream package's human-readable sentence (or fallback for an
// error that isn't upstream-typed) — never a raw Go error — and the status
// mirrors the upstream condition so the UI can tell "retry later" (429/504)
// from "it's broken" (502).
func writeUpstreamErr(w http.ResponseWriter, err error, fallback string) {
	status := upstream.HTTPStatus(err)
	code := "upstream_error"
	if status == http.StatusTooManyRequests {
		code = "upstream_rate_limited"
	}
	writeError(w, status, code, upstream.UserMessage(err, fallback))
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
		writeUpstreamErr(w, err, "The artist search could not be completed. Try again in a moment.")
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
		writeUpstreamErr(w, err, "The release search could not be completed. Try again in a moment.")
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *Handler) releaseGroupGenres(w http.ResponseWriter, r *http.Request) {
	mbid := mux.Vars(r)["mbid"]
	if !mbidRe.MatchString(mbid) {
		writeError(w, http.StatusBadRequest, "validation_error", "mbid must be a valid MusicBrainz identifier")
		return
	}
	genres, err := h.Search.ReleaseGroupGenres(r.Context(), mbid)
	if err != nil {
		writeUpstreamErr(w, err, "The genre lookup could not be completed. Try again in a moment.")
		return
	}
	if genres == nil {
		genres = []string{}
	}
	writeJSON(w, http.StatusOK, genres)
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

// imageSourceResponse tells the UI where the image getCoverArt currently serves
// for this artist comes from, so the editor can name it instead of showing a
// bare "No file chosen". Source is one of:
//
//	"upload"  — an image the user uploaded, held in aether's asset store
//	"fetched" — an image aether auto-fetched from an image provider
//	"folder"  — an artist.jpg/folder.jpg next to the artist's albums; Path set
//	"none"    — nothing on file; getCoverArt serves the generated avatar
//
// Filename names the image file in every case but "none": the stored file for
// upload/fetched, the on-disk file for folder.
//
// This is a filesystem detail of the server, hence `/api/v1` rather than a
// non-standard field on the Subsonic artist response.
type imageSourceResponse struct {
	Source   string `json:"source"`
	Path     string `json:"path"`
	Filename string `json:"filename"`
}

// storedImageSource builds the response for an image held in aether's asset
// store, distinguishing a user upload from an auto-fetched one.
func storedImageSource(path string, manual bool) imageSourceResponse {
	source := "fetched"
	if manual {
		source = "upload"
	}
	return imageSourceResponse{Source: source, Filename: filepath.Base(path)}
}

// getImageSource mirrors artistCoverMeta's precedence in handlers/subsonic:
// asset store first, then the scanner-detected folder image. Keep the two in
// step, or the note will describe an image the user isn't looking at.
func (h *Handler) getImageSource(w http.ResponseWriter, r *http.Request) {
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

	if artist.MBArtistID != "" {
		if p, manual, ok := h.Assets.GetEntry(assetstore.KindArtist, assetkey.Artist(artist.MBArtistID, artist.NameNorm)); ok {
			writeJSON(w, http.StatusOK, storedImageSource(p, manual))
			return
		}
	}
	if p, manual, ok := h.Assets.GetEntry(assetstore.KindArtist, assetkey.Artist("", artist.NameNorm)); ok {
		writeJSON(w, http.StatusOK, storedImageSource(p, manual))
		return
	}
	// A recorded path whose file has gone away is not the served image —
	// getCoverArt falls through to the generated avatar, so report "none".
	if artistimage.IsUsablePath(artist.ImagePath) {
		writeJSON(w, http.StatusOK, imageSourceResponse{
			Source:   "folder",
			Path:     artist.ImagePath,
			Filename: filepath.Base(artist.ImagePath),
		})
		return
	}
	writeJSON(w, http.StatusOK, imageSourceResponse{Source: "none"})
}

var mbidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type imageCandidate struct {
	URL      string `json:"url"`
	ThumbURL string `json:"thumbUrl"`
	Provider string `json:"provider"`
}

// imageCandidates lists the portrait images the providers hold for a MusicBrainz
// artist. It returns URLs only — the browser loads the thumbnails straight from
// the provider CDNs, so this spends no image bandwidth here.
func (h *Handler) imageCandidates(w http.ResponseWriter, r *http.Request) {
	mbid := r.URL.Query().Get("mbid")
	if !mbidRe.MatchString(mbid) {
		writeError(w, http.StatusBadRequest, "validation_error", "mbid must be a valid MusicBrainz identifier")
		return
	}
	if h.Fetcher == nil {
		writeError(w, http.StatusServiceUnavailable, "not_configured",
			"Artist image fetching is not configured. Add an image-provider API key to use it.")
		return
	}
	cands, err := h.Fetcher.List(r.Context(), mbid)
	if err != nil {
		writeUpstreamErr(w, err, "The image lookup could not be completed. Try again in a moment.")
		return
	}
	out := make([]imageCandidate, 0, len(cands))
	for _, c := range cands {
		out = append(out, imageCandidate{URL: c.FullURL, ThumbURL: c.ThumbURL, Provider: c.Provider})
	}
	writeJSON(w, http.StatusOK, out)
}

type imageFromSearchRequest struct {
	MBID string `json:"mbid"`
	URL  string `json:"url"`
}

// setImageFromSearch stores the image of a user-chosen MusicBrainz artist for
// this artist, as a **manual** upload so it outranks whatever the auto-fetch job
// puts in the auto slot and survives the job running again.
//
// It must land in the same slot a normal upload would (the artist's own MBID when
// matched, else its DB ID): cover resolution checks the MBID slot *first*, so a
// pick filed under the DB ID would lose to an auto-fetched image in the MBID
// slot. The *chosen* MBID is only used to fetch — writing it to
// artist.MBArtistID is deliberately avoided, since picking a portrait is not the
// same as asserting a metadata match.
func (h *Handler) setImageFromSearch(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	var in imageFromSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	if !mbidRe.MatchString(in.MBID) {
		writeError(w, http.StatusBadRequest, "validation_error", "mbid must be a valid MusicBrainz identifier")
		return
	}
	if h.Fetcher == nil {
		writeError(w, http.StatusServiceUnavailable, "not_configured",
			"Artist image fetching is not configured. Add an image-provider API key to use it.")
		return
	}
	artist, _, err := h.Store.GetArtist(id)
	if err != nil {
		status, code := mapStoreError(err)
		writeError(w, status, code, err.Error())
		return
	}
	// SSRF guard: re-list and only download a URL the provider itself just
	// offered for this MBID — never an arbitrary URL from the client.
	cands, err := h.Fetcher.List(r.Context(), in.MBID)
	if err != nil {
		writeUpstreamErr(w, err, "The image lookup could not be completed. Try again in a moment.")
		return
	}
	var chosen *artistimage.ImageCandidate
	for i := range cands {
		if cands[i].FullURL == in.URL {
			chosen = &cands[i]
			break
		}
	}
	if chosen == nil {
		writeError(w, http.StatusBadRequest, "validation_error", "url is not among the candidates for this artist")
		return
	}
	data, ext, err := h.Fetcher.Download(r.Context(), chosen.Provider, chosen.FullURL)
	if err != nil {
		writeUpstreamErr(w, err, "The image lookup could not be completed. Try again in a moment.")
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusNotFound, "not_found", "No image found for this artist.")
		return
	}
	if err := h.Assets.PutManual(assetstore.KindArtist, assetkey.ArtistOf(artist), ext, data); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"stored": true})
}

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
