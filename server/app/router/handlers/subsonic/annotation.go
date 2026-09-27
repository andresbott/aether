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

// starrableTypes are the item types star/unstar accept. Songs, albums and
// artists come from the Subsonic spec; playlists are Aether's "playlistStar"
// extension. Genres and radio stations are starrable in neither, so their ids
// are ignored — persisting them would write rows no endpoint can read back.
var starrableTypes = map[string]bool{
	"artist":   true,
	"album":    true,
	"track":    true,
	"playlist": true,
}

func starItems(h *Handler, r *http.Request, doStar bool) {
	// The spec's albumId/artistId parameters are typed, so an id of any other
	// kind is a client error and gets dropped rather than starring the wrong row.
	starParam(h, r, doStar, "id", "")
	starParam(h, r, doStar, "albumId", "album")
	starParam(h, r, doStar, "artistId", "artist")
}

// starParam stars every id in the given request parameter. When want is
// non-empty, only ids of that item type are accepted; when empty, any starrable
// type is.
func starParam(h *Handler, r *http.Request, doStar bool, param, want string) {
	owner := requestOwner(r)
	for _, idStr := range paramStrSlice(r, param) {
		itemType, id, err := decodeID(idStr)
		if err != nil {
			continue
		}
		if want != "" && itemType != want {
			continue
		}
		if !starrableTypes[itemType] {
			continue
		}
		if doStar {
			_ = h.store.Star(owner, itemType, id)
		} else {
			_ = h.store.Unstar(owner, itemType, id)
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
			candidate := time.UnixMilli(ms)
			// Reject timestamps more than a day in the future or before Unix epoch.
			// A malformed timestamp must not corrupt the stored data and make the
			// endpoints fail permanently.
			if candidate.After(time.Now().Add(24*time.Hour)) || candidate.Before(time.Unix(0, 0)) {
				playedAt = time.Now()
			} else {
				playedAt = candidate
			}
		}
	}
	// Playlists are played as a unit and counted separately from their tracks
	// ("playlistScrobble" extension); every other id type is a track play.
	owner := requestOwner(r)
	switch itemType {
	case "playlist":
		err = h.store.RecordPlaylistPlay(owner, id, playedAt)
	default:
		err = h.store.RecordPlay(owner, id, playedAt)
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
