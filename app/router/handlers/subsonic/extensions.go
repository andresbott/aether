package subsonic

import "net/http"

func (h *Handler) getOpenSubsonicExtensions(w http.ResponseWriter, r *http.Request) {
	writeResponse(w, map[string]any{
		"openSubsonicExtensions": []map[string]any{
			{
				// v1: views (discover/artists/releases) and defaultView, one
				// of them, on every getMusicFolders entry.
				// v2: adds splitViews, whether a client should list each view
				// as its own navigation entry rather than one for the folder,
				// and a catalog descriptor (views/defaultView/splitViews, no
				// id) on musicFolders for the root, browsed without a folder.
				"name":     "musicFolderViews",
				"versions": []int{1, 2},
			},
			{
				// Material Symbols icon name in Google's snake_case (e.g.
				// "queue_music") carried on each getMusicFolders entry.
				"name":     "musicFolderIcon",
				"versions": []int{1},
			},
			{
				"name":     "albumList2Index",
				"versions": []int{1},
			},
			{
				// v1: the releaseType parameter on getAlbumList2/getAlbumList2Index.
				// v2: adds getReleaseTypes, listing the types worth filtering by.
				"name":     "releaseTypeFilter",
				"versions": []int{1, 2},
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
			{
				"name":     "albumCoverArt",
				"versions": []int{1},
			},
			{
				"name":     "playlistStar",
				"versions": []int{1},
			},
			{
				"name":     "playlistScrobble",
				"versions": []int{1},
			},
			{
				"name":     "playlistStats",
				"versions": []int{1},
			},
			{
				"name":     "discovery",
				"versions": []int{1},
			},
			{
				"name":     "indexBasedQueue",
				"versions": []int{1},
			},
			{
				"name":     "searchGenres",
				"versions": []int{1},
			},
			{
				"name":     "apiKeyAuthentication",
				"versions": []int{1},
			},
			{
				"name":     "generatedCovers",
				"versions": []int{1},
			},
		},
	})
}
