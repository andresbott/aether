package subsonic

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/covergen"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/tags"
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

// artistCoverMeta resolves an artist's cover. A cover keyed by MusicBrainz ID
// (auto-fetched or a manual upload made while the artist was matched) takes
// precedence; then the DB-ID slot used for manual uploads on unmatched artists;
// then an image found next to the artist's albums on disk (`ImagePath`, set by
// the scanner for `<collection>/<artist>/<album>` layouts). Nothing found means
// the name-seeded generated avatar.
func (h *Handler) artistCoverMeta(artist *model.Artist) coverMeta {
	meta := coverMeta{seed: artist.NameNorm}
	if artist.MBArtistID != "" {
		if p, ok := h.assets.Get(assetstore.KindArtist, artist.MBArtistID); ok {
			meta.coverPath = p
		}
	}
	if meta.coverPath == "" {
		if p, ok := h.assets.Get(assetstore.KindArtist, strconv.FormatUint(uint64(artist.ID), 10)); ok {
			meta.coverPath = p
		}
	}
	if meta.coverPath == "" {
		meta.coverPath = artist.ImagePath
	}
	return meta
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
		meta := coverMeta{coverPath: album.CoverPath, albumID: album.ID, seed: album.AlbumArtistNorm + "|" + album.NameNorm}
		// A cover saved to aether's managed store (metadata editor "save to DB"
		// target) takes precedence over the folder file and embedded art.
		if p, ok := h.assets.Get(assetstore.KindAlbum, strconv.FormatUint(uint64(album.ID), 10)); ok {
			meta.coverPath = p
		}
		return meta, true
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
		return h.artistCoverMeta(artist), true
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
	case "playlist":
		pl, err := h.store.GetPlaylist(id)
		if err != nil {
			writeError(w, 70, "playlist not found")
			return coverMeta{}, false
		}
		// A manually uploaded cover (see updatePlaylist) takes precedence;
		// otherwise fall through to the name-seeded generated cover (same
		// mechanism as artists/radio).
		meta := coverMeta{seed: pl.Name}
		if p, ok := h.assets.Get(assetstore.KindPlaylist, strconv.FormatUint(uint64(pl.ID), 10)); ok {
			meta.coverPath = p
		}
		return meta, true
	case "genre":
		genre, err := h.store.GetGenre(id)
		if err != nil {
			writeError(w, 70, "genre not found")
			return coverMeta{}, false
		}
		// A manually uploaded cover (see updateGenre) takes precedence; otherwise
		// fall through to the name-seeded generated cover (same mechanism as
		// artists/radio/playlists). Keyed by DB ID — genre names may contain
		// characters the assetstore key regexp rejects.
		meta := coverMeta{seed: genre.Name}
		if p, ok := h.assets.Get(assetstore.KindGenre, strconv.FormatUint(uint64(genre.ID), 10)); ok {
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
		if info, err := os.Stat(meta.coverPath); err == nil {
			// Validate on *which* file is served, not on how old it is.
			// http.ServeFile alone sends Last-Modified from the mtime and honors
			// If-Modified-Since, so falling back to an older file (deleting an
			// upload uncovers the music-folder image) would answer 304 and leave
			// the client showing the image that was just removed. An ETag over
			// path+size+mtime changes whenever the served file does, and dropping
			// Last-Modified keeps the date-based check out of the picture.
			serveCoverFile(w, r, meta.coverPath, info)
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
	info, err := os.Stat(cachePath)
	if err != nil {
		http.Error(w, "cover generation failed", http.StatusInternalServerError)
		return
	}
	serveCoverFile(w, r, cachePath, info)
}

// serveCoverFile serves path with an ETag identifying that exact file, and no
// Last-Modified. Cover URLs are stable while the file behind them is not — and
// the replacement is not always newer (removing an uploaded image falls back to
// an older folder image or to a long-cached generated avatar), so a date-based
// validator can wrongly answer 304 and pin a stale image in the browser until a
// hard refresh.
func serveCoverFile(w http.ResponseWriter, r *http.Request, path string, info os.FileInfo) {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d", path, info.Size(), info.ModTime().UnixNano())))
	w.Header().Set("ETag", `"`+hex.EncodeToString(sum[:16])+`"`)

	f, err := os.Open(path) //nolint:gosec // G304: path comes from the cover resolver (asset store, scanner-detected image, or the generated-cover cache), never from the request
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()

	// A zero modtime tells ServeContent to omit Last-Modified, leaving the ETag
	// as the only validator. ServeContent still handles If-None-Match and Range.
	http.ServeContent(w, r, filepath.Base(path), time.Time{}, f)
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

// readEmbeddedCover returns the album's embedded front cover, or nil when the
// flagged track has none. Only the picture typed "Front Cover" is a cover: a
// file may also embed a back cover, disc scan or booklet page, and those must
// never be served as the album art.
func (h *Handler) readEmbeddedCover(albumID uint) []byte {
	trackPath, err := h.store.GetCoverTrackPath(albumID)
	if err != nil || trackPath == "" {
		return nil
	}
	data, ok, err := tags.ReadFrontCover(trackPath)
	if err != nil || !ok {
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
