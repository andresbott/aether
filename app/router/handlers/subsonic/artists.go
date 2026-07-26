package subsonic

import (
	"net/http"
	"strconv"

	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
)

// artistCoverKey is the asset-store key for an artist's manually uploaded cover:
// the MusicBrainz ID when the artist is matched (durable across DB drops, and
// shared with the auto-fetcher), otherwise the DB ID so unmatched artists can
// still hold a manual upload. getCoverArt looks up both slots.
func artistCoverKey(a *model.Artist) string {
	if a.MBArtistID != "" {
		return a.MBArtistID
	}
	return strconv.FormatUint(uint64(a.ID), 10)
}

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

	key := artistCoverKey(artist)
	switch {
	case coverBytes != nil:
		if err := h.assets.PutManual(assetstore.KindArtist, key, coverExt, coverBytes); err != nil {
			writeError(w, 0, "internal error")
			return
		}
	case r.Form.Get("coverClear") == "true":
		// Clear the user's upload only. An auto-fetched image is not something
		// the user put here (and neither is a music-folder image), so it must
		// survive and become the served cover again — deleting the whole entry
		// would silently throw away art aether fetched itself.
		_ = h.assets.DeleteManual(assetstore.KindArtist, key)
		// Also clear the DB-ID slot in case a prior upload was made while the
		// artist was unmatched.
		if dbKey := strconv.FormatUint(uint64(artist.ID), 10); dbKey != key {
			_ = h.assets.DeleteManual(assetstore.KindArtist, dbKey)
		}
	}

	writeResponse(w, nil)
}
