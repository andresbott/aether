package subsonic

import (
	"net/http"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
)

// updateAlbum handles the OpenSubsonic "albumCoverArt" extension: a multipart
// request that carries an optional cover image ("coverFile") or a "coverClear"
// flag for an album. There is no standard Subsonic updateAlbum endpoint, so this
// endpoint exists solely for cover management. Covers are keyed by DB ID, the
// same key albumCoverMeta reads, so an upload serves through getCoverArt
// immediately. The parse/guard/store body is shared with updateGenre via
// updateManualCover.
func (h *Handler) updateAlbum(w http.ResponseWriter, r *http.Request) {
	h.updateManualCover(w, r, "updateAlbum", "album", assetstore.KindAlbum, func(id uint) (string, error) {
		album, err := h.store.GetAlbum(id)
		if err != nil {
			return "", err
		}
		return assetkey.AlbumOf(album), nil
	})
}
