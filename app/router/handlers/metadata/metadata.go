package metadata

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/andresbott/aether/internal/metadataedit"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// Handler serves the metadata editor API. It depends only on the library
// portion of the store plus a tags.Reader for on-demand per-file reads.
type Handler struct {
	Store  *store.Store
	Reader tags.Reader
}

type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// Routes mounts the handler under an already-subrouted mux.Router.
// Endpoints live beneath /metadata/.
func (h *Handler) Routes(r *mux.Router) {
	r.Path("/metadata/folders").Methods(http.MethodGet).HandlerFunc(h.folders)
	r.Path("/metadata/tracks").Methods(http.MethodGet).HandlerFunc(h.tracks)
	r.Path("/metadata/tracks").Methods(http.MethodPut).HandlerFunc(h.updateTracks)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, apiError{Error: msg, Code: code})
}

func (h *Handler) resolveLibraryRel(r *http.Request) (lib *librarySummary, absPath string, httpStatus int, err error) {
	idStr := r.URL.Query().Get("library_id")
	id, perr := strconv.ParseUint(idStr, 10, 64)
	if perr != nil {
		return nil, "", http.StatusBadRequest, errors.New("library_id required")
	}
	libModel, gerr := h.Store.GetLibrary(uint(id))
	if gerr != nil {
		if errors.Is(gerr, gorm.ErrRecordNotFound) {
			return nil, "", http.StatusNotFound, gerr
		}
		return nil, "", http.StatusInternalServerError, gerr
	}
	rel := r.URL.Query().Get("path")
	abs, rerr := metadataedit.ResolveInLibrary(libModel.Path, rel)
	if rerr != nil {
		return nil, "", http.StatusBadRequest, rerr
	}
	return &librarySummary{
		ID:                    libModel.ID,
		Path:                  libModel.Path,
		MultiValueArtist:      libModel.MultiValueArtist,
		MultiValueAlbumArtist: libModel.MultiValueAlbumArtist,
	}, abs, 0, nil
}

type librarySummary struct {
	ID                    uint
	Path                  string
	MultiValueArtist      string
	MultiValueAlbumArtist string
}

type folderDTO struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	HasSubfolders bool   `json:"has_subfolders"`
}

func (h *Handler) folders(w http.ResponseWriter, r *http.Request) {
	_, abs, status, err := h.resolveLibraryRel(r)
	if err != nil {
		writeErr(w, status, codeFor(status), err.Error())
		return
	}
	folders, err := metadataedit.ListFolders(abs)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]folderDTO, 0, len(folders))
	for _, f := range folders {
		out = append(out, folderDTO{Name: f.Name, Path: f.Path, HasSubfolders: f.HasSubfolders})
	}
	writeJSON(w, http.StatusOK, map[string]any{"folders": out})
}

type trackDTO struct {
	Path             string   `json:"path"`
	Name             string   `json:"name"`
	Title            string   `json:"title"`
	Artists          []string `json:"artists"`
	AlbumArtists     []string `json:"album_artists"`
	Album            string   `json:"album"`
	Year             int      `json:"year"`
	Compilation      bool     `json:"compilation"`
	MBArtistIDs      []string `json:"mb_artist_ids"`
	MBAlbumArtistIDs []string `json:"mb_album_artist_ids"`
	Error            string   `json:"error,omitempty"`
}

func (h *Handler) tracks(w http.ResponseWriter, r *http.Request) {
	lib, abs, status, err := h.resolveLibraryRel(r)
	if err != nil {
		writeErr(w, status, codeFor(status), err.Error())
		return
	}
	rows, err := metadataedit.ListTracks(lib.Path, abs, h.Reader)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]trackDTO, 0, len(rows))
	for _, t := range rows {
		out = append(out, trackDTO{
			Path:             t.Path,
			Name:             t.Name,
			Title:            t.Title,
			Artists:          t.Artists,
			AlbumArtists:     t.AlbumArtists,
			Album:            t.Album,
			Year:             t.Year,
			Compilation:      t.Compilation,
			MBArtistIDs:      t.MBArtistIDs,
			MBAlbumArtistIDs: t.MBAlbumArtistIDs,
			Error:            t.Error,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"tracks": out})
}

type updateRequest struct {
	LibraryID uint     `json:"library_id"`
	Paths     []string `json:"paths"`
	Fields    fields   `json:"fields"`
}

type fields struct {
	Title            *string            `json:"title,omitempty"`
	Album            *string            `json:"album,omitempty"`
	Artists          *[]string          `json:"artists,omitempty"`
	AlbumArtists     *[]string          `json:"album_artists,omitempty"`
	Year             *int               `json:"year,omitempty"`
	Compilation      *bool              `json:"compilation,omitempty"`
	ArtistMBIDs      *map[string]string `json:"artist_mbids,omitempty"`
	AlbumArtistMBIDs *map[string]string `json:"album_artist_mbids,omitempty"`
}

type updateResult struct {
	Path  string `json:"path"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func (h *Handler) updateTracks(w http.ResponseWriter, r *http.Request) {
	var body updateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	if body.LibraryID == 0 || len(body.Paths) == 0 {
		writeErr(w, http.StatusBadRequest, "validation_error", "library_id and paths are required")
		return
	}
	libModel, err := h.Store.GetLibrary(body.LibraryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	patch := metadataedit.Patch{
		Title:           body.Fields.Title,
		Album:           body.Fields.Album,
		Artists:         body.Fields.Artists,
		AlbumArtists:    body.Fields.AlbumArtists,
		Year:            body.Fields.Year,
		Compilation:     body.Fields.Compilation,
		ArtistMBID:      body.Fields.ArtistMBIDs,
		AlbumArtistMBID: body.Fields.AlbumArtistMBIDs,
	}
	needMB := body.Fields.ArtistMBIDs != nil || body.Fields.AlbumArtistMBIDs != nil
	cfg := metadataedit.LibraryCfg{
		MultiValueArtist:      libModel.MultiValueArtist,
		MultiValueAlbumArtist: libModel.MultiValueAlbumArtist,
	}

	resolved := make([]string, 0, len(body.Paths))
	for _, p := range body.Paths {
		abs, err := metadataedit.ResolveInLibrary(libModel.Path, p)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
		resolved = append(resolved, abs)
	}

	results := make([]updateResult, 0, len(resolved))
	anyOK := false
	for i, abs := range resolved {
		var cur metadataedit.CurrentTags
		if needMB {
			meta, rerr := h.Reader.Read(abs)
			if rerr != nil {
				results = append(results, updateResult{Path: body.Paths[i], OK: false, Error: rerr.Error()})
				continue
			}
			cur = metadataedit.CurrentTags{
				Artists:          meta.Artist,
				ArtistMBIDs:      meta.MBArtistID,
				AlbumArtists:     meta.AlbumArtist,
				AlbumArtistMBIDs: meta.MBAlbumArtistID,
			}
		}
		if err := metadataedit.WriteMetadata(abs, patch, cfg, cur); err != nil {
			results = append(results, updateResult{Path: body.Paths[i], OK: false, Error: err.Error()})
			continue
		}
		results = append(results, updateResult{Path: body.Paths[i], OK: true})
		anyOK = true
	}
	status := http.StatusOK
	if !anyOK {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, map[string]any{"results": results})
}

func codeFor(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "validation_error"
	case http.StatusNotFound:
		return "not_found"
	default:
		return "internal"
	}
}
