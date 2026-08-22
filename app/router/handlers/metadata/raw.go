package metadata

import (
	"net/http"

	"github.com/andresbott/aether/internal/metadataedit"
	"go.senan.xyz/taglib"
)

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
// ReplayGain, encoder tags, custom fields). The selection travels in the POST
// body (library_id + paths[]), decoded the same way as the picture-selection
// endpoints: this was one of the endpoints the production 431 was reported
// against (a large multi-disc selection as a repeated ?paths= query param
// overflowed a reverse proxy's header buffer). See
// docs/superpowers/specs/2026-08-22-metadata-picture-api-header-safe-redesign.md.
func (h *Handler) rawTags(w http.ResponseWriter, r *http.Request) {
	lib, sel, status, err := h.decodeSelection(r)
	if err != nil {
		writeErr(w, r, status, codeFor(status), err.Error())
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
	results := make([]rawTagsResultDTO, 0, len(sel.Paths))
	for _, p := range sel.Paths {
		abs, rerr := metadataedit.ResolveInLibrary(lib.Path, p)
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
