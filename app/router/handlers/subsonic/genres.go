package subsonic

import (
	"net/http"
	"strconv"

	"github.com/andresbott/aether/internal/assetstore"
)

// updateGenre handles the OpenSubsonic "genreCoverArt" extension: a multipart
// request that carries an optional cover image ("coverFile") or a "coverClear"
// flag for a genre. There is no standard Subsonic updateGenre endpoint, so this
// endpoint exists solely for cover management. Mirrors updateArtist in
// artists.go; covers are keyed by DB ID because genre names may contain
// characters the assetstore key regexp rejects.
func (h *Handler) updateGenre(w http.ResponseWriter, r *http.Request) {
	if !isMultipart(r) {
		writeError(w, 0, "updateGenre requires a multipart request")
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
	if err != nil || kind != "genre" {
		writeError(w, 0, "invalid id")
		return
	}
	genre, err := h.store.GetGenre(id)
	if err != nil {
		writeError(w, 70, "genre not found")
		return
	}

	coverBytes, coverExt, err := readCoverFile(r)
	if err != nil {
		writeError(w, 0, err.Error())
		return
	}

	key := strconv.FormatUint(uint64(genre.ID), 10)
	switch {
	case coverBytes != nil:
		if err := h.assets.PutManual(assetstore.KindGenre, key, coverExt, coverBytes); err != nil {
			writeError(w, 0, "internal error")
			return
		}
	case r.Form.Get("coverClear") == "true":
		_ = h.assets.Delete(assetstore.KindGenre, key)
	}

	writeResponse(w, nil)
}
