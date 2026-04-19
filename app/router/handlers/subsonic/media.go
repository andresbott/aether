package subsonic

import (
	"net/http"
	"os"

	"go.senan.xyz/taglib"
)

func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
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
	filePath, err := h.store.GetTrackFilePath(id)
	if err != nil {
		writeError(w, 70, "song not found")
		return
	}
	http.ServeFile(w, r, filePath)
}

func (h *Handler) getCoverArt(w http.ResponseWriter, r *http.Request) {
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
	var coverPath string
	var albumID uint
	switch itemType {
	case "album":
		album, err := h.store.GetAlbum(id)
		if err != nil {
			writeError(w, 70, "album not found")
			return
		}
		coverPath = album.CoverPath
		albumID = album.ID
	case "track":
		song, err := h.store.GetSong(id)
		if err != nil {
			writeError(w, 70, "song not found")
			return
		}
		if song.Album != nil {
			coverPath = song.Album.CoverPath
			albumID = song.Album.ID
		}
	case "artist":
		artist, _, err := h.store.GetArtist(id)
		if err != nil {
			writeError(w, 70, "artist not found")
			return
		}
		coverPath = artist.CoverPath
	default:
		writeError(w, 0, "unsupported cover art id type")
		return
	}

	if coverPath != "" {
		if _, err := os.Stat(coverPath); err == nil {
			http.ServeFile(w, r, coverPath)
			return
		}
	}

	if albumID > 0 {
		if data := h.readEmbeddedCover(albumID); data != nil {
			w.Header().Set("Content-Type", detectImageContentType(data))
			w.Write(data)
			return
		}
	}

	http.NotFound(w, r)
}

func (h *Handler) readEmbeddedCover(albumID uint) []byte {
	trackPath, err := h.store.GetCoverTrackPath(albumID)
	if err != nil || trackPath == "" {
		return nil
	}
	data, err := taglib.ReadImage(trackPath)
	if err != nil || len(data) == 0 {
		return nil
	}
	return data
}

func detectImageContentType(data []byte) string {
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return "image/png"
	}
	return "application/octet-stream"
}
