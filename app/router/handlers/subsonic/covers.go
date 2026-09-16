package subsonic

import (
	"net/http"
)

// updateManualCover is the shared body of the album, genre, and artist
// cover-write endpoints (updateAlbum / updateGenre / updateArtist): admin-gated
// multipart handlers that store or clear a manual cover keyed by DB ID. They
// differ only in the entity they resolve (via resolveKey) and the assetstore
// kind, so they pass those in rather than repeating identical parse/guard/store
// boilerplate. resolveKey returns the key to write plus any extra keys to clear:
// artist has a second (name-hash) slot to wipe on clear, while album and genre
// have none.
func (h *Handler) updateManualCover(
	w http.ResponseWriter,
	r *http.Request,
	endpoint, idKind, storeKind string,
	resolveKey func(id uint) (writeKey string, clearKeys []string, err error),
) {
	if !h.requireAdmin(w, r) {
		return
	}
	if !isMultipart(r) {
		writeError(w, 0, endpoint+" requires a multipart request")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxCoverRequestBytes)
	if err := r.ParseMultipartForm(coverMultipartMemory); err != nil { //nolint:gosec // G120: body is bounded by http.MaxBytesReader on the previous line
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
	key, clearKeys, err := resolveKey(id)
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
		_ = h.images.Delete(storeKind, key)
		for _, k := range clearKeys {
			_ = h.assets.Delete(storeKind, k)
			_ = h.images.Delete(storeKind, k)
		}
	}

	writeResponse(w, nil)
}
