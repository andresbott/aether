package subsonic

import (
	"net/http"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
)

// updateArtist handles the OpenSubsonic "artistCoverArt" extension: a multipart
// request that carries an optional cover image ("coverFile") or a "coverClear"
// flag for an artist. There is no standard Subsonic updateArtist endpoint, so
// this endpoint exists solely for cover management. The parse/guard/store body
// is shared with updateAlbum and updateGenre via updateManualCover; artist is
// the one caller that returns a second key to clear (the name-hash slot).
func (h *Handler) updateArtist(w http.ResponseWriter, r *http.Request) {
	h.updateManualCover(w, r, "updateArtist", "artist", assetstore.KindArtist, func(id uint) (string, []string, error) {
		artist, _, err := h.store.GetArtist(id)
		if err != nil {
			return "", nil, err
		}
		key := assetkey.ArtistOf(artist)
		// Also clear the name-hash slot in case a prior upload was made while
		// the artist was unmatched (or gained an MBID since).
		var clearKeys []string
		if nameHashKey := assetkey.Artist("", artist.NameNorm); nameHashKey != key {
			clearKeys = append(clearKeys, nameHashKey)
		}
		return key, clearKeys, nil
	})
}
