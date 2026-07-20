package libraries

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
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
	ExcludePatterns   []string   `json:"exclude_patterns"`
	FollowSymlinks    bool       `json:"follow_symlinks"`
	ShowArtists       *bool      `json:"show_artists"`
	DefaultView       string     `json:"default_view"`
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
	// Convert HideArtists (internal, inverted bool) to ShowArtists (API, positive bool).
	// HideArtists=false (zero value, default) means artists are visible, so ShowArtists=true.
	// HideArtists=true means artists are hidden, so ShowArtists=false.
	showArtists := !lib.HideArtists
	return libraryDTO{
		ID:                lib.ID,
		Name:              lib.Name,
		Path:              lib.Path,
		ExcludePatterns:   patterns,
		FollowSymlinks:    lib.FollowSymlinks,
		ShowArtists:       &showArtists,
		DefaultView:       dv,
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

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in libraryDTO
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	if err := validateName(in.Name); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	abs, err := validatePath(in.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	if err := validateExcludePatterns(in.ExcludePatterns); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	if err := validateDefaultView(in.DefaultView); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
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
	// ShowArtists is a pointer: nil means "visible" (HideArtists=false),
	// true means visible (HideArtists=false), false means hidden (HideArtists=true).
	hideArtists := in.ShowArtists != nil && !*in.ShowArtists
	lib := &model.Library{
		Name:            in.Name,
		Path:            abs,
		ExcludePatterns: excludes,
		FollowSymlinks:  in.FollowSymlinks,
		HideArtists:     hideArtists,
		DefaultView:     dv,
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

	var in libraryDTO
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	if err := validateName(in.Name); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	abs, err := validatePath(in.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	if err := validateExcludePatterns(in.ExcludePatterns); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	if err := validateDefaultView(in.DefaultView); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
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
	existing.FollowSymlinks = in.FollowSymlinks
	// ShowArtists is a pointer: nil means "keep current", otherwise set HideArtists to the inverse.
	if in.ShowArtists != nil {
		existing.HideArtists = !*in.ShowArtists
	}
	dv := in.DefaultView
	if dv == "" {
		dv = "albums"
	}
	existing.DefaultView = dv

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
	if err := h.Store.DeleteLibrary(id); err != nil {
		status, code := mapStoreError(err)
		writeError(w, status, code, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Map gorm errors to API status codes.
func mapStoreError(err error) (status int, code string) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return http.StatusNotFound, "not_found"
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return http.StatusConflict, "conflict"
	}
	// SQLite unique-constraint surfaces as a generic error string in some GORM versions.
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "duplicate") {
			return http.StatusConflict, "conflict"
		}
	}
	return http.StatusInternalServerError, "internal"
}
