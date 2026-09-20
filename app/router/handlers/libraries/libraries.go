package libraries

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/go-bumbu/http/problemjson"
	"github.com/gorilla/mux"
)

type Handler struct {
	Store *store.Store
	// Problems writes this handler's application/problem+json error responses.
	Problems *problemjson.Writer
}

type libraryDTO struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	// ShowArtists is a pointer so an omitted key keeps its default (true on
	// create) instead of reading as false.
	ShowArtists *bool     `json:"show_artists"`
	DefaultView string    `json:"default_view"`
	Icon        string    `json:"icon"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	TrackCount  int64     `json:"track_count"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func parseID(r *http.Request) (uint, error) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, errors.New("invalid id")
	}
	return uint(id), nil
}

// parseBoolParam reads an optional boolean query parameter; an absent value is
// false.
func parseBoolParam(raw string) (bool, error) {
	if raw == "" {
		return false, nil
	}
	return strconv.ParseBool(raw)
}

func (h *Handler) modelToDTO(lib model.Library) (libraryDTO, error) {
	count, err := h.Store.CountTracks(store.LibraryScope(&lib))
	if err != nil {
		return libraryDTO{}, err
	}
	dv := lib.DefaultView
	if dv == "" {
		dv = "albums"
	}
	icon := lib.Icon
	if icon == "" {
		icon = "folder"
	}
	// Convert HideArtists (internal, inverted bool) to ShowArtists (API, positive bool).
	// HideArtists=false (zero value, default) means artists are visible, so ShowArtists=true.
	// HideArtists=true means artists are hidden, so ShowArtists=false.
	showArtists := !lib.HideArtists
	return libraryDTO{
		ID:          lib.ID,
		Name:        lib.Name,
		ShowArtists: &showArtists,
		DefaultView: dv,
		Icon:        icon,
		CreatedAt:   lib.CreatedAt,
		UpdatedAt:   lib.UpdatedAt,
		TrackCount:  count,
	}, nil
}

func (h *Handler) Routes(r *mux.Router) {
	r.Path("/libraries/browse").Methods(http.MethodGet).HandlerFunc(h.browse)
	r.Path("/libraries").Methods(http.MethodGet).HandlerFunc(h.list)
	r.Path("/libraries").Methods(http.MethodPost).HandlerFunc(h.create)
	r.Path("/libraries/{id:[0-9]+}").Methods(http.MethodGet).HandlerFunc(h.get)
	r.Path("/libraries/{id:[0-9]+}").Methods(http.MethodPut).HandlerFunc(h.update)
	r.Path("/libraries/{id:[0-9]+}").Methods(http.MethodDelete).HandlerFunc(h.delete)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	libs, err := h.Store.ListLibraries()
	if err != nil {
		h.Problems.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]libraryDTO, 0, len(libs))
	for _, lib := range libs {
		dto, err := h.modelToDTO(lib)
		if err != nil {
			h.Problems.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, map[string][]libraryDTO{"libraries": out})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	lib, err := h.Store.GetLibrary(id)
	if err != nil {
		status, code := mapStoreError(err)
		h.Problems.Write(w, r, status, code, err.Error())
		return
	}
	dto, err := h.modelToDTO(lib)
	if err != nil {
		h.Problems.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dto)
}

// validateDTO checks every field of an incoming library payload. It answers
// the response itself; the return value is false when the request is done. A
// missing required field (name) is 400; a present-but-invalid value (too
// long, an unknown enum) is well-formed-but-invalid input, answered as a 422
// validation problem.
func validateDTO(w http.ResponseWriter, r *http.Request, in libraryDTO, pw *problemjson.Writer) bool {
	if err := ValidateName(in.Name); err != nil {
		writeFieldValidationErr(w, r, "/name", err, pw)
		return false
	}
	// Unlike name, neither of these ever fails on a missing value (an
	// empty/omitted field is always accepted, defaulted elsewhere) — any
	// failure here is unconditionally a present-but-invalid value.
	for _, check := range []struct {
		pointer string
		err     error
	}{
		{"/default_view", ValidateDefaultView(in.DefaultView)},
		{"/icon", ValidateIcon(in.Icon)},
	} {
		if check.err != nil {
			pw.WriteValidation(w, r, check.err.Error(), problemjson.FieldError{Pointer: check.pointer, Detail: check.err.Error()})
			return false
		}
	}
	return true
}

