package subsonic

import (
	"net/http"
	"sort"

	"github.com/andresbott/aether/internal/store"
)

// getGroupings is the albumGroupings extension's index: every grouping with an
// album in the requested library, in the alphabetical-index shape getArtists
// uses so clients can reuse their letter rail.
func (h *Handler) getGroupings(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.libraryScope(w, r)
	if !ok {
		return
	}
	groupings, err := h.store.GetGroupings(&store.GroupingsFilter{Scope: scope})
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	indexMap := make(map[string][]map[string]any)
	for _, g := range groupings {
		letter := firstLetter(g.Name)
		indexMap[letter] = append(indexMap[letter], groupingToMap(g))
	}
	letters := make([]string, 0, len(indexMap))
	for letter := range indexMap {
		letters = append(letters, letter)
	}
	sort.Strings(letters)
	indices := make([]map[string]any, 0, len(letters))
	for _, letter := range letters {
		indices = append(indices, map[string]any{
			"name":     letter,
			"grouping": indexMap[letter],
		})
	}
	writeResponse(w, map[string]any{
		"groupings": map[string]any{"index": indices},
	})
}

// getGrouping answers one grouping with its albums in browsing order — oldest
// first, undated last — narrowed to the library when musicFolderId is given.
func (h *Handler) getGrouping(w http.ResponseWriter, r *http.Request) {
	idStr := paramStr(r, "id")
	if idStr == "" {
		writeError(w, 10, "missing id parameter")
		return
	}
	kind, id, err := decodeID(idStr)
	if err != nil || kind != "grouping" {
		writeError(w, 0, "invalid id")
		return
	}
	scope, ok := h.libraryScope(w, r)
	if !ok {
		return
	}
	filter := &store.GroupingsFilter{Scope: scope}
	grouping, albums, err := h.store.GetGrouping(id, filter)
	if err != nil {
		writeError(w, 70, "grouping not found")
		return
	}
	ids := albumIDs(albums)
	stats, err := h.store.AlbumTrackStats(ids)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	stars := newStarLookup(h.store, requestOwner(r), nil, ids, nil)
	albumList := make([]map[string]any, 0, len(albums))
	songs, duration, year := 0, 0, 0
	for i := range albums {
		al := &albums[i]
		albumList = append(albumList, applyAlbumStats(stars.applyAlbum(albumToMap(al), al.ID), stats, al.ID))
		songs += stats[al.ID].Count
		duration += stats[al.ID].Duration
		if al.Year > 0 && (year == 0 || al.Year < year) {
			year = al.Year
		}
	}
	entry := map[string]any{
		"id":         encodeGroupingID(grouping.ID),
		"name":       grouping.Name,
		"coverArt":   encodeGroupingID(grouping.ID),
		"albumCount": len(albums),
		"songCount":  songs,
		"duration":   duration,
		"album":      albumList,
	}
	if year > 0 {
		entry["year"] = year
	}
	writeResponse(w, map[string]any{"grouping": entry})
}

func groupingToMap(g store.GroupingWithCounts) map[string]any {
	m := map[string]any{
		"id":         encodeGroupingID(g.ID),
		"name":       g.Name,
		"coverArt":   encodeGroupingID(g.ID),
		"albumCount": g.AlbumCount,
		"songCount":  g.SongCount,
		"duration":   g.Duration,
	}
	if g.Year > 0 {
		m["year"] = g.Year
	}
	return m
}
