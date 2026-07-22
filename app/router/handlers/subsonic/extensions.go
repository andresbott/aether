package subsonic

import "net/http"

func (h *Handler) getOpenSubsonicExtensions(w http.ResponseWriter, r *http.Request) {
	writeResponse(w, map[string]any{
		"openSubsonicExtensions": []map[string]any{
			{
				"name":     "musicFolderDefaultView",
				"versions": []int{1},
			},
			{
				"name":     "musicFolderShowArtists",
				"versions": []int{1},
			},
			{
				"name":     "musicFolderIcon",
				"versions": []int{1},
			},
			{
				"name":     "albumList2Index",
				"versions": []int{1},
			},
			{
				"name":     "internetRadioCoverArt",
				"versions": []int{1},
			},
			{
				"name":     "playlistCoverArt",
				"versions": []int{1},
			},
			{
				"name":     "artistCoverArt",
				"versions": []int{1},
			},
			{
				"name":     "genreCoverArt",
				"versions": []int{1},
			},
		},
	})
}
