// Package wishlist serves /api/v0/wishlist: music the server does not have
// yet, picked from MusicBrainz and matched against the catalog by MBID.
//
// It lives on /api/v0 rather than /rest by the owner's choice: the wishlist is
// a server-wide acquisition list, not a music-client feature, and no Subsonic
// client has a use for it.
package wishlist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/andresbott/aether/internal/artistimage"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/go-bumbu/http/problemjson"
	"github.com/gorilla/mux"
)

const (
	// MaxAddItems caps one add request; the UI adds a page of picks at most.
	MaxAddItems = 50
	// maxFieldLen bounds every snapshot field, in characters.
	maxFieldLen = 500
	// MaxNoteLen bounds the free-text note, in characters.
	MaxNoteLen = 2000
	// minQueryLen and maxQueryLen bound a search's text: a
	// URL is a bounded channel, and every search spends the shared
	// MusicBrainz budget.
	minQueryLen         = 2
	maxQueryLen         = 200
	defaultSearchLimit  = 15
	maxSearchLimit      = 25
	maxRequestBodyBytes = 1 << 20
)

var mbidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Searcher runs the three MusicBrainz searches the wishlist picks from.
// Satisfied by *artistimage.MusicBrainzSearch.
type Searcher interface {
	Search(ctx context.Context, query string, limit int) ([]artistimage.Candidate, error)
	SearchReleaseGroups(ctx context.Context, query string, limit int) ([]artistimage.ReleaseGroupCandidate, error)
	SearchRecordings(ctx context.Context, query string, limit int) ([]artistimage.RecordingCandidate, error)
}

type Handler struct {
	Store  *store.Store
	Search Searcher
	// Problems writes this handler's application/problem+json error responses.
	Problems *problemjson.Writer
}

func (h *Handler) Routes(r *mux.Router) {
	r.Path("/wishlist").Methods(http.MethodGet).HandlerFunc(h.list)
	r.Path("/wishlist").Methods(http.MethodPost).HandlerFunc(h.add)
	r.Path("/wishlist/search").Methods(http.MethodGet).HandlerFunc(h.search)
	r.Path("/wishlist/{id:[0-9]+}").Methods(http.MethodGet).HandlerFunc(h.get)
	r.Path("/wishlist/{id:[0-9]+}").Methods(http.MethodPut).HandlerFunc(h.update)
	r.Path("/wishlist/{id:[0-9]+}").Methods(http.MethodDelete).HandlerFunc(h.remove)
}

// snapshot is the part of an entry copied from the MusicBrainz result it was
// picked from — the shape a search result and an add request share.
type snapshot struct {
	Kind           model.WishlistKind `json:"kind"`
	MBID           string             `json:"mbid"`
	Title          string             `json:"title"`
	Artist         string             `json:"artist"`
	Album          string             `json:"album"`
	Date           string             `json:"date"`
	Type           string             `json:"type"`
	Disambiguation string             `json:"disambiguation"`
}

// match is where an MBID is in the catalog. MatchID is the artist, album or
// track id by kind; MatchAlbumID the track's album, for songs only.
type match struct {
	InLibrary    bool `json:"in_library"`
	MatchID      uint `json:"match_id,omitempty"`
	MatchAlbumID uint `json:"match_album_id,omitempty"`
}

type itemDTO struct {
	ID uint `json:"id"`
	snapshot
	Note string `json:"note"`
	match
	FoundAt   *time.Time `json:"found_at"`
	CreatedAt time.Time  `json:"created_at"`
}

type resultDTO struct {
	snapshot
	Score int `json:"score"`
	// WishlistID is the entry already listing this result, 0 when none.
	WishlistID uint `json:"wishlist_id,omitempty"`
	match
}

