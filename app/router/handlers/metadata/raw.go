package metadata

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/andresbott/aether/internal/metadataedit"
	"go.senan.xyz/taglib"
	"gorm.io/gorm"
)

// maxRawPaths caps a single raw-tags request; mirrors the identify cap.
const maxRawPaths = 50

type rawTagsResultDTO struct {
	Path string              `json:"path"`
	Tags map[string][]string `json:"tags"`
	// Unsupported lists descriptors of hidden frames: metadata the tag map
	// cannot represent as text (ID3v2 PRIV/GEOB/POPM, unknown binary frames).
	// The descriptors can be sent back verbatim in an update's
	// remove_unsupported list to delete those frames.
	Unsupported []string `json:"unsupported"`
	Error       string   `json:"error,omitempty"`
}

// rawTags serves the complete tag map of the requested files, unfiltered —
// including keys the structured editor does not manage (legacy frames,
// ReplayGain, encoder tags, custom fields).
func (h *Handler) rawTags(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("library_id")
	id, perr := strconv.ParseUint(idStr, 10, 64)
	if perr != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "library_id required")
		return
	}
	paths := r.URL.Query()["paths"]
	if len(paths) == 0 {
		writeErr(w, http.StatusBadRequest, "validation_error", "paths are required")
		return
	}
	if len(paths) > maxRawPaths {
		writeErr(w, http.StatusBadRequest, "validation_error", "too many paths in one request")
		return
	}
	libModel, err := h.Store.GetLibrary(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	readRaw := h.RawTagReader
	if readRaw == nil {
		readRaw = taglib.ReadTags
	}
	readUnsupported := h.UnsupportedReader
	if readUnsupported == nil {
		readUnsupported = taglib.ReadUnsupported
	}
	results := make([]rawTagsResultDTO, 0, len(paths))
	for _, p := range paths {
		abs, rerr := metadataedit.ResolveInLibrary(libModel.Path, p)
		if rerr != nil {
			results = append(results, rawTagsResultDTO{Path: p, Tags: map[string][]string{}, Unsupported: []string{}, Error: rerr.Error()})
			continue
		}
		tagMap, terr := readRaw(abs)
		if terr != nil {
			results = append(results, rawTagsResultDTO{Path: p, Tags: map[string][]string{}, Unsupported: []string{}, Error: terr.Error()})
			continue
		}
		unsupported, uerr := readUnsupported(abs)
		if uerr != nil {
			// The tag map alone is still useful; degrade to "no hidden frames"
			// rather than failing the whole row.
			unsupported = nil
		}
		// Embedded cover art also lands in unsupportedData (APIC/covr/...) but
		// has its own management UI; hide it so it can't be deleted as junk.
		filtered := make([]string, 0, len(unsupported))
		for _, d := range unsupported {
			if !metadataedit.IsCoverDescriptor(d) {
				filtered = append(filtered, d)
			}
		}
		results = append(results, rawTagsResultDTO{Path: p, Tags: tagMap, Unsupported: filtered})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}
