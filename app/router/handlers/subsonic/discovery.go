package subsonic

import (
	"net/http"
	"strconv"
	"time"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

const (
	discoveryDefaultSize = 48
	discoveryMaxSize     = 200
)

// getDiscovery serves the ranked Discovery feed — the "discovery" OpenSubsonic
// extension.
//
// The response uses per-type arrays (album[], playlist[]) like every other
// Subsonic container, with two additive fields on each entity: "rank", the
// absolute position in the merged cross-type ordering, and "reason", why the
// item surfaced. A client that ignores both still gets two valid lists. The
// internal score is deliberately not exposed: rank carries everything a client
// needs, and publishing scores invites clients to re-sort.
func (h *Handler) getDiscovery(w http.ResponseWriter, r *http.Request) {
	size := paramInt(r, "size", discoveryDefaultSize)
	if size > discoveryMaxSize {
		size = discoveryMaxSize
	}
	if size < 1 {
		size = discoveryDefaultSize
	}
	offset := paramInt(r, "offset", 0)
	if offset < 0 {
		offset = 0
	}

	filter := &store.DiscoveryFilter{LibraryID: paramLibraryID(r)}
	items, err := h.store.DiscoveryFeed(size, offset, discoverySeed(r), filter)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}

	albumRanks := map[uint]store.DiscoveryItem{}
	playlistRanks := map[uint]store.DiscoveryItem{}
	albumIDList := make([]uint, 0, len(items))
	playlistIDList := make([]uint, 0, len(items))
	for _, it := range items {
		if it.Kind == "al" {
			albumRanks[it.AlbumID] = it
			albumIDList = append(albumIDList, it.AlbumID)
		} else {
			playlistRanks[it.PlaylistID] = it
			playlistIDList = append(playlistIDList, it.PlaylistID)
		}
	}

	albums, err := h.discoveryAlbums(albumIDList, albumRanks)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	playlists, err := h.discoveryPlaylists(playlistIDList, playlistRanks)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}

	writeResponse(w, map[string]any{
		"discovery": map[string]any{
			"album":    albums,
			"playlist": playlists,
		},
	})
}

// discoverySeed reads the optional seed. A malformed value falls back to the
// day-derived default rather than erroring: a bad seed should still give the
// user a feed, just not the one they asked for.
func discoverySeed(r *http.Request) int64 {
	if s := paramStr(r, "seed"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			return v
		}
	}
	return time.Now().Truncate(24 * time.Hour).Unix()
}

// discoveryAlbums builds the album array in rank order, reusing albumToMap and
// batched lookups so the entities are byte-identical to getAlbumList2's — and so
// a 48-item page costs a handful of queries rather than one per row.
func (h *Handler) discoveryAlbums(ids []uint, ranks map[uint]store.DiscoveryItem) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	albums, err := h.store.GetAlbumsByIDs(ids)
	if err != nil {
		return nil, err
	}
	stats, err := h.store.AlbumTrackStats(ids)
	if err != nil {
		return nil, err
	}
	stars := newStarLookup(h.store, nil, ids, nil)

	byID := make(map[uint]*model.Album, len(albums))
	for i := range albums {
		byID[albums[i].ID] = &albums[i]
	}
	// Iterate ids, not albums: ids carries the ranking, and GetAlbumsByIDs makes
	// no order guarantee. An album that vanished between ranking and rendering is
	// skipped rather than fatal — the feed is a suggestion, not a transactional
	// read.
	for _, id := range ids {
		al, ok := byID[id]
		if !ok {
			continue
		}
		m := stars.applyAlbum(albumToMap(al), al.ID)
		if st, ok := stats[al.ID]; ok {
			m["songCount"] = st.Count
			m["duration"] = st.Duration
		}
		m["rank"] = ranks[id].Rank
		m["reason"] = ranks[id].Reason
		out = append(out, m)
	}
	return out, nil
}

// discoveryPlaylists builds the playlist array in rank order, reusing
// playlistToMap with batched star and stat lookups.
func (h *Handler) discoveryPlaylists(ids []uint, ranks map[uint]store.DiscoveryItem) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	starredAt, err := h.store.StarredAt("playlist", ids)
	if err != nil {
		return nil, err
	}
	stats, err := h.store.PlaylistStats(ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		pl, err := h.store.GetPlaylist(id)
		if err != nil || pl == nil {
			continue
		}
		count, _ := h.store.GetPlaylistTrackCount(pl.ID)
		dur, _ := h.store.GetPlaylistDuration(pl.ID)
		var starPtr *time.Time
		if ts, ok := starredAt[pl.ID]; ok {
			starPtr = &ts
		}
		var statPtr *store.PlaylistStat
		if st, ok := stats[pl.ID]; ok {
			statPtr = &st
		}
		m := playlistToMap(pl, int(count), dur, starPtr, statPtr)
		m["rank"] = ranks[id].Rank
		m["reason"] = ranks[id].Reason
		out = append(out, m)
	}
	return out, nil
}
