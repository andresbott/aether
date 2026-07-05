package subsonic

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/covergen"
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

type coverMeta struct {
	coverPath string
	albumID   uint
	seed      string
}

// resolveCoverMeta looks up cover metadata for the given item type and ID.
// It writes an HTTP error and returns false if the item cannot be found or the
// type is unsupported.
func (h *Handler) resolveCoverMeta(w http.ResponseWriter, itemType string, id uint) (coverMeta, bool) {
	switch itemType {
	case "album":
		album, err := h.store.GetAlbum(id)
		if err != nil {
			writeError(w, 70, "album not found")
			return coverMeta{}, false
		}
		return coverMeta{coverPath: album.CoverPath, albumID: album.ID, seed: album.AlbumArtistNorm + "|" + album.NameNorm}, true
	case "track":
		song, err := h.store.GetSong(id)
		if err != nil {
			writeError(w, 70, "song not found")
			return coverMeta{}, false
		}
		if song.Album != nil {
			return coverMeta{coverPath: song.Album.CoverPath, albumID: song.Album.ID, seed: song.Album.AlbumArtistNorm + "|" + song.Album.NameNorm}, true
		}
		return coverMeta{}, true
	case "artist":
		artist, _, err := h.store.GetArtist(id)
		if err != nil {
			writeError(w, 70, "artist not found")
			return coverMeta{}, false
		}
		meta := coverMeta{seed: artist.NameNorm}
		if artist.MBArtistID != "" {
			if p, ok := h.assets.Get(assetstore.KindArtist, artist.MBArtistID); ok {
				meta.coverPath = p
			}
		}
		return meta, true
	case "radio":
		station, err := h.store.GetInternetRadioStation(id)
		if err != nil {
			writeError(w, 70, "radio station not found")
			return coverMeta{}, false
		}
		meta := coverMeta{seed: station.Name}
		if p, ok := h.assets.Get(assetstore.KindRadio, RadioKey(station.StreamURL)); ok {
			meta.coverPath = p
		}
		return meta, true
	default:
		writeError(w, 0, "unsupported cover art id type")
		return coverMeta{}, false
	}
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
	meta, ok := h.resolveCoverMeta(w, itemType, id)
	if !ok {
		return
	}

	// Force revalidation on every request. Without this, browsers may
	// heuristically cache a response (e.g. the generated-avatar fallback
	// served before an artist has a fetched image) and keep serving it from
	// cache after the underlying file changes, since the URL is unchanged.
	w.Header().Set("Cache-Control", "no-cache")

	if meta.coverPath != "" {
		if _, err := os.Stat(meta.coverPath); err == nil {
			http.ServeFile(w, r, meta.coverPath)
			return
		}
	}

	if meta.albumID > 0 {
		if data := h.readEmbeddedCover(meta.albumID); data != nil {
			w.Header().Set("Content-Type", detectImageContentType(data))
			_, _ = w.Write(data)
			return
		}
	}

	if meta.seed == "" {
		http.NotFound(w, r)
		return
	}
	size := quantizeCoverSize(paramInt(r, "size", 512))
	cachePath, err := h.generatedCoverPath(meta.seed, size)
	if err != nil {
		http.Error(w, "cover generation failed", http.StatusInternalServerError)
		return
	}
	http.ServeFile(w, r, cachePath)
}

// quantizeCoverSize rounds the requested size up to the nearest supported
// generated-cover bucket. Values outside the range are clamped.
func quantizeCoverSize(requested int) int {
	buckets := []int{128, 256, 512, 1024}
	if requested <= 0 {
		return 512
	}
	for _, b := range buckets {
		if requested <= b {
			return b
		}
	}
	return buckets[len(buckets)-1]
}

// generatedCoverPath returns the filesystem path of a generated cover for
// seed at size. The file is created on demand; concurrent callers may both
// generate and rename — the final os.Rename is atomic so the served file is
// always a complete PNG.
func (h *Handler) generatedCoverPath(seed string, size int) (string, error) {
	hash := sha256.Sum256([]byte(seed))
	name := fmt.Sprintf("%s_%d.png", hex.EncodeToString(hash[:6]), size)
	path := filepath.Join(h.coverCacheDir, name)

	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	if err := os.MkdirAll(h.coverCacheDir, 0750); err != nil {
		return "", fmt.Errorf("create cover cache dir: %w", err)
	}
	data, err := covergen.Generate(seed, size)
	if err != nil {
		return "", fmt.Errorf("generate cover (size=%d): %w", size, err)
	}
	tmp, err := os.CreateTemp(h.coverCacheDir, "cover-*.png.tmp")
	if err != nil {
		return "", fmt.Errorf("create temp cover file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("write temp cover file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("close temp cover file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("rename temp cover file: %w", err)
	}
	return path, nil
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
