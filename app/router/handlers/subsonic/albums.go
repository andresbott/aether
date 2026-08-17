package subsonic

import (
	"net/http"
	"strconv"

	"github.com/andresbott/aether/internal/assetstore"
)

// updateAlbum handles the OpenSubsonic "albumCoverArt" extension: a multipart
// request that carries an optional cover image ("coverFile") or a "coverClear"
// flag for an album. There is no standard Subsonic updateAlbum endpoint, so this
// endpoint exists solely for cover management. Mirrors updateGenre in genres.go;
// covers are keyed by DB ID, the same key albumCoverMeta reads, so an upload
// serves through getCoverArt immediately.
func (h *Handler) updateAlbum(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if !isMultipart(r) {
		writeError(w, 0, "updateAlbum requires a multipart request")
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
	if err != nil || kind != "album" {
		writeError(w, 0, "invalid id")
		return
	}
	album, err := h.store.GetAlbum(id)
	if err != nil {
		writeError(w, 70, "album not found")
		return
	}

	coverBytes, coverExt, err := readCoverFile(r)
	if err != nil {
		writeError(w, 0, err.Error())
		return
	}

	key := strconv.FormatUint(uint64(album.ID), 10)
	switch {
	case coverBytes != nil:
		if err := h.assets.PutManual(assetstore.KindAlbum, key, coverExt, coverBytes); err != nil {
			writeError(w, 0, "internal error")
			return
		}
	case r.Form.Get("coverClear") == "true":
		_ = h.assets.Delete(assetstore.KindAlbum, key)
	}

	writeResponse(w, nil)
}
