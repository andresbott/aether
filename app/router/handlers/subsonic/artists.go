package subsonic

import (
	"net/http"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
)

// updateArtist handles the OpenSubsonic "artistCoverArt" extension: a multipart
// request that carries an optional cover image ("coverFile") or a "coverClear"
// flag for an artist. There is no standard Subsonic updateArtist endpoint, so
// this endpoint exists solely for cover management. Mirrors
// updatePlaylistMultipart in playlists.go.
func (h *Handler) updateArtist(w http.ResponseWriter, r *http.Request) {
	if !isMultipart(r) {
		writeError(w, 0, "updateArtist requires a multipart request")
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
	if err != nil || kind != "artist" {
		writeError(w, 0, "invalid id")
		return
	}
	artist, _, err := h.store.GetArtist(id)
	if err != nil {
		writeError(w, 70, "artist not found")
		return
	}

	coverBytes, coverExt, err := readCoverFile(r)
	if err != nil {
		writeError(w, 0, err.Error())
		return
	}

	key := assetkey.ArtistOf(artist)
	switch {
	case coverBytes != nil:
		if err := h.assets.PutManual(assetstore.KindArtist, key, coverExt, coverBytes); err != nil {
			writeError(w, 0, "internal error")
			return
		}
	case r.Form.Get("coverClear") == "true":
		_ = h.assets.Delete(assetstore.KindArtist, key)
		// Also clear the name-hash slot in case a prior upload was made while the
		// artist was unmatched (or gained an MBID since).
		if nameHashKey := assetkey.Artist("", artist.NameNorm); nameHashKey != key {
			_ = h.assets.Delete(assetstore.KindArtist, nameHashKey)
		}
	}

	writeResponse(w, nil)
}
