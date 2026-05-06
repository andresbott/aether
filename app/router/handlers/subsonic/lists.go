package subsonic

import (
	"net/http"

	"github.com/andresbott/aether/internal/store"
)

func (h *Handler) getAlbumList2(w http.ResponseWriter, r *http.Request) {
	listType := paramStr(r, "type")
	if listType == "" {
		writeError(w, 10, "missing type parameter")
		return
	}
	size := paramInt(r, "size", 10)
	offset := paramInt(r, "offset", 0)
	if size > 500 {
		size = 500
	}
	filter := &store.AlbumListFilter{
		Genre:     paramStr(r, "genre"),
		FromYear:  paramInt(r, "fromYear", 0),
		ToYear:    paramInt(r, "toYear", 0),
		LibraryID: paramLibraryID(r),
	}
	albums, err := h.store.GetAlbumList(listType, size, offset, filter)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	albumList := make([]map[string]any, 0, len(albums))
	for _, al := range albums {
		albumList = append(albumList, albumToMap(&al))
	}
	writeResponse(w, map[string]any{
		"albumList2": map[string]any{
			"album": albumList,
		},
	})
}

func (h *Handler) getRandomSongs(w http.ResponseWriter, r *http.Request) {
	size := paramInt(r, "size", 10)
	if size > 500 {
		size = 500
	}
	filter := &store.RandomSongsFilter{
		Genre:     paramStr(r, "genre"),
		FromYear:  paramInt(r, "fromYear", 0),
		ToYear:    paramInt(r, "toYear", 0),
		LibraryID: paramLibraryID(r),
	}
	tracks, err := h.store.GetRandomSongs(size, filter)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	songs := make([]map[string]any, 0, len(tracks))
	for _, t := range tracks {
		songs = append(songs, trackToChild(&t, t.Album))
	}
	writeResponse(w, map[string]any{
		"randomSongs": map[string]any{
			"song": songs,
		},
	})
}

func (h *Handler) getSongsByGenre(w http.ResponseWriter, r *http.Request) {
	genre := paramStr(r, "genre")
	if genre == "" {
		writeError(w, 10, "missing genre parameter")
		return
	}
	count := paramInt(r, "count", 10)
	offset := paramInt(r, "offset", 0)
	filter := &store.SearchFilter{LibraryID: paramLibraryID(r)}
	tracks, err := h.store.GetSongsByGenre(genre, count, offset, filter)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	songs := make([]map[string]any, 0, len(tracks))
	for _, t := range tracks {
		songs = append(songs, trackToChild(&t, t.Album))
	}
	writeResponse(w, map[string]any{
		"songsByGenre": map[string]any{
			"song": songs,
		},
	})
}

func (h *Handler) getStarred2(w http.ResponseWriter, r *http.Request) {
	starred, err := h.store.GetStarred(&store.StarredFilter{LibraryID: paramLibraryID(r)})
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	artists := make([]map[string]any, 0, len(starred.Artists))
	for _, a := range starred.Artists {
		artists = append(artists, map[string]any{
			"id":   encodeArtistID(a.ID),
			"name": a.Name,
		})
	}
	albums := make([]map[string]any, 0, len(starred.Albums))
	for _, al := range starred.Albums {
		albums = append(albums, albumToMap(&al))
	}
	songs := make([]map[string]any, 0, len(starred.Tracks))
	for _, t := range starred.Tracks {
		songs = append(songs, trackToChild(&t, t.Album))
	}
	writeResponse(w, map[string]any{
		"starred2": map[string]any{
			"artist": artists,
			"album":  albums,
			"song":   songs,
		},
	})
}

func (h *Handler) getNowPlaying(w http.ResponseWriter, r *http.Request) {
	tracks, err := h.store.GetNowPlaying()
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	entries := make([]map[string]any, 0, len(tracks))
	for _, t := range tracks {
		entry := trackToChild(&t, t.Album)
		entry["username"] = "admin"
		entries = append(entries, entry)
	}
	writeResponse(w, map[string]any{
		"nowPlaying": map[string]any{
			"entry": entries,
		},
	})
}
