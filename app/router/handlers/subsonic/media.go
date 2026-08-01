package subsonic

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/covergen"
	"github.com/andresbott/aether/internal/imagecache"
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
	// cacheKind/cacheKey file this entity's cached derivatives (imagecache
	// mirrors the assetstore layout: <kind>/<key>/). They identify the entity,
	// not the winning source — which source wins changes as uploads land and
	// are removed, and the per-derivative fingerprint covers that.
	cacheKind string
	cacheKey  string
	// styleFor resolves the configured covergen style for the entity when
	// the generated-cover fallback is reached; nil means "auto". Deferred so
	// requests served from a real cover skip the library lookup.
	styleFor func() (string, error)
}

// artistCoverMeta resolves an artist's cover. A cover keyed by MusicBrainz ID
// (auto-fetched or a manual upload made while the artist was matched) takes
// precedence; then the DB-ID slot used for manual uploads on unmatched artists;
// then an image found next to the artist's albums on disk (`ImagePath`, set by
// the scanner for `<collection>/<artist>/<album>` layouts). Nothing found means
// the name-seeded generated avatar.
func (h *Handler) artistCoverMeta(artist *model.Artist) coverMeta {
	id := artist.ID
	meta := coverMeta{
		seed:      artist.NameNorm,
		cacheKind: assetstore.KindArtist,
		cacheKey:  strconv.FormatUint(uint64(artist.ID), 10),
		styleFor: func() (string, error) {
			return h.store.CoverStyleForArtist(id)
		},
	}
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

// albumCoverMeta resolves an album's cover. A cover saved to aether's managed
// store (the metadata editor's "save to DB" target) takes precedence over the
// folder file, which in turn beats embedded art.
func (h *Handler) albumCoverMeta(album *model.Album) coverMeta {
	key := strconv.FormatUint(uint64(album.ID), 10)
	meta := coverMeta{
		coverPath: album.CoverPath,
		albumID:   album.ID,
		seed:      album.AlbumArtistNorm + "|" + album.NameNorm,
		cacheKind: assetstore.KindAlbum,
		cacheKey:  key,
		styleFor:  h.albumStyleFor(album.ID),
	}
	if p, ok := h.assets.Get(assetstore.KindAlbum, key); ok {
		meta.coverPath = p
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
		return h.albumCoverMeta(album), true
	case "track":
		song, err := h.store.GetSong(id)
		if err != nil {
			writeError(w, 70, "song not found")
			return coverMeta{}, false
		}
		// A track's cover is its album's, cached under the album's identity so
		// the two ids share one set of derivatives.
		if song.Album != nil {
			return h.albumCoverMeta(song.Album), true
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
		meta := coverMeta{
			seed:      station.Name,
			cacheKind: assetstore.KindRadio,
			cacheKey:  strconv.FormatUint(uint64(station.ID), 10),
		}
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
		meta := coverMeta{
			seed:      pl.Name,
			cacheKind: assetstore.KindPlaylist,
			cacheKey:  strconv.FormatUint(uint64(pl.ID), 10),
		}
		if p, ok := h.assets.Get(assetstore.KindPlaylist, meta.cacheKey); ok {
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
		meta := coverMeta{
			seed:      genre.Name,
			cacheKind: assetstore.KindGenre,
			cacheKey:  strconv.FormatUint(uint64(genre.ID), 10),
		}
		if p, ok := h.assets.Get(assetstore.KindGenre, meta.cacheKey); ok {
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
	// One URL serves WebP or JPEG depending on what the client accepts, so any
	// shared cache must key on Accept as well.
	w.Header().Set("Vary", "Accept")

	sources := h.coverSources(meta)
	if len(sources) == 0 {
		http.NotFound(w, r)
		return
	}

	format := imagecache.FormatForAccept(r.Header.Get("Accept"))
	size := quantizeCoverSize(paramInt(r, "size", maxCoverSize))

	// Walk the candidates in precedence order, falling through when one cannot
	// be turned into an image. A library holds truncated cover files and tracks
	// re-tagged since the scan; answering 500 would leave a broken image in
	// every grid cell the entity appears in, so a lower-precedence source — in
	// the worst case the generated cover — takes over.
	for _, src := range sources {
		path, err := h.images.Path(imagecache.Request{
			Kind:        src.kind,
			Key:         src.key,
			Name:        src.name,
			Size:        size,
			Format:      format,
			Fingerprint: src.fingerprint,
			Load:        src.load,
		})
		if err != nil {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		w.Header().Set("Content-Type", format.ContentType())
		serveCoverFile(w, r, path, info)
		return
	}
	http.Error(w, "cover art could not be prepared", http.StatusInternalServerError)
}

// coverSource is the winning source image for an entity's cover, resolved to
// everything the image cache needs to build and key a derivative from it.
type coverSource struct {
	kind string
	key  string
	// name separates derivatives of different sources for the same entity, so
	// the generated fallback never collides with a real cover.
	name string
	// fingerprint identifies these source bytes; a change invalidates the
	// entity's cached derivatives.
	fingerprint string
	load        func() ([]byte, error)
}

// coverSources lists the entity's candidate sources in precedence order — the
// real cover file (managed store, then the folder/artist image), then the album's
// embedded front cover, then the name-seeded generated cover. Every candidate is
// lazy: nothing is read or rendered unless the cache misses and that candidate is
// actually reached.
func (h *Handler) coverSources(meta coverMeta) []coverSource {
	if meta.cacheKind == "" || meta.cacheKey == "" {
		return nil
	}
	var out []coverSource

	if meta.coverPath != "" {
		if info, err := os.Stat(meta.coverPath); err == nil {
			path := meta.coverPath
			out = append(out, coverSource{
				kind: meta.cacheKind,
				key:  meta.cacheKey,
				name: "cover",
				// Fingerprint on *which* file is served plus its size and
				// mtime, not on age alone: falling back to an older file
				// (removing an upload uncovers the folder image) must still
				// invalidate the cached derivative.
				fingerprint: fmt.Sprintf("file|%s|%d|%d", path, info.Size(), info.ModTime().UnixNano()),
				load:        func() ([]byte, error) { return os.ReadFile(path) }, //nolint:gosec // G304: path comes from the cover resolver (asset store or scanner-detected image), never from the request
			})
		}
	}

	if src, ok := h.embeddedCoverSource(meta); ok {
		out = append(out, src)
	}

	if meta.seed != "" {
		seed := meta.seed
		style := resolveCoverStyle(meta.styleFor)
		out = append(out, coverSource{
			kind: meta.cacheKind,
			key:  meta.cacheKey,
			name: "generated",
			// The style is configurable per library, so it belongs in the key: a
			// style change must re-render rather than serve the old look.
			fingerprint: "generated|" + seed + "|" + style,
			load: func() ([]byte, error) {
				// covergen renders square, so the requested size fully determines
				// the output; it is rendered once here and re-encoded by the cache.
				return generateCover(seed, style)
			},
		})
	}
	return out
}

// embeddedCoverSource resolves the album's embedded front cover as a cache
// source, or ok=false when the album has no flagged cover track on disk.
func (h *Handler) embeddedCoverSource(meta coverMeta) (coverSource, bool) {
	if meta.albumID == 0 {
		return coverSource{}, false
	}
	trackPath, err := h.store.GetCoverTrackPath(meta.albumID)
	if err != nil || trackPath == "" {
		return coverSource{}, false
	}
	info, err := os.Stat(trackPath)
	if err != nil {
		return coverSource{}, false
	}
	albumID := meta.albumID
	return coverSource{
		kind: meta.cacheKind,
		key:  meta.cacheKey,
		name: "embedded",
		// Keyed on the audio file so re-tagging it rebuilds the derivative; the
		// point of caching here is that a hit costs no tag read of a whole music
		// file at all.
		fingerprint: fmt.Sprintf("embedded|%s|%d|%d", trackPath, info.Size(), info.ModTime().UnixNano()),
		load: func() ([]byte, error) {
			data := h.readEmbeddedCover(albumID)
			if data == nil {
				return nil, errNoEmbeddedCover
			}
			return data, nil
		},
	}, true
}

// errNoEmbeddedCover reports a track flagged as carrying embedded art whose
// front cover cannot be read after all (re-tagged since the scan).
var errNoEmbeddedCover = errors.New("track has no embedded front cover")

// generateCover renders a generated cover at the largest size the server will
// serve. The image cache scales it down per request, so one render covers every
// size instead of one PNG per (seed, size) as the old generated-covers tree did.
func generateCover(seed, style string) ([]byte, error) {
	if st, ok := covergen.ParseStyle(style); ok {
		return covergen.GenerateStyle(seed, maxCoverSize, st)
	}
	return covergen.Generate(seed, maxCoverSize)
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

// maxCoverSize is the largest edge the server will serve, and what a request
// with no size gets. Originals are never served as-is: a 3000px scan behind an
// unsized request is exactly the traffic this cache exists to avoid.
const maxCoverSize = 1024

// coverSizeBuckets are the sizes derivatives are actually built at. Quantizing
// bounds the cache to a handful of files per entity instead of one per distinct
// size any client ever asks for. The small buckets match what the web UI
// requests (48/80/96/160/200/250 across rows, cards and the player).
var coverSizeBuckets = []int{48, 96, 160, 256, 512, maxCoverSize}

// quantizeCoverSize rounds the requested size up to the nearest bucket, so a
// client asking for 200 gets the 256 derivative scaled down by the browser
// rather than its own cache entry. Oversized and non-positive values clamp to
// the cap.
func quantizeCoverSize(requested int) int {
	if requested <= 0 {
		return maxCoverSize
	}
	for _, b := range coverSizeBuckets {
		if requested <= b {
			return b
		}
	}
	return maxCoverSize
}

// albumStyleFor returns a deferred cover-style resolver for an album.
func (h *Handler) albumStyleFor(albumID uint) func() (string, error) {
	return func() (string, error) { return h.store.CoverStyleForAlbum(albumID) }
}

// resolveCoverStyle runs the deferred resolver and maps its result to a
// covergen style name, degrading to "auto" when the resolver is absent,
// fails, or names an unknown style.
func resolveCoverStyle(styleFor func() (string, error)) string {
	if styleFor == nil {
		return "auto"
	}
	name, err := styleFor()
	if err != nil || name == "" || name == "auto" {
		return "auto"
	}
	if _, ok := covergen.ParseStyle(name); !ok {
		return "auto"
	}
	return name
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
