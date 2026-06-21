package subsonic

import (
	"net/http"
	"sort"
	"strconv"
)

func (h *Handler) getPlaylists(w http.ResponseWriter, r *http.Request) {
	playlists, err := h.store.GetPlaylists()
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	items := make([]map[string]any, 0, len(playlists))
	for _, pl := range playlists {
		count, _ := h.store.GetPlaylistTrackCount(pl.ID)
		dur, _ := h.store.GetPlaylistDuration(pl.ID)
		items = append(items, map[string]any{
			"id":        encodePlaylistID(pl.ID),
			"name":      pl.Name,
			"comment":   pl.Comment,
			"owner":     pl.Owner,
			"public":    pl.Public,
			"songCount": count,
			"duration":  dur,
			"created":   pl.CreatedAt,
			"changed":   pl.UpdatedAt,
		})
	}
	writeResponse(w, map[string]any{
		"playlists": map[string]any{
			"playlist": items,
		},
	})
}

func (h *Handler) getPlaylist(w http.ResponseWriter, r *http.Request) {
	idStr := paramStr(r, "id")
	if idStr == "" {
		writeError(w, 10, "missing id parameter")
		return
	}
	_, id, err := decodeID(idStr)
	if err != nil {
		writeError(w, 0, "invalid id")
		return
	}
	pl, err := h.store.GetPlaylist(id)
	if err != nil {
		writeError(w, 70, "playlist not found")
		return
	}
	tracks, err := h.store.GetPlaylistTracks(id)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	songs := make([]map[string]any, 0, len(tracks))
	var dur int
	for _, t := range tracks {
		songs = append(songs, trackToChild(&t, t.Album))
		dur += t.Duration
	}
	writeResponse(w, map[string]any{
		"playlist": map[string]any{
			"id":        encodePlaylistID(pl.ID),
			"name":      pl.Name,
			"comment":   pl.Comment,
			"owner":     pl.Owner,
			"public":    pl.Public,
			"songCount": len(tracks),
			"duration":  dur,
			"created":   pl.CreatedAt,
			"changed":   pl.UpdatedAt,
			"entry":     songs,
		},
	})
}

func (h *Handler) createPlaylist(w http.ResponseWriter, r *http.Request) {
	name := paramStr(r, "name")
	if name == "" {
		writeError(w, 10, "missing name parameter")
		return
	}
	songIDs := paramStrSlice(r, "songId")
	trackIDs := make([]uint, 0, len(songIDs))
	for _, s := range songIDs {
		_, id, err := decodeID(s)
		if err == nil {
			trackIDs = append(trackIDs, id)
		}
	}
	pl, err := h.store.CreatePlaylist(name, "admin", trackIDs)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	writeResponse(w, map[string]any{
		"playlist": map[string]any{
			"id":      encodePlaylistID(pl.ID),
			"name":    pl.Name,
			"created": pl.CreatedAt,
		},
	})
}

func (h *Handler) updatePlaylist(w http.ResponseWriter, r *http.Request) {
	idStr := paramStr(r, "playlistId")
	if idStr == "" {
		writeError(w, 10, "missing playlistId parameter")
		return
	}
	_, id, err := decodeID(idStr)
	if err != nil {
		writeError(w, 0, "invalid id")
		return
	}
	name := paramStr(r, "name")
	comment := paramStr(r, "comment")
	if name != "" || comment != "" {
		name, comment = h.fillPlaylistDefaults(id, name, comment)
		if err := h.store.UpdatePlaylist(id, name, comment); err != nil {
			writeError(w, 0, "internal error")
			return
		}
	}
	songIDsToAdd := paramStrSlice(r, "songIdToAdd")
	if len(songIDsToAdd) > 0 {
		trackIDs := make([]uint, 0, len(songIDsToAdd))
		for _, s := range songIDsToAdd {
			_, tid, err := decodeID(s)
			if err == nil {
				trackIDs = append(trackIDs, tid)
			}
		}
		if err := h.store.AddTracksToPlaylist(id, trackIDs); err != nil {
			writeError(w, 0, "internal error")
			return
		}
	}
	indicesToRemove := paramStrSlice(r, "songIndexToRemove")
	indices := make([]int, 0, len(indicesToRemove))
	for _, s := range indicesToRemove {
		if idx, err := strconv.Atoi(s); err == nil {
			indices = append(indices, idx)
		}
	}
	sort.Ints(indices)
	for i := len(indices) - 1; i >= 0; i-- {
		if err := h.store.RemoveTrackFromPlaylist(id, indices[i]); err != nil {
			writeError(w, 0, "internal error")
			return
		}
	}
	writeResponse(w, nil)
}

// fillPlaylistDefaults backfills an empty name or comment from the stored
// playlist so a partial update doesn't blank out the field that was omitted.
func (h *Handler) fillPlaylistDefaults(id uint, name, comment string) (string, string) {
	pl, _ := h.store.GetPlaylist(id)
	if pl == nil {
		return name, comment
	}
	if name == "" {
		name = pl.Name
	}
	if comment == "" {
		comment = pl.Comment
	}
	return name, comment
}

func (h *Handler) deletePlaylist(w http.ResponseWriter, r *http.Request) {
	idStr := paramStr(r, "id")
	if idStr == "" {
		writeError(w, 10, "missing id parameter")
		return
	}
	_, id, err := decodeID(idStr)
	if err != nil {
		writeError(w, 0, "invalid id")
		return
	}
	if err := h.store.DeletePlaylist(id); err != nil {
		writeError(w, 70, "playlist not found")
		return
	}
	writeResponse(w, nil)
}
