package subsonic

import (
	"net/http"
	"strconv"
	"time"

	"github.com/andresbott/aether/internal/store"
)

// playQueueOwner is the single user every queue belongs to until auth lands.
// Playlists hardcode the same owner (see createPlaylist).
const playQueueOwner = "admin"

// savePlayQueue stores the queue with the current track named by id, per the
// Subsonic spec. A call with no ids clears the saved queue.
//
// The id is resolved to a queue index before storage: a queue may hold the same
// track in several slots, and only an index says which one is playing. When the
// id appears more than once the first slot wins — clients that need the exact
// slot use savePlayQueueByIndex ("indexBasedQueue" extension).
func (h *Handler) savePlayQueue(w http.ResponseWriter, r *http.Request) {
	trackIDs := decodeTrackIDs(paramStrSlice(r, "id"))
	if len(trackIDs) == 0 {
		h.clearSavedQueue(w)
		return
	}

	currentStr := paramStr(r, "current")
	if currentStr == "" {
		// OpenSubsonic: current is required unless id is empty.
		writeError(w, 10, "missing current parameter")
		return
	}
	kind, currentID, err := decodeID(currentStr)
	if err != nil || kind != "track" {
		writeError(w, 10, "invalid current parameter")
		return
	}
	currentIndex := -1
	for i, id := range trackIDs {
		if id == currentID {
			currentIndex = i
			break
		}
	}
	if currentIndex == -1 {
		// The spec requires current to be one of the supplied ids; storing a queue
		// whose current track is not in it would be unreadable on restore.
		writeError(w, 10, "current is not part of the play queue")
		return
	}

	h.storeQueue(w, r, trackIDs, currentIndex)
}

// savePlayQueueByIndex is the "indexBasedQueue" extension: it names the current
// track by its 0-based queue slot, which is unambiguous even when the queue holds
// the same track twice.
func (h *Handler) savePlayQueueByIndex(w http.ResponseWriter, r *http.Request) {
	trackIDs := decodeTrackIDs(paramStrSlice(r, "id"))
	if len(trackIDs) == 0 {
		h.clearSavedQueue(w)
		return
	}

	idxStr := paramStr(r, "currentIndex")
	if idxStr == "" {
		writeError(w, 10, "missing currentIndex parameter")
		return
	}
	currentIndex, err := strconv.Atoi(idxStr)
	if err != nil {
		writeError(w, 10, "invalid currentIndex parameter")
		return
	}
	if currentIndex < 0 || currentIndex >= len(trackIDs) {
		writeError(w, 10, "currentIndex is out of range")
		return
	}

	h.storeQueue(w, r, trackIDs, currentIndex)
}

// storeQueue persists a validated queue. position defaults to 0 per the spec, and
// the client name (`c`) is recorded so a client can recognise its own writes.
func (h *Handler) storeQueue(w http.ResponseWriter, r *http.Request, trackIDs []uint, currentIndex int) {
	var positionMs int64
	if s := paramStr(r, "position"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 {
			positionMs = v
		}
	}
	if err := h.store.SavePlayQueue(playQueueOwner, trackIDs, currentIndex, positionMs, paramStr(r, "c"), time.Now()); err != nil {
		writeError(w, 0, "internal error")
		return
	}
	writeResponse(w, nil)
}

func (h *Handler) clearSavedQueue(w http.ResponseWriter) {
	if err := h.store.ClearPlayQueue(playQueueOwner); err != nil {
		writeError(w, 0, "internal error")
		return
	}
	writeResponse(w, nil)
}

func (h *Handler) getPlayQueue(w http.ResponseWriter, r *http.Request) {
	q, ok := h.loadQueue(w)
	if !ok {
		return
	}
	if q == nil {
		// No queue element at all: clients test for presence, and an empty one
		// would read as "a queue with no tracks".
		writeResponse(w, nil)
		return
	}
	m := playQueueToMap(h.store, q)
	if q.CurrentIndex >= 0 && q.CurrentIndex < len(q.Tracks) {
		m["current"] = encodeTrackID(q.Tracks[q.CurrentIndex].ID)
	}
	writeResponse(w, map[string]any{"playQueue": m})
}

// getPlayQueueByIndex is the "indexBasedQueue" extension's read side. It reports
// the same stored queue as getPlayQueue, with currentIndex in place of current.
func (h *Handler) getPlayQueueByIndex(w http.ResponseWriter, r *http.Request) {
	q, ok := h.loadQueue(w)
	if !ok {
		return
	}
	if q == nil {
		writeResponse(w, nil)
		return
	}
	m := playQueueToMap(h.store, q)
	m["currentIndex"] = q.CurrentIndex
	writeResponse(w, map[string]any{"playQueueByIndex": m})
}

// loadQueue fetches the saved queue, writing an error response and returning
// ok=false when the store fails. A nil queue with ok=true means "nothing saved".
func (h *Handler) loadQueue(w http.ResponseWriter) (*store.PlayQueueState, bool) {
	q, err := h.store.GetPlayQueue(playQueueOwner)
	if err != nil {
		writeError(w, 0, "internal error")
		return nil, false
	}
	if q == nil || len(q.Tracks) == 0 {
		return nil, true
	}
	return q, true
}

// playQueueToMap builds the fields both queue shapes share. Entries are full
// Child objects with star state, so a restoring client rebuilds its queue from
// this one response instead of re-fetching every track.
func playQueueToMap(s starGetter, q *store.PlayQueueState) map[string]any {
	return map[string]any{
		"entry":     starredSongList(s, q.Tracks),
		"position":  q.PositionMs,
		"username":  playQueueOwner,
		"changed":   q.Changed.Format(time.RFC3339),
		"changedBy": q.ChangedBy,
	}
}
