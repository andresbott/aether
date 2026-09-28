package libraries

import (
	"encoding/json"
	"net/http"

	"github.com/andresbott/aether/internal/libraryfilter"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

type previewRequest struct {
	Filters []model.LibraryFilter `json:"filters"`
}

type previewDTO struct {
	TrackCount int64 `json:"track_count"`
	AlbumCount int64 `json:"album_count"`
}

// preview answers what a set of filters would select, without storing anything:
// the admin UI's filter builder (LibraryFilterBuilder) calls it as the admin
// edits. It is a read that travels as POST because the filters are a
// structured body (see api-conventions.md).
func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLibraryBodyBytes)
	var in previewRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	filters, issues := libraryfilter.Validate(in.Filters, h.Folders)
	if len(issues) > 0 {
		h.Problems.WriteValidation(w, r, "the filters are not valid", fieldErrors(issues)...)
		return
	}
	scope := store.ScopeOf(filters)
	tracks, err := h.Store.CountTracks(scope)
	if err != nil {
		h.Problems.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	albums, err := h.Store.CountAlbums(scope)
	if err != nil {
		h.Problems.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, previewDTO{TrackCount: tracks, AlbumCount: albums})
}
