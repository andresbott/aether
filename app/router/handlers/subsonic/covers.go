package subsonic

import (
	"net/http"
)

// updateManualCover is the shared body of the album and genre cover-write
// endpoints (updateAlbum / updateGenre): admin-gated multipart handlers that
// store or clear a manual cover keyed by DB ID. The two differ only in the
// entity they resolve (via resolveKey) and the assetstore kind, so they pass
// those in rather than repeating identical parse/guard/store boilerplate.
// updateArtist stays separate: it resolves a second (name-hash) key it must
// also clear.
func (h *Handler) updateManualCover(
	w http.ResponseWriter,
	r *http.Request,
	endpoint, idKind, storeKind string,
	resolveKey func(id uint) (string, error),
) {
	if !h.requireAdmin(w, r) {
		return
	}
	if !isMultipart(r) {
		writeError(w, 0, endpoint+" requires a multipart request")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRadioRequestBytes)
	if err := r.ParseMultipartForm(radioMultipartMemory); err != nil { //nolint:gosec // G120: body is bounded by http.MaxBytesReader on the previous line
		writeError(w, 0, "invalid multipart body")
		return
	}
	idStr := r.Form.Get("id")
	if idStr == "" {
		writeError(w, 10, "missing id parameter")
		return
	}
	kind, id, err := decodeID(idStr)
	if err != nil || kind != idKind {
		writeError(w, 0, "invalid id")
		return
	}
	key, err := resolveKey(id)
	if err != nil {
		writeError(w, 70, idKind+" not found")
		return
	}

	coverBytes, coverExt, err := readCoverFile(r)
	if err != nil {
		writeError(w, 0, err.Error())
		return
	}

	switch {
	case coverBytes != nil:
		if err := h.assets.PutManual(storeKind, key, coverExt, coverBytes); err != nil {
			writeError(w, 0, "internal error")
			return
		}
	case r.Form.Get("coverClear") == "true":
		_ = h.assets.Delete(storeKind, key)
	}

	writeResponse(w, nil)
}
