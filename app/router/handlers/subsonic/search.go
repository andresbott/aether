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
	// "searchGenres" extension. Default 0 so a client unaware of the extension
	// never pays for a query whose results it cannot render.
	genreCount := paramInt(r, "genreCount", 0)
	genreOffset := paramInt(r, "genreOffset", 0)

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
	var genres []store.GenreWithCounts
	if genreCount > 0 {
		genres, err = h.store.SearchGenres(query, genreCount, genreOffset, filter)
		if err != nil {
			writeError(w, 0, "internal error")
			return
		}
	}

	owner := requestOwner(r)
	albumIDList := albumIDs(albums)
	albumStats, err := h.store.AlbumTrackStats(albumIDList)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	stars := newStarLookup(h.store, owner, artistIDs(artists), albumIDList, trackIDs(songs))
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
		albumList = append(albumList, applyAlbumStats(stars.applyAlbum(albumToMap(&al), al.ID), albumStats, al.ID))
	}
	songList := make([]map[string]any, 0, len(songs))
	for _, t := range songs {
		songList = append(songList, stars.applyTrack(trackToChild(&t, t.Album), t.ID))
	}

	result := map[string]any{
		"artist": artistList,
		"album":  albumList,
		"song":   songList,
	}
	// The spec's SearchResult3 has no genre field, so the array is added only for
	// a client that asked via the "searchGenres" extension; the standard shape
	// stays byte-identical for everyone else.
	if genreCount > 0 {
		genreList := make([]map[string]any, 0, len(genres))
		for _, g := range genres {
			genreList = append(genreList, genreToMap(g))
		}
		result["genre"] = genreList
	}

	writeResponse(w, map[string]any{
		"searchResult3": result,
	})
}
