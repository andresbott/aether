package libraries

import (
	"encoding/json"
	"net/http"

	"github.com/andresbott/aether/internal/model"
)

// catalogDTO is the root library's settings — the whole catalog, browsed
// without a library.
type catalogDTO struct {
	// SplitViews lists each root view as its own sidebar entry; false gives the
	// root a single entry whose page switches views.
	SplitViews bool `json:"split_views"`
}

// catalogWriteDTO is the body of an update; an omitted field keeps what is
// stored.
type catalogWriteDTO struct {
	SplitViews *bool `json:"split_views"`
}

func (h *Handler) getCatalog(w http.ResponseWriter, r *http.Request) {
	cs, err := h.Store.GetCatalogSettings()
	if err != nil {
		h.Problems.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, catalogDTO{SplitViews: cs.SplitViews})
}

func (h *Handler) updateCatalog(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLibraryBodyBytes)
	var in catalogWriteDTO
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	cs, err := h.Store.GetCatalogSettings()
	if err != nil {
		h.Problems.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if in.SplitViews != nil {
		cs.SplitViews = *in.SplitViews
	}
	if err := h.Store.SaveCatalogSettings(model.CatalogSettings{SplitViews: cs.SplitViews}); err != nil {
		h.Problems.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, catalogDTO{SplitViews: cs.SplitViews})
}
