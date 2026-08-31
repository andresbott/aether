package subsonic

import (
	"net/http"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
)

// updateGenre handles the OpenSubsonic "genreCoverArt" extension: a multipart
// request that carries an optional cover image ("coverFile") or a "coverClear"
// flag for a genre. There is no standard Subsonic updateGenre endpoint, so this
// endpoint exists solely for cover management. Covers are keyed by DB ID because
// genre names may contain characters the assetstore key regexp rejects. The
// parse/guard/store body is shared with updateAlbum via updateManualCover.
func (h *Handler) updateGenre(w http.ResponseWriter, r *http.Request) {
	h.updateManualCover(w, r, "updateGenre", "genre", assetstore.KindGenre, func(id uint) (string, error) {
		genre, err := h.store.GetGenre(id)
		if err != nil {
			return "", err
		}
		return assetkey.GenreOf(genre), nil
	})
}
