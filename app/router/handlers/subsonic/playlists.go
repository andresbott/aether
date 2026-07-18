package subsonic

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/andresbott/aether/internal/assetstore"
)

// playlistCoverKey is the asset-store key for a playlist's manually uploaded
// cover: the playlist's DB ID, matching how album covers are keyed.
func playlistCoverKey(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

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
			"coverArt":  encodePlaylistID(pl.ID),
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
	h.writePlaylistResponse(w, id)
}

// writePlaylistResponse loads a playlist plus its tracks and writes the full
// Subsonic "playlist" object (with the entry list). Shared by getPlaylist and
// createPlaylist. Writes error 70 if the playlist does not exist.
func (h *Handler) writePlaylistResponse(w http.ResponseWriter, id uint) {
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
			"coverArt":  encodePlaylistID(pl.ID),
			"created":   pl.CreatedAt,
			"changed":   pl.UpdatedAt,
			"entry":     songs,
		},
	})
}

// createPlaylist creates a new playlist or, when playlistId is supplied, updates
// the existing one — replacing its entire track list with the given songId set,
// per the Subsonic spec. Returns the full playlist object.
func (h *Handler) createPlaylist(w http.ResponseWriter, r *http.Request) {
	trackIDs := decodeTrackIDs(paramStrSlice(r, "songId"))
	public := paramBoolPtr(r, "public")

	if idStr := paramStr(r, "playlistId"); idStr != "" {
		h.recreatePlaylist(w, r, idStr, trackIDs, public)
		return
	}

	name := paramStr(r, "name")
	if name == "" {
		writeError(w, 10, "missing name parameter")
		return
	}
	isPublic := public != nil && *public
	pl, err := h.store.CreatePlaylist(name, "admin", isPublic, trackIDs)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	h.writePlaylistResponse(w, pl.ID)
}

// recreatePlaylist handles the createPlaylist update-by-id path: it replaces the
// playlist's entire track list with trackIDs and optionally updates name/public.
func (h *Handler) recreatePlaylist(w http.ResponseWriter, r *http.Request, idStr string, trackIDs []uint, public *bool) {
	kind, id, err := decodeID(idStr)
	if err != nil || kind != "playlist" {
		writeError(w, 0, "invalid id")
		return
	}
	if _, err := h.store.GetPlaylist(id); err != nil {
		writeError(w, 70, "playlist not found")
		return
	}
	var namePtr *string
	if name := paramStr(r, "name"); name != "" {
		namePtr = &name
	}
	if namePtr != nil || public != nil {
		if err := h.store.UpdatePlaylist(id, namePtr, nil, public); err != nil {
			writeError(w, 0, "internal error")
			return
		}
	}
	if err := h.store.SetPlaylistTracks(id, trackIDs); err != nil {
		writeError(w, 0, "internal error")
		return
	}
	h.writePlaylistResponse(w, id)
}

// decodeTrackIDs turns Subsonic song IDs into internal track IDs, silently
// dropping any that fail to decode.
func decodeTrackIDs(songIDs []string) []uint {
	trackIDs := make([]uint, 0, len(songIDs))
	for _, s := range songIDs {
		if _, id, err := decodeID(s); err == nil {
			trackIDs = append(trackIDs, id)
		}
	}
	return trackIDs
}

func (h *Handler) updatePlaylist(w http.ResponseWriter, r *http.Request) {
	if isMultipart(r) {
		h.updatePlaylistMultipart(w, r)
		return
	}
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
	var namePtr, commentPtr *string
	if r.URL.Query().Has("name") {
		name := paramStr(r, "name")
		namePtr = &name
	}
	if r.URL.Query().Has("comment") {
		comment := paramStr(r, "comment")
		commentPtr = &comment
	}
	public := paramBoolPtr(r, "public")
	if namePtr != nil || commentPtr != nil || public != nil {
		if err := h.store.UpdatePlaylist(id, namePtr, commentPtr, public); err != nil {
			writeError(w, 0, "internal error")
			return
		}
	}
	songIDsToAdd := paramStrSlice(r, "songIdToAdd")
	if len(songIDsToAdd) > 0 {
		if err := h.store.AddTracksToPlaylist(id, decodeTrackIDs(songIDsToAdd)); err != nil {
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

// updatePlaylistMultipart handles the OpenSubsonic "playlistCoverArt" extension:
// a multipart updatePlaylist request that carries an optional cover image
// ("coverFile") or a "coverClear" flag, alongside the usual name/comment/public
// fields. Mirrors updateRadioMultipart in radio.go.
func (h *Handler) updatePlaylistMultipart(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRadioRequestBytes)
	if err := r.ParseMultipartForm(radioMultipartMemory); err != nil {
		writeError(w, 0, "invalid multipart body")
		return
	}
	idStr := r.Form.Get("playlistId")
	if idStr == "" {
		writeError(w, 10, "missing playlistId parameter")
		return
	}
	kind, id, err := decodeID(idStr)
	if err != nil || kind != "playlist" {
		writeError(w, 0, "invalid id")
		return
	}
	if _, err := h.store.GetPlaylist(id); err != nil {
		writeError(w, 70, "playlist not found")
		return
	}

	var namePtr, commentPtr *string
	if r.Form.Has("name") {
		name := r.Form.Get("name")
		namePtr = &name
	}
	if r.Form.Has("comment") {
		comment := r.Form.Get("comment")
		commentPtr = &comment
	}
	var public *bool
	if r.Form.Has("public") {
		b := r.Form.Get("public") == "true"
		public = &b
	}
	if namePtr != nil || commentPtr != nil || public != nil {
		if err := h.store.UpdatePlaylist(id, namePtr, commentPtr, public); err != nil {
			writeError(w, 0, "internal error")
			return
		}
	}

	coverBytes, coverExt, err := readCoverFile(r)
	if err != nil {
		writeError(w, 0, err.Error())
		return
	}
	key := playlistCoverKey(id)
	switch {
	case coverBytes != nil:
		if err := h.assets.PutManual(assetstore.KindPlaylist, key, coverExt, coverBytes); err != nil {
			writeError(w, 0, "internal error")
			return
		}
	case r.Form.Get("coverClear") == "true":
		_ = h.assets.Delete(assetstore.KindPlaylist, key)
	}

	writeResponse(w, nil)
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
	if _, err := h.store.GetPlaylist(id); err != nil {
		writeError(w, 70, "playlist not found")
		return
	}
	if err := h.store.DeletePlaylist(id); err != nil {
		writeError(w, 0, "internal error")
		return
	}
	_ = h.assets.Delete(assetstore.KindPlaylist, playlistCoverKey(id))
	writeResponse(w, nil)
}
