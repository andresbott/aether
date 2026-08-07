package subsonic

import (
	"net/http"
	"time"

	"github.com/andresbott/aether/internal/model"
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
	owner := requestOwner(r)
	filter := &store.AlbumListFilter{
		Genre:     paramStr(r, "genre"),
		FromYear:  paramInt(r, "fromYear", 0),
		ToYear:    paramInt(r, "toYear", 0),
		LibraryID: paramLibraryID(r),
		Owner:     owner,
	}
	albums, err := h.store.GetAlbumList(listType, size, offset, filter)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	ids := albumIDs(albums)
	stats, err := h.store.AlbumTrackStats(ids)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	stars := newStarLookup(h.store, owner, nil, ids, nil)
	albumList := make([]map[string]any, 0, len(albums))
	for _, al := range albums {
		albumList = append(albumList, applyAlbumStats(stars.applyAlbum(albumToMap(&al), al.ID), stats, al.ID))
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
	owner := requestOwner(r)
	songs := starredSongList(h.store, owner, tracks)
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
	owner := requestOwner(r)
	songs := starredSongList(h.store, owner, tracks)
	writeResponse(w, map[string]any{
		"songsByGenre": map[string]any{
			"song": songs,
		},
	})
}

func (h *Handler) getStarred2(w http.ResponseWriter, r *http.Request) {
	owner := requestOwner(r)
	libraryID := paramLibraryID(r)
	starred, err := h.store.GetStarred(owner, &store.StarredFilter{LibraryID: libraryID})
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	albumIDList := albumIDs(starred.Albums)
	stars := newStarLookup(h.store, owner, artistIDs(starred.Artists), albumIDList, trackIDs(starred.Tracks))
	// Album/artist counts, the same ones getAlbumList2 and getArtists emit: the
	// favorites list is rendered by the same rows/cards as the full library, and
	// without these their count columns would sit empty.
	albumCounts, err := h.store.GetArtistAlbumCounts(&store.ArtistsFilter{LibraryID: libraryID})
	if err != nil {
		albumCounts = make(map[uint]int)
	}
	artists := make([]map[string]any, 0, len(starred.Artists))
	for _, a := range starred.Artists {
		artists = append(artists, stars.applyArtist(map[string]any{
			"id":         encodeArtistID(a.ID),
			"name":       a.Name,
			"coverArt":   encodeArtistID(a.ID),
			"albumCount": albumCounts[a.ID],
		}, a.ID))
	}
	albumStats, err := h.store.AlbumTrackStats(albumIDList)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	albums := make([]map[string]any, 0, len(starred.Albums))
	for _, al := range starred.Albums {
		albums = append(albums, applyAlbumStats(stars.applyAlbum(albumToMap(&al), al.ID), albumStats, al.ID))
	}
	songs := make([]map[string]any, 0, len(starred.Tracks))
	for _, t := range starred.Tracks {
		songs = append(songs, stars.applyTrack(trackToChild(&t, t.Album), t.ID))
	}
	playlistIDs := make([]uint, 0, len(starred.Playlists))
	for i := range starred.Playlists {
		playlistIDs = append(playlistIDs, starred.Playlists[i].ID)
	}
	plStarredAt, err := h.store.StarredAt(owner, "playlist", playlistIDs)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	plStats, err := h.store.PlaylistStats(owner, playlistIDs)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	playlists := make([]map[string]any, 0, len(starred.Playlists))
	for i := range starred.Playlists {
		pl := starred.Playlists[i]
		count, _ := h.store.GetPlaylistTrackCount(pl.ID)
		dur, _ := h.store.GetPlaylistDuration(pl.ID)
		var starPtr *time.Time
		if ts, ok := plStarredAt[pl.ID]; ok {
			starPtr = &ts
		}
		var statPtr *store.PlaylistStat
		if st, ok := plStats[pl.ID]; ok {
			statPtr = &st
		}
		playlists = append(playlists, playlistToMap(&pl, int(count), dur, starPtr, statPtr))
	}
	writeResponse(w, map[string]any{
		"starred2": map[string]any{
			"artist":   artists,
			"album":    albums,
			"song":     songs,
			"playlist": playlists,
		},
	})
}

func (h *Handler) getAlbumList2Index(w http.ResponseWriter, r *http.Request) {
	filter := &store.AlbumListFilter{LibraryID: paramLibraryID(r)}
	letters, total, err := h.store.GetAlbumLetterIndex(filter)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	index := make([]map[string]any, 0, len(letters))
	for _, l := range letters {
		index = append(index, map[string]any{
			"name":   l.Letter,
			"offset": l.Offset,
			"count":  l.Count,
		})
	}
	writeResponse(w, map[string]any{
		"albumList2Index": map[string]any{
			"total": total,
			"index": index,
		},
	})
}

func (h *Handler) getNowPlaying(w http.ResponseWriter, r *http.Request) {
	nowPlaying, err := h.store.GetNowPlaying()
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	tracks := make([]model.Track, 0, len(nowPlaying))
	for _, e := range nowPlaying {
		tracks = append(tracks, e.Track)
	}
	// Star state is the VIEWER's, usernames are each entry's real player.
	entries := starredSongList(h.store, requestOwner(r), tracks)
	for i, entry := range entries {
		entry["username"] = nowPlaying[i].Owner
	}
	writeResponse(w, map[string]any{
		"nowPlaying": map[string]any{
			"entry": entries,
		},
	})
}
