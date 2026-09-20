package libraries

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/andresbott/aether/internal/libraryfilter"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/internal/store"
	"github.com/go-bumbu/http/problemjson"
	"github.com/gorilla/mux"
)

type Handler struct {
	Store *store.Store
	// Folders is the set of configured scan folders: what a scan_folder filter may
	// name, and where the folder picker may look.
	Folders *scanfolder.Set
	// Problems writes this handler's application/problem+json error responses.
	Problems *problemjson.Writer
}

// maxLibraryBodyBytes caps the JSON bodies this handler decodes, so a
// pathologically large one is cut short instead of buffered whole — the same
// defense in depth the metadata editor's POST-reads apply (maxSelectionBodyBytes).
const maxLibraryBodyBytes = 1 << 20

// warningDTO flags something about a stored library that is not an error —
// today a scan_folder value whose folder is no longer configured.
type warningDTO struct {
	Pointer string `json:"pointer"`
	Detail  string `json:"detail"`
}

// libraryWriteDTO is the body of a create or an update. Filters is a pointer so
// an update can tell "not mentioned" from "set to none": an update that omits
// the key keeps the stored filters — omission must never widen a library to the
// whole catalog — while an explicit [] clears them. On create there is nothing
// to keep, so an absent key means no filters.
type libraryWriteDTO struct {
	Name string `json:"name"`
	// ShowArtists is a pointer so an omitted key keeps its default (true on
	// create, the stored value on update) instead of reading as false.
	ShowArtists *bool                  `json:"show_artists"`
	DefaultView string                 `json:"default_view"`
	Icon        string                 `json:"icon"`
	Filters     *[]model.LibraryFilter `json:"filters"`
}

// libraryDTO is what the API answers with; libraryWriteDTO is what it accepts.
type libraryDTO struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	ShowArtists bool   `json:"show_artists"`
	DefaultView string `json:"default_view"`
	Icon        string `json:"icon"`
	// Filters selects the library's tracks; always emitted as an array (see
	// modelToDTO), never null.
	Filters []model.LibraryFilter `json:"filters"`
	// Warnings flags something about a stored library's filters that is not
	// an error; empty/omitted when there is nothing to report.
	Warnings   []warningDTO `json:"warnings,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
	TrackCount int64        `json:"track_count"`
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
	// The serializer stores a nil slice as JSON null, and a library created
	// with no filters has one — coerce to a non-nil empty slice so the API
	// always emits an array.
	filters := lib.Filters
	if filters == nil {
		filters = []model.LibraryFilter{}
	}
	var warnings []warningDTO
	for _, is := range libraryfilter.Dangling(lib.Filters, h.Folders) {
		warnings = append(warnings, warningDTO{Pointer: is.Pointer, Detail: is.Detail})
	}
	return libraryDTO{
		ID:   lib.ID,
		Name: lib.Name,
		// Convert HideArtists (internal, inverted bool) to ShowArtists (API,
		// positive bool): HideArtists=false (the zero value) means the artists
		// are visible, so ShowArtists=true.
		ShowArtists: !lib.HideArtists,
		DefaultView: dv,
		Icon:        icon,
		Filters:     filters,
		Warnings:    warnings,
		CreatedAt:   lib.CreatedAt,
		UpdatedAt:   lib.UpdatedAt,
		TrackCount:  count,
	}, nil
}

func (h *Handler) Routes(r *mux.Router) {
	r.Path("/libraries/browse").Methods(http.MethodGet).HandlerFunc(h.browse)
	r.Path("/libraries/filter-options").Methods(http.MethodGet).HandlerFunc(h.filterOptions)
	r.Path("/libraries/preview").Methods(http.MethodPost).HandlerFunc(h.preview)
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
func validateDTO(w http.ResponseWriter, r *http.Request, in libraryWriteDTO, pw *problemjson.Writer) bool {
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

// fieldErrors converts libraryfilter issues into problemjson field errors, at
// the same pointers Validate reported them at. Shared by validateFilters and
// preview: both turn a []libraryfilter.Issue into a 422's []FieldError.
func fieldErrors(issues []libraryfilter.Issue) []problemjson.FieldError {
	fields := make([]problemjson.FieldError, 0, len(issues))
	for _, is := range issues {
		fields = append(fields, problemjson.FieldError{Pointer: is.Pointer, Detail: is.Detail})
	}
	return fields
}

// needsAFilter is the one rule that spans fields: a library that hides its
// artists must select something narrower than the whole catalog, or it would
// hide every artist. It is judged against the EFFECTIVE filters — the ones the
// request sends, or the stored ones an update keeps.
var needsAFilter = problemjson.FieldError{
	Pointer: "/show_artists",
	Detail:  "a library that hides its artists needs at least one filter: without any it covers the whole catalog and would hide every artist",
}

// validateFilters checks the filters a request sends, plus needsAFilter. It
// answers the 422 itself, itemising EVERY problem so the form can mark each
// row; ok is false when the request is done.
func (h *Handler) validateFilters(w http.ResponseWriter, r *http.Request, in []model.LibraryFilter, hideArtists bool) (filters []model.LibraryFilter, ok bool) {
	filters, issues := libraryfilter.Validate(in, h.Folders)
	fields := fieldErrors(issues)
	if len(issues) == 0 && hideArtists && len(filters) == 0 {
		fields = append(fields, needsAFilter)
	}
	if len(fields) > 0 {
		h.Problems.WriteValidation(w, r, "the library's filters are not valid", fields...)
		return nil, false
	}
	return filters, true
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLibraryBodyBytes)
	var in libraryWriteDTO
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
	// On create there is nothing to keep, so an absent "filters" key and an
	// explicit [] mean the same thing: no filters, the whole catalog.
	var requested []model.LibraryFilter
	if in.Filters != nil {
		requested = *in.Filters
	}
	filters, ok := h.validateFilters(w, r, requested, hideArtists)
	if !ok {
		return
	}
	lib := &model.Library{
		Name:        in.Name,
		HideArtists: hideArtists,
		DefaultView: dv,
		Icon:        icon,
		Filters:     filters,
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

	r.Body = http.MaxBytesReader(w, r.Body, maxLibraryBodyBytes)
	var in libraryWriteDTO
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	if !validateDTO(w, r, in, h.Problems) {
		return
	}

	// The effective hideArtists is the request's show_artists when present,
	// else whatever is already stored: an update that omits show_artists
	// keeps that value (see the ShowArtists assignment below), so the
	// has-a-filter rule must be judged against the value that will actually
	// be saved, not against a field the caller never touched.
	effectiveHideArtists := existing.HideArtists
	if in.ShowArtists != nil {
		effectiveHideArtists = !*in.ShowArtists
	}
	// Same reasoning for the filters: an update that never mentions them keeps
	// the stored ones, so they are what the has-a-filter rule is judged
	// against. They are NOT re-validated — they passed on the way in, and
	// re-checking them would make a library unrenameable once its scan folder
	// left the config (it reports that as a warning instead).
	filters := existing.Filters
	if in.Filters != nil {
		var ok bool
		if filters, ok = h.validateFilters(w, r, *in.Filters, effectiveHideArtists); !ok {
			return
		}
	} else if effectiveHideArtists && len(filters) == 0 {
		h.Problems.WriteValidation(w, r, "the library's filters are not valid", needsAFilter)
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
	existing.Filters = filters

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
