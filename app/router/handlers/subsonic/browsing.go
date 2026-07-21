package subsonic

import (
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

func (h *Handler) getMusicFolders(w http.ResponseWriter, r *http.Request) {
	libs, err := h.store.ListLibraries()
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	folders := make([]map[string]any, 0, len(libs))
	for _, lib := range libs {
		dv := lib.DefaultView
		if dv == "" {
			dv = "albums"
		}
		folders = append(folders, map[string]any{
			"id":          lib.ID,
			"name":        lib.Name,
			"defaultView": dv,
			"showArtists": !lib.HideArtists,
		})
	}
	writeResponse(w, map[string]any{
		"musicFolders": map[string]any{
			"musicFolder": folders,
		},
	})
}

func (h *Handler) getArtists(w http.ResponseWriter, r *http.Request) {
	h.writeArtistIndex(w, r, "artists")
}

func (h *Handler) getIndexes(w http.ResponseWriter, r *http.Request) {
	h.writeArtistIndex(w, r, "indexes")
}

func (h *Handler) writeArtistIndex(w http.ResponseWriter, r *http.Request, key string) {
	filter := &store.ArtistsFilter{LibraryID: paramLibraryID(r)}
	artists, err := h.store.GetArtists(filter)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	albumCounts, err := h.store.GetArtistAlbumCounts(filter)
	if err != nil {
		albumCounts = make(map[uint]int)
	}
	indexMap := make(map[string][]map[string]any)
	for _, a := range artists {
		letter := firstLetter(a.Name)
		indexMap[letter] = append(indexMap[letter], map[string]any{
			"id":         encodeArtistID(a.ID),
			"name":       a.Name,
			"coverArt":   encodeArtistID(a.ID),
			"albumCount": albumCounts[a.ID],
		})
	}
	letters := make([]string, 0, len(indexMap))
	for letter := range indexMap {
		letters = append(letters, letter)
	}
	sort.Strings(letters)
	indices := make([]map[string]any, 0, len(letters))
	for _, letter := range letters {
		indices = append(indices, map[string]any{
			"name":   letter,
			"artist": indexMap[letter],
		})
	}
	writeResponse(w, map[string]any{
		key: map[string]any{
			"index": indices,
		},
	})
}

func (h *Handler) getArtist(w http.ResponseWriter, r *http.Request) {
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
	artist, albums, err := h.store.GetArtist(id)
	if err != nil {
		writeError(w, 70, "artist not found")
		return
	}
	albumList := make([]map[string]any, 0, len(albums))
	for _, al := range albums {
		albumList = append(albumList, albumToMap(&al))
	}
	writeResponse(w, map[string]any{
		"artist": map[string]any{
			"id":       encodeArtistID(artist.ID),
			"name":     artist.Name,
			"coverArt": encodeArtistID(artist.ID),
			"album":    albumList,
		},
	})
}

func (h *Handler) getAlbum(w http.ResponseWriter, r *http.Request) {
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
	album, err := h.store.GetAlbum(id)
	if err != nil {
		writeError(w, 70, "album not found")
		return
	}
	songs := make([]map[string]any, 0, len(album.Tracks))
	for _, t := range album.Tracks {
		songs = append(songs, trackToChild(t, album))
	}
	result := albumToMap(album)
	result["song"] = songs
	writeResponse(w, map[string]any{
		"album": result,
	})
}

func (h *Handler) getSong(w http.ResponseWriter, r *http.Request) {
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
	track, err := h.store.GetSong(id)
	if err != nil {
		writeError(w, 70, "song not found")
		return
	}
	writeResponse(w, map[string]any{
		"song": trackToChild(track, track.Album),
	})
}

func (h *Handler) getGenres(w http.ResponseWriter, r *http.Request) {
	genres, err := h.store.GetGenres()
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	genreList := make([]map[string]any, 0, len(genres))
	for _, g := range genres {
		genreList = append(genreList, map[string]any{
			"value":      g.Name,
			"songCount":  g.SongCount,
			"albumCount": g.AlbumCount,
			// OpenSubsonic "genreCoverArt" extension: a cover-art id per genre.
			"coverArt": encodeGenreID(g.ID),
		})
	}
	writeResponse(w, map[string]any{
		"genres": map[string]any{
			"genre": genreList,
		},
	})
}

// Helpers

func firstLetter(name string) string {
	for _, r := range name {
		if unicode.IsLetter(r) {
			return strings.ToUpper(string(r))
		}
	}
	return "#"
}

func albumToMap(al *model.Album) map[string]any {
	m := map[string]any{
		"id":       encodeAlbumID(al.ID),
		"name":     al.Name,
		"year":     al.Year,
		"coverArt": encodeAlbumID(al.ID),
		"created":  al.CreatedAt,
	}
	if len(al.Artists) > 0 {
		m["artist"] = al.Artists[0].Name
		m["artistId"] = encodeArtistID(al.Artists[0].ID)
	}
	if len(al.Genres) > 0 {
		m["genre"] = al.Genres[0].Name
	}
	if len(al.Tracks) > 0 {
		m["songCount"] = len(al.Tracks)
		var dur int
		for _, t := range al.Tracks {
			dur += t.Duration
		}
		m["duration"] = dur
	}
	return m
}

func trackToChild(t *model.Track, album *model.Album) map[string]any {
	m := map[string]any{
		"id":          encodeTrackID(t.ID),
		"title":       t.Title,
		"track":       t.TrackNumber,
		"discNumber":  t.DiscNumber,
		"year":        t.Year,
		"size":        t.FileSize,
		"duration":    t.Duration,
		"bitRate":     t.Bitrate,
		"suffix":      strings.TrimPrefix(filepath.Ext(t.Filename), "."),
		"contentType": mimeType(t.Filename),
		"isDir":       false,
		"type":        "music",
		"created":     t.CreatedAt,
	}
	if album != nil {
		m["parent"] = encodeAlbumID(album.ID)
		m["album"] = album.Name
		m["albumId"] = encodeAlbumID(album.ID)
		m["coverArt"] = encodeAlbumID(album.ID)
	}
	if len(t.Artists) > 0 {
		m["artist"] = t.Artists[0].Name
		m["artistId"] = encodeArtistID(t.Artists[0].ID)
	}
	if len(t.Genres) > 0 {
		m["genre"] = t.Genres[0].Name
	}
	return m
}

func mimeType(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".mp3":
		return "audio/mpeg"
	case ".flac":
		return "audio/flac"
	case ".ogg":
		return "audio/ogg"
	case ".opus":
		return "audio/opus"
	case ".m4a", ".aac":
		return "audio/mp4"
	case ".wav":
		return "audio/wav"
	case ".wma":
		return "audio/x-ms-wma"
	default:
		return "application/octet-stream"
	}
}
