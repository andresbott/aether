package subsonic

import (
	"net/http"

	"github.com/andresbott/aether/internal/store"
)

func (h *Handler) search3(w http.ResponseWriter, r *http.Request) {
	query := paramStr(r, "query")
	artistCount := paramInt(r, "artistCount", 20)
	artistOffset := paramInt(r, "artistOffset", 0)
	albumCount := paramInt(r, "albumCount", 20)
	albumOffset := paramInt(r, "albumOffset", 0)
	songCount := paramInt(r, "songCount", 20)
	songOffset := paramInt(r, "songOffset", 0)

	filter := &store.SearchFilter{LibraryID: paramLibraryID(r)}

	artists, err := h.store.SearchArtists(query, artistCount, artistOffset, filter)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	albums, err := h.store.SearchAlbums(query, albumCount, albumOffset, filter)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	songs, err := h.store.SearchSongs(query, songCount, songOffset, filter)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}

	stars := newStarLookup(h.store, artistIDs(artists), albumIDs(albums), trackIDs(songs))
	artistList := make([]map[string]any, 0, len(artists))
	for _, a := range artists {
		artistList = append(artistList, stars.applyArtist(map[string]any{
			"id":       encodeArtistID(a.ID),
			"name":     a.Name,
			"coverArt": encodeArtistID(a.ID),
		}, a.ID))
	}
	albumList := make([]map[string]any, 0, len(albums))
	for _, al := range albums {
		albumList = append(albumList, stars.applyAlbum(albumToMap(&al), al.ID))
	}
	songList := make([]map[string]any, 0, len(songs))
	for _, t := range songs {
		songList = append(songList, stars.applyTrack(trackToChild(&t, t.Album), t.ID))
	}

	writeResponse(w, map[string]any{
		"searchResult3": map[string]any{
			"artist": artistList,
			"album":  albumList,
			"song":   songList,
		},
	})
}
