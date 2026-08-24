package libraries

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type Handler struct {
	Store *store.Store
}

type libraryDTO struct {
	ID                uint       `json:"id"`
	Name              string     `json:"name"`
	Path              string     `json:"path"`
	ExcludePatterns []string `json:"exclude_patterns"`
	// FollowSymlinks and ShowArtists are pointers so an omitted key keeps its
	// default (both true on create) instead of reading as false.
	FollowSymlinks    *bool      `json:"follow_symlinks"`
	ShowArtists       *bool      `json:"show_artists"`
	DefaultView       string     `json:"default_view"`
	Icon              string     `json:"icon"`
	CoverStyle string `json:"cover_style"`
	// Source is "db" for a library managed here or "config" for one declared in
	// the server config file. Config libraries are read-only over this API and
	// the UI renders them without edit/delete actions.
	Source            string     `json:"source"`
	LastScanStartedAt *time.Time `json:"last_scan_started_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	TrackCount        int64      `json:"track_count"`
	PathChanged       bool       `json:"path_changed,omitempty"`
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

// parseBoolParam reads an optional boolean query parameter; an absent value is
// false.
func parseBoolParam(raw string) (bool, error) {
	if raw == "" {
		return false, nil
	}
	return strconv.ParseBool(raw)
}

func (h *Handler) modelToDTO(lib model.Library) (libraryDTO, error) {
	patterns, _ := decodeExcludePatterns(lib.ExcludePatterns)
	count, err := h.Store.CountTracksForLibrary(lib.ID)
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
	cs := lib.CoverStyle
	if cs == "" {
		cs = "auto"
	}
	source := lib.Source
	if source == "" {
		source = model.SourceDB
	}
	// Convert HideArtists (internal, inverted bool) to ShowArtists (API, positive bool).
	// HideArtists=false (zero value, default) means artists are visible, so ShowArtists=true.
	// HideArtists=true means artists are hidden, so ShowArtists=false.
	showArtists := !lib.HideArtists
	followSymlinks := lib.FollowSymlinks
	return libraryDTO{
		ID:                lib.ID,
		Name:              lib.Name,
		Path:              lib.Path,
		ExcludePatterns:   patterns,
		FollowSymlinks:    &followSymlinks,
		ShowArtists:       &showArtists,
		DefaultView:       dv,
		Icon:              icon,
		CoverStyle:        cs,
		Source:            source,
		LastScanStartedAt: lib.LastScanStartedAt,
		CreatedAt:         lib.CreatedAt,
		UpdatedAt:         lib.UpdatedAt,
		TrackCount:        count,
	}, nil
}

func decodeExcludePatterns(s string) ([]string, error) {
	if s == "" {
		return []string{}, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func encodeExcludePatterns(patterns []string) (string, error) {
	if len(patterns) == 0 {
		return "", nil
	}
	b, err := json.Marshal(patterns)
	if err != nil {
		return "", err
	}
	return string(b), nil
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
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]libraryDTO, 0, len(libs))
	for _, lib := range libs {
		dto, err := h.modelToDTO(lib)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, map[string][]libraryDTO{"libraries": out})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	lib, err := h.Store.GetLibrary(id)
	if err != nil {
		status, code := mapStoreError(err)
		writeError(w, status, code, err.Error())
		return
	}
	dto, err := h.modelToDTO(lib)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dto)
}

// validateDTO checks every field of an incoming library payload and returns the
// absolute path. It answers 400 itself; ok is false when the request is done.
func validateDTO(w http.ResponseWriter, in libraryDTO) (abs string, ok bool) {
	if err := ValidateName(in.Name); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return "", false
	}
	abs, err := ValidatePath(in.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return "", false
	}
	for _, check := range []error{
		ValidateExcludePatterns(in.ExcludePatterns),
		ValidateDefaultView(in.DefaultView),
		ValidateIcon(in.Icon),
		ValidateCoverStyle(in.CoverStyle),
	} {
		if check != nil {
			writeError(w, http.StatusBadRequest, "validation_error", check.Error())
			return "", false
		}
	}
	return abs, true
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in libraryDTO
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	abs, ok := validateDTO(w, in)
	if !ok {
		return
	}
	if refuseIfShadowsConfig(w, h.Store, in.Name, abs) {
		return
	}
	excludes, err := encodeExcludePatterns(in.ExcludePatterns)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
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
	cs := in.CoverStyle
	if cs == "" {
		cs = "auto"
	}
	// ShowArtists is a pointer: nil means "visible" (HideArtists=false),
	// true means visible (HideArtists=false), false means hidden (HideArtists=true).
	hideArtists := in.ShowArtists != nil && !*in.ShowArtists
	// FollowSymlinks defaults to true when the key is absent.
	followSymlinks := in.FollowSymlinks == nil || *in.FollowSymlinks
	lib := &model.Library{
		Name:            in.Name,
		Path:            abs,
		ExcludePatterns: excludes,
		FollowSymlinks:  followSymlinks,
		HideArtists:     hideArtists,
		DefaultView:     dv,
		Icon:            icon,
		CoverStyle:      cs,
		Source:          model.SourceDB,
	}
	if err := h.Store.CreateLibrary(lib); err != nil {
		status, code := mapStoreError(err)
		writeError(w, status, code, err.Error())
		return
	}
	dto, err := h.modelToDTO(*lib)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, dto)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	existing, err := h.Store.GetLibrary(id)
	if err != nil {
		status, code := mapStoreError(err)
		writeError(w, status, code, err.Error())
		return
	}
	if refuseIfConfigManaged(w, existing) {
		return
	}

	var in libraryDTO
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	abs, ok := validateDTO(w, in)
	if !ok {
		return
	}
	excludes, err := encodeExcludePatterns(in.ExcludePatterns)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	pathChanged := abs != existing.Path

	existing.Name = in.Name
	existing.Path = abs
	existing.ExcludePatterns = excludes
	// Both bools are pointers: nil means "keep current".
	if in.FollowSymlinks != nil {
		existing.FollowSymlinks = *in.FollowSymlinks
	}
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
	cs := in.CoverStyle
	if cs == "" {
		cs = "auto"
	}
	existing.CoverStyle = cs

	err = h.Store.Transaction(func(tx *store.Store) error {
		if pathChanged {
			if err := tx.DeleteTracksForLibrary(existing.ID); err != nil {
				return err
			}
			if err := tx.DeleteOrphanedAggregates(); err != nil {
				return err
			}
		}
		return tx.UpdateLibrary(&existing)
	})
	if err != nil {
		status, code := mapStoreError(err)
		writeError(w, status, code, err.Error())
		return
	}

	dto, err := h.modelToDTO(existing)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	dto.PathChanged = pathChanged
	writeJSON(w, http.StatusOK, dto)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	existing, err := h.Store.GetLibrary(id)
	if err != nil {
		status, code := mapStoreError(err)
		writeError(w, status, code, err.Error())
		return
	}
	if refuseIfConfigManaged(w, existing) {
		return
	}
	if err := h.Store.DeleteLibrary(id); err != nil {
		status, code := mapStoreError(err)
		writeError(w, status, code, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// configManagedMsg explains why a write was refused. Config libraries are
// rewritten from the config file on every startup, so an accepted edit here
// would silently revert on the next restart — refusing is the honest answer.
const configManagedMsg = "library %q is provisioned from the server config file; " +
	"edit the Libraries section of config.yaml and restart to change it"

// refuseIfConfigManaged answers 409 for a config-provisioned library and
// reports whether the request was refused.
func refuseIfConfigManaged(w http.ResponseWriter, lib model.Library) bool {
	if !lib.IsConfigManaged() {
		return false
	}
	writeError(w, http.StatusConflict, "config_managed", fmt.Sprintf(configManagedMsg, lib.Name))
	return true
}

// refuseIfShadowsConfig rejects creating a library whose name or path is
// already owned by config. Without this the request would fail anyway on the
// unique indexes, but as an opaque "conflict" — this says which config entry is
// in the way. Lookup errors other than "not found" are reported as-is.
func refuseIfShadowsConfig(w http.ResponseWriter, s *store.Store, name, path string) bool {
	lookups := []struct {
		field string
		find  func() (model.Library, error)
	}{
		{"name", func() (model.Library, error) { return s.FindLibraryByName(name) }},
		{"path", func() (model.Library, error) { return s.FindLibraryByPath(path) }},
	}
	for _, lookup := range lookups {
		lib, err := lookup.find()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return true
		}
		if lib.IsConfigManaged() {
			writeError(w, http.StatusConflict, "config_managed", fmt.Sprintf(
				"a library provisioned from the server config file already uses this %s (%q)",
				lookup.field, lib.Name))
			return true
		}
	}
	return false
}

// Map gorm errors to API status codes.
func mapStoreError(err error) (status int, code string) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return http.StatusNotFound, "not_found"
	}
	if store.IsUniqueViolation(err) {
		return http.StatusConflict, "conflict"
	}
	return http.StatusInternalServerError, "internal"
}
