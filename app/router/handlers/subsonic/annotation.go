package subsonic

import (
	"net/http"
	"strconv"
	"time"
)

func (h *Handler) star(w http.ResponseWriter, r *http.Request) {
	starItems(h, r, true)
	writeResponse(w, nil)
}

func (h *Handler) unstar(w http.ResponseWriter, r *http.Request) {
	starItems(h, r, false)
	writeResponse(w, nil)
}

func starItems(h *Handler, r *http.Request, doStar bool) {
	for _, idStr := range paramStrSlice(r, "id") {
		itemType, id, err := decodeID(idStr)
		if err != nil {
			continue
		}
		if doStar {
			_ = h.store.Star(itemType, id)
		} else {
			_ = h.store.Unstar(itemType, id)
		}
	}
	for _, idStr := range paramStrSlice(r, "albumId") {
		_, id, err := decodeID(idStr)
		if err != nil {
			continue
		}
		if doStar {
			_ = h.store.Star("album", id)
		} else {
			_ = h.store.Unstar("album", id)
		}
	}
	for _, idStr := range paramStrSlice(r, "artistId") {
		_, id, err := decodeID(idStr)
		if err != nil {
			continue
		}
		if doStar {
			_ = h.store.Star("artist", id)
		} else {
			_ = h.store.Unstar("artist", id)
		}
	}
}

func (h *Handler) scrobble(w http.ResponseWriter, r *http.Request) {
	idStr := paramStr(r, "id")
	if idStr == "" {
		writeError(w, 10, "missing id parameter")
		return
	}
	itemType, id, err := decodeID(idStr)
	if err != nil {
		writeError(w, 0, "invalid id")
		return
	}
	playedAt := time.Now()
	if tStr := paramStr(r, "time"); tStr != "" {
		if ms, err := strconv.ParseInt(tStr, 10, 64); err == nil {
			playedAt = time.UnixMilli(ms)
		}
	}
	// Playlists are played as a unit and counted separately from their tracks
	// ("playlistScrobble" extension); every other id type is a track play.
	switch itemType {
	case "playlist":
		err = h.store.RecordPlaylistPlay(id, playedAt)
	default:
		err = h.store.RecordPlay(id, playedAt)
	}
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	writeResponse(w, nil)
}

func (h *Handler) setRating(w http.ResponseWriter, r *http.Request) {
	writeResponse(w, nil)
}