func toDTO(it model.WishlistItem) itemDTO {
	return itemDTO{
		ID: it.ID,
		snapshot: snapshot{
			Kind:           it.Kind,
			MBID:           it.MBID,
			Title:          it.Title,
			Artist:         it.Artist,
			Album:          it.Album,
			Date:           it.Date,
			Type:           it.Type,
			Disambiguation: it.Disambiguation,
		},
		Note:      it.Note,
		match:     match{InLibrary: it.MatchID != 0, MatchID: it.MatchID, MatchAlbumID: it.MatchAlbumID},
		FoundAt:   it.MatchedAt,
		CreatedAt: it.CreatedAt,
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListWishlist()
	if err != nil {
		h.storeError(w, r, err)
		return
	}
	out := make([]itemDTO, 0, len(items))
	for _, it := range items {
		out = append(out, toDTO(it))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}
	it, err := h.Store.GetWishlistItem(id)
	if err != nil {
		h.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toDTO(*it))
}

// addItem is one entry to add: a MusicBrainz pick (with its mbid), or an entry
// added by hand (no mbid — a title, a kind and the note as its description).
type addItem struct {
	snapshot
	Note string `json:"note"`
}

type addRequest struct {
	Items []addItem `json:"items"`
}

// add puts MusicBrainz picks — or entries added by hand, which carry no mbid —
// on the list. It is idempotent per (kind, mbid): an already-listed pick comes
// back as the stored entry, so adding a selection twice is harmless; a manual
// entry is always new. The answer lists the entries in request order.
func (h *Handler) add(w http.ResponseWriter, r *http.Request) {
	var req addRequest
	if !h.decode(w, r, &req) {
		return
	}
	if errs := validateItems(req.Items); len(errs) > 0 {
		h.Problems.WriteValidation(w, r, "the wishlist items are not valid", errs...)
		return
	}
	rows := make([]model.WishlistItem, 0, len(req.Items))
	for _, s := range req.Items {
		rows = append(rows, model.WishlistItem{
			Kind:           s.Kind,
			MBID:           strings.ToLower(strings.TrimSpace(s.MBID)),
			Title:          strings.TrimSpace(s.Title),
			Artist:         strings.TrimSpace(s.Artist),
			Album:          strings.TrimSpace(s.Album),
			Date:           strings.TrimSpace(s.Date),
			Type:           strings.TrimSpace(s.Type),
			Disambiguation: strings.TrimSpace(s.Disambiguation),
			Note:           strings.TrimSpace(s.Note),
		})
	}
	stored, err := h.Store.AddWishlistItems(rows)
	if err != nil {
		h.storeError(w, r, err)
		return
	}
	out := make([]itemDTO, 0, len(stored))
	for _, it := range stored {
		out = append(out, toDTO(it))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// validateItems reports every problem in an add request, each pointer rooted
// at the offending element.
func validateItems(items []addItem) []problemjson.FieldError {
	if len(items) == 0 {
		return []problemjson.FieldError{{Pointer: "/items", Detail: "at least one item is required"}}
	}
	if len(items) > MaxAddItems {
		return []problemjson.FieldError{{Pointer: "/items", Detail: fmt.Sprintf("at most %d items can be added at once", MaxAddItems)}}
	}
	var errs []problemjson.FieldError
	for i, s := range items {
		at := func(field string) string { return fmt.Sprintf("/items/%d/%s", i, field) }
		if !s.Kind.Valid() {
			errs = append(errs, problemjson.FieldError{Pointer: at("kind"), Detail: "kind must be artist, album or song"})
		}
		// Empty is an entry added by hand; anything else must be a real MBID.
		if mbid := strings.TrimSpace(s.MBID); mbid != "" && !mbidRe.MatchString(strings.ToLower(mbid)) {
			errs = append(errs, problemjson.FieldError{Pointer: at("mbid"), Detail: "mbid must be a MusicBrainz identifier, or empty for an entry added by hand"})
		}
		if strings.TrimSpace(s.Title) == "" {
			errs = append(errs, problemjson.FieldError{Pointer: at("title"), Detail: "title is required"})
		}
		for _, f := range []struct{ name, value string }{
			{"title", s.Title}, {"artist", s.Artist}, {"album", s.Album}, {"date", s.Date},
			{"type", s.Type}, {"disambiguation", s.Disambiguation},
		} {
			if utf8.RuneCountInString(f.value) > maxFieldLen {
				errs = append(errs, problemjson.FieldError{Pointer: at(f.name), Detail: fmt.Sprintf("at most %d characters", maxFieldLen)})
			}
		}
		if utf8.RuneCountInString(strings.TrimSpace(s.Note)) > MaxNoteLen {
			errs = append(errs, problemjson.FieldError{Pointer: at("note"), Detail: fmt.Sprintf("at most %d characters", MaxNoteLen)})
		}
	}
	return errs
}

// updateRequest is a partial update: an absent field is left as is. `note`
// (the description) may change on any entry; `title` and `kind` only on an
// entry added by hand — on a MusicBrainz pick they are its snapshot.
type updateRequest struct {
	Note  *string             `json:"note"`
	Title *string             `json:"title"`
	Kind  *model.WishlistKind `json:"kind"`
}

// update edits an entry: its note on any entry, every field on one added by
// hand. Every problem is itemised in one 422.
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}
	var req updateRequest
	if !h.decode(w, r, &req) {
		return
	}
	u, errs := validateUpdate(req)
	if len(errs) > 0 {
		h.Problems.WriteValidation(w, r, "the update is not valid", errs...)
		return
	}
	it, err := h.Store.UpdateWishlistItem(id, u)
	if errors.Is(err, store.ErrWishlistNotManual) {
		pointer := "/kind"
		if req.Title != nil {
			pointer = "/title"
		}
		h.Problems.WriteValidation(w, r, "only an entry added by hand can change its title or kind",
			problemjson.FieldError{Pointer: pointer, Detail: "this entry comes from MusicBrainz; only its note can change"})
		return
	}
	if err != nil {
		h.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toDTO(*it))
}

// validateUpdate trims an update and reports every problem in it.
func validateUpdate(req updateRequest) (store.WishlistUpdate, []problemjson.FieldError) {
	var u store.WishlistUpdate
	var errs []problemjson.FieldError
	if req.Note == nil && req.Title == nil && req.Kind == nil {
		errs = append(errs, problemjson.FieldError{Pointer: "/note", Detail: "nothing to update: send note, title or kind"})
	}
	if req.Note != nil {
		note := strings.TrimSpace(*req.Note)
		if utf8.RuneCountInString(note) > MaxNoteLen {
			errs = append(errs, problemjson.FieldError{Pointer: "/note", Detail: fmt.Sprintf("at most %d characters", MaxNoteLen)})
		}
		u.Note = &note
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		switch {
		case title == "":
			errs = append(errs, problemjson.FieldError{Pointer: "/title", Detail: "title is required"})
		case utf8.RuneCountInString(title) > maxFieldLen:
			errs = append(errs, problemjson.FieldError{Pointer: "/title", Detail: fmt.Sprintf("at most %d characters", maxFieldLen)})
		}
		u.Title = &title
	}
	if req.Kind != nil {
		if !req.Kind.Valid() {
			errs = append(errs, problemjson.FieldError{Pointer: "/kind", Detail: "kind must be artist, album or song"})
		}
		u.Kind = req.Kind
	}
	return u, errs
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}
	if err := h.Store.DeleteWishlistItem(id); err != nil {
		h.storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// search looks MusicBrainz up for one kind and marks each result that is
// already on the wishlist or already in the catalog. `q` is free text: a
// title, a name, or both ("in utero nirvana").
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	kind := model.WishlistKind(qs.Get("kind"))
	q := strings.TrimSpace(qs.Get("q"))
	var errs []problemjson.FieldError
	if !kind.Valid() {
		errs = append(errs, problemjson.FieldError{Pointer: "/kind", Detail: "kind must be artist, album or song"})
	}
	switch n := utf8.RuneCountInString(q); {
	case n < minQueryLen:
		errs = append(errs, problemjson.FieldError{Pointer: "/q", Detail: fmt.Sprintf("the search needs at least %d characters", minQueryLen)})
	case n > maxQueryLen:
		errs = append(errs, problemjson.FieldError{Pointer: "/q", Detail: fmt.Sprintf("at most %d characters", maxQueryLen)})
	}
	limit := defaultSearchLimit
	if l := qs.Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil || n < 1 || n > maxSearchLimit {
			errs = append(errs, problemjson.FieldError{Pointer: "/limit", Detail: fmt.Sprintf("limit must be between 1 and %d", maxSearchLimit)})
		}
		limit = n
	}
	if len(errs) > 0 {
		h.Problems.WriteValidation(w, r, "the search is not valid", errs...)
		return
	}

	results, err := h.runSearch(r.Context(), kind, artistimage.SearchQuery(q), limit)
	if err != nil {
		h.Problems.WriteUpstream(w, r, err, "The MusicBrainz search could not be completed. Try again in a moment.")
		return
	}

	mbids := make([]string, 0, len(results))
	for _, res := range results {
		mbids = append(mbids, res.MBID)
	}
	listed, err := h.Store.WishlistIDsByMBID(kind, mbids)
	if err != nil {
		h.storeError(w, r, err)
		return
	}
	owned, err := h.Store.WishlistCatalogMatches(kind, mbids)
	if err != nil {
		h.storeError(w, r, err)
		return
	}
	for i := range results {
		results[i].WishlistID = listed[results[i].MBID]
		if m, ok := owned[results[i].MBID]; ok {
			results[i].match = match{InLibrary: true, MatchID: m.ID, MatchAlbumID: m.AlbumID}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// runSearch runs the MusicBrainz search for kind and flattens its candidates
// into the snapshot an add request carries back.
func (h *Handler) runSearch(ctx context.Context, kind model.WishlistKind, query string, limit int) ([]resultDTO, error) {
	out := []resultDTO{}
	switch kind {
	case model.WishlistArtist:
		cands, err := h.Search.Search(ctx, query, limit)
		if err != nil {
			return nil, err
		}
		for _, c := range cands {
			out = append(out, resultDTO{Score: c.Score, snapshot: snapshot{
				Kind: kind, MBID: c.MBID, Title: c.Name, Type: c.Type,
				Date: c.LifeSpanBegin, Disambiguation: c.Disambiguation,
			}})
		}
	case model.WishlistAlbum:
		cands, err := h.Search.SearchReleaseGroups(ctx, query, limit)
		if err != nil {
			return nil, err
		}
		for _, c := range cands {
			types := append([]string{c.PrimaryType}, c.SecondaryTypes...)
			out = append(out, resultDTO{Score: c.Score, snapshot: snapshot{
				Kind: kind, MBID: c.MBID, Title: c.Title, Artist: c.Artist,
				Date: c.FirstReleaseDate, Type: joinNonEmpty(types, " · "), Disambiguation: c.Disambiguation,
			}})
		}
	case model.WishlistSong:
		cands, err := h.Search.SearchRecordings(ctx, query, limit)
		if err != nil {
			return nil, err
		}
		for _, c := range cands {
			out = append(out, resultDTO{Score: c.Score, snapshot: snapshot{
				Kind: kind, MBID: c.MBID, Title: c.Title, Artist: c.Artist, Album: c.Release,
				Date: c.FirstReleaseDate, Disambiguation: c.Disambiguation,
			}})
		}
	}
	return out, nil
}

func joinNonEmpty(parts []string, sep string) string {
	kept := parts[:0:0]
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, sep)
}

func (h *Handler) parseID(w http.ResponseWriter, r *http.Request) (uint, bool) {
	id, err := strconv.ParseUint(mux.Vars(r)["id"], 10, 64)
	if err != nil || id == 0 {
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", "invalid wishlist id")
		return 0, false
	}
	return uint(id), true
}

func (h *Handler) decode(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			h.Problems.Write(w, r, http.StatusRequestEntityTooLarge, "validation_error", "the request body is too large")
			return false
		}
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", "invalid JSON body")
		return false
	}
	return true
}

func (h *Handler) storeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		h.Problems.Write(w, r, http.StatusNotFound, "not_found", "wishlist item not found")
		return
	}
	h.Problems.Write(w, r, http.StatusInternalServerError, "internal", "internal error")
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