// writeFieldValidationErr answers a ValidateName failure: a missing required
// field stays 400; a present-but-invalid value (too long) is
// well-formed-but-invalid (422).
func writeFieldValidationErr(w http.ResponseWriter, r *http.Request, pointer string, err error, pw *problemjson.Writer) {
	if isValueError(err) {
		pw.WriteValidation(w, r, err.Error(), problemjson.FieldError{Pointer: pointer, Detail: err.Error()})
		return
	}
	pw.Write(w, r, http.StatusBadRequest, "validation_error", err.Error())
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in libraryDTO
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	if !validateDTO(w, r, in, h.Problems) {
		return
	}

	dv := in.DefaultView
	if dv == "" {
		dv = "albums"
	}
	icon := in.Icon
	if icon == "" {
		icon = "folder"
	}
	// ShowArtists is a pointer: nil means "visible" (HideArtists=false),
	// true means visible (HideArtists=false), false means hidden (HideArtists=true).
	hideArtists := in.ShowArtists != nil && !*in.ShowArtists
	// A newly created library has no filters, i.e. it selects the whole
	// catalog: filters are not yet accepted over this API.
	lib := &model.Library{
		Name:        in.Name,
		HideArtists: hideArtists,
		DefaultView: dv,
		Icon:        icon,
	}
	if err := h.Store.CreateLibrary(lib); err != nil {
		status, code := mapStoreError(err)
		h.Problems.Write(w, r, status, code, err.Error())
		return
	}
	dto, err := h.modelToDTO(*lib)
	if err != nil {
		h.Problems.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, dto)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	existing, err := h.Store.GetLibrary(id)
	if err != nil {
		status, code := mapStoreError(err)
		h.Problems.Write(w, r, status, code, err.Error())
		return
	}

	var in libraryDTO
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	if !validateDTO(w, r, in, h.Problems) {
		return
	}

	existing.Name = in.Name
	// ShowArtists is a pointer: nil means "keep current", otherwise set HideArtists to the inverse.
	if in.ShowArtists != nil {
		existing.HideArtists = !*in.ShowArtists
	}
	dv := in.DefaultView
	if dv == "" {
		dv = "albums"
	}
	existing.DefaultView = dv
	icon := in.Icon
	if icon == "" {
		icon = "folder"
	}
	existing.Icon = icon

	if err := h.Store.UpdateLibrary(&existing); err != nil {
		status, code := mapStoreError(err)
		h.Problems.Write(w, r, status, code, err.Error())
		return
	}

	dto, err := h.modelToDTO(existing)
	if err != nil {
		h.Problems.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dto)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	// The store's delete does not error on a missing row, so fetch first to
	// answer 404 instead of a silent no-op 204.
	if _, err := h.Store.GetLibrary(id); err != nil {
		status, code := mapStoreError(err)
		h.Problems.Write(w, r, status, code, err.Error())
		return
	}
	if err := h.Store.DeleteLibrary(r.Context(), id); err != nil {
		status, code := mapStoreError(err)
		h.Problems.Write(w, r, status, code, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Map store errors to API status codes.
func mapStoreError(err error) (status int, code string) {
	if errors.Is(err, store.ErrNotFound) {
		return http.StatusNotFound, "not_found"
	}
	if store.IsUniqueViolation(err) {
		return http.StatusConflict, "conflict"
	}
	return http.StatusInternalServerError, "internal"
}
