package metadata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/andresbott/aether/internal/imagecache"
	"github.com/andresbott/aether/internal/metadataedit"
	"github.com/andresbott/aether/internal/tags"
	"gorm.io/gorm"
)

const (
	pictureMultipartMemory = 1 << 20  // 1 MiB kept in memory; larger parts spill to temp files
	maxPictureRequestBytes = 12 << 20 // 12 MiB total request cap
	frontCoverType         = tags.FrontCoverType
)

var errPictureSource = errors.New("an image file or image_url is required")

// pictureSlots lists the storage slots a picture may live in, in the editor's
// display/preference order (embedded first). The editor deals exclusively with
// metadata on disk: a picture is either embedded in the song's tags or an art
// file in the album folder. Aether's managed store is NOT an editor concern —
// manual album covers are set through the /rest updateAlbum extension.
var pictureSlots = []string{"embedded", "folder"}

// resolvedPicture is one type+slot cell's current image. Exactly one of
// filePath / data is set when found.
type resolvedPicture struct {
	filePath string // serve this file (folder slot)
	data     []byte // serve these bytes (embedded slot)
}

// requestedType validates the request's picture type against the registry,
// defaulting to Front Cover when absent.
func requestedType(r *http.Request) (metadataedit.PictureType, error) {
	id := r.FormValue("type")
	if id == "" {
		id = frontCoverType
	}
	pt, ok := metadataedit.PictureTypeByID(id)
	if !ok {
		return metadataedit.PictureType{}, fmt.Errorf("unknown picture type %q", id)
	}
	return pt, nil
}

// effectiveSelection returns the library-relative paths ResolveAlbum should
// resolve: the request's explicit paths[] when given, or — matching today's
// "no paths means the whole folder" default — every track in the requested
// folder, or — when that folder holds none either — the folder itself, so a
// folder-only album (no tracks: an empty or not-yet-populated folder) still
// resolves for folder-art lookups instead of ResolveAlbum rejecting an empty
// selection.
func (h *Handler) effectiveSelection(ctx context.Context, lib *librarySummary, abs string, paths []string) []string {
	if len(paths) > 0 {
		return paths
	}
	rows, _ := metadataedit.ListTracks(ctx, lib.Path, abs, h.Reader)
	if len(rows) > 0 {
		out := make([]string, len(rows))
		for i, t := range rows {
			out[i] = t.Path
		}
		return out
	}
	return []string{folderSeed(lib, abs)}
}

// folderSeed returns abs's path relative to the library root — the fallback
// selection entry for "nothing else resolved", anchoring a directory-only
// Album on the browsed folder itself (Dirs()=[abs], Tracks()=nil).
func folderSeed(lib *librarySummary, abs string) string {
	rel, err := filepath.Rel(lib.Path, abs)
	if err != nil {
		return "."
	}
	return rel
}

// resolveAlbum builds the Album for a request: paths[] (or, absent that,
// every admissible track in the browsed folder — see effectiveSelection)
// resolved leniently — an entry that fails to resolve (an absolute path, or
// one escaping the library root) is skipped, not fatal — falling back to the
// browsed folder itself when nothing in the selection resolves at all,
// whether because every explicit path was malformed or the folder holds no
// tracks. This exactly mirrors what selectionPaths/selectionDirs did before
// they became this Album: a bad or empty selection degrades gracefully
// (200, or 404 for a truly-missing image) rather than failing the request.
func (h *Handler) resolveAlbum(ctx context.Context, lib *librarySummary, abs string, paths []string) (metadataedit.Album, error) {
	al, err := metadataedit.ResolveAlbum(lib.Path, h.effectiveSelection(ctx, lib, abs, paths))
	if err != nil {
		return al, err
	}
	if len(al.Dirs()) == 0 {
		// Every entry in the selection failed to resolve (effectiveSelection
		// itself only ever returns paths, enumerated tracks, or the folder
		// seed — all of which resolve on their own, so this is specifically
		// the "explicit paths[] given but all of them bad" case).
		return metadataedit.ResolveAlbum(lib.Path, []string{folderSeed(lib, abs)})
	}
	return al, nil
}

// findCell returns the (typeID, slot) cell from a picture matrix, or nil when
// that type+slot is absent.
func findCell(matrix []metadataedit.TypeSlots, typeID, slot string) *metadataedit.SlotState {
	for _, ts := range matrix {
		if ts.Type.ID != typeID {
			continue
		}
		for i := range ts.Slots {
			if ts.Slots[i].Slot == slot {
				return &ts.Slots[i]
			}
		}
	}
	return nil
}

type pictureSlotDTO struct {
	Slot   string `json:"slot"` // "embedded" | "folder"
	Detail string `json:"detail,omitempty"`
	// Mixed marks a folder slot whose art is not the same in every directory
	// the album spans (a multi-disc release): one disc folder is missing it or
	// holds a different image. The editor shows the first one and warns that
	// saving overwrites all of them.
	Mixed bool `json:"mixed,omitempty"`
}

type pictureDTO struct {
	Type  string           `json:"type"`
	Slots []pictureSlotDTO `json:"slots"`
}

// pictures reports, for every registry type present somewhere, which slots
// hold it. Embedded presence is counted over the selected paths (or the whole
// folder when none are given).
func (h *Handler) pictures(w http.ResponseWriter, r *http.Request) {
	lib, abs, status, err := h.resolveLibraryRel(r)
	if err != nil {
		writeErr(w, status, codeFor(status), err.Error())
		return
	}
	al, aerr := h.resolveAlbum(r.Context(), lib, abs, r.URL.Query()["paths"])
	if aerr != nil {
		writeErr(w, http.StatusInternalServerError, "internal", aerr.Error())
		return
	}

	matrix := al.Matrix(r.Context(), h.Reader)
	out := make([]pictureDTO, 0, len(matrix))
	for _, ts := range matrix {
		slots := make([]pictureSlotDTO, 0, len(ts.Slots))
		for _, sl := range ts.Slots {
			dto := pictureSlotDTO{Slot: sl.Slot, Mixed: sl.Mixed}
			if sl.Slot == "embedded" {
				dto.Detail = fmt.Sprintf("%d of %d files", sl.PresentCount, sl.TotalCount)
			} else {
				dto.Detail = sl.Detail
			}
			slots = append(slots, dto)
		}
		out = append(out, pictureDTO{Type: ts.Type.ID, Slots: slots})
	}
	writeJSON(w, http.StatusOK, map[string]any{"pictures": out})
}

// pictureImage serves the image of one type+slot cell. 404 when the cell is
// empty.
func (h *Handler) pictureImage(w http.ResponseWriter, r *http.Request) {
	lib, abs, status, err := h.resolveLibraryRel(r)
	if err != nil {
		writeErr(w, status, codeFor(status), err.Error())
		return
	}
	pt, terr := requestedType(r)
	if terr != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", terr.Error())
		return
	}
	slot := r.URL.Query().Get("slot")
	if !validSlot(slot) {
		writeErr(w, http.StatusBadRequest, "validation_error", "slot must be one of embedded, folder")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")

	al, aerr := h.resolveAlbum(r.Context(), lib, abs, r.URL.Query()["paths"])
	if aerr != nil {
		writeErr(w, http.StatusInternalServerError, "internal", aerr.Error())
		return
	}
	cell := findCell(al.Matrix(r.Context(), h.Reader), pt.ID, slot)
	if cell == nil {
		http.NotFound(w, r)
		return
	}
	data, filePath, fingerprint, operr := al.Open(cell.Source)
	if operr != nil {
		http.NotFound(w, r)
		return
	}
	rp := resolvedPicture{filePath: filePath, data: data}

	// A size means "this is a grid thumbnail" — serve an optimized derivative.
	// Without one the original is served verbatim, because the editor also
	// fetches this URL to copy an image into another slot, and a copy must carry
	// the full-fidelity bytes rather than a downscaled re-encode.
	if size := requestedPictureSize(r); size > 0 && h.Images != nil {
		if h.servePictureThumb(w, r, pt, slot, rp, fingerprint, size) {
			return
		}
	}

	if rp.filePath != "" {
		http.ServeFile(w, r, rp.filePath)
		return
	}
	writeImage(w, rp.data)
}

// maxPictureThumbSize caps the thumbnail size a client can ask for; larger
// requests are served at the cap rather than rendered on demand.
const maxPictureThumbSize = 1024

// requestedPictureSize parses the optional size parameter, clamped to the cap.
// Zero means "no thumbnail requested".
func requestedPictureSize(r *http.Request) int {
	raw := r.URL.Query().Get("size")
	if raw == "" {
		return 0
	}
	size, err := strconv.Atoi(raw)
	if err != nil || size <= 0 {
		return 0
	}
	return min(size, maxPictureThumbSize)
}

// servePictureThumb serves a cached, display-sized copy of the resolved picture.
// It reports false when no derivative could be produced (an undecodable or
// unreadable source), leaving the caller to serve the original.
func (h *Handler) servePictureThumb(
	w http.ResponseWriter, r *http.Request, pt metadataedit.PictureType, slot string, rp resolvedPicture, fingerprint string, size int,
) bool {
	// Keyed by the picture's identity — type and slot — under a kind of its own,
	// so editor thumbnails never collide with the covers served by /rest.
	name := pt.FileBase + "_" + slot
	load := loadFuncFor(rp)
	if load == nil {
		return false
	}
	format := imagecache.FormatForAccept(r.Header.Get("Accept"))
	path, err := h.Images.Path(imagecache.Request{
		Kind:        pictureThumbKind,
		Key:         pictureThumbKey(rp),
		Name:        name,
		Size:        size,
		Format:      format,
		Fingerprint: fingerprint,
		Load:        load,
	})
	if err != nil {
		return false
	}
	f, err := os.Open(path) //nolint:gosec // G304: path is built by the image cache from a validated kind/key, never from the request
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	w.Header().Set("Content-Type", format.ContentType())
	w.Header().Set("Vary", "Accept")
	// A zero modtime omits Last-Modified: Cache-Control is already no-cache and
	// the editor busts the URL explicitly after every change.
	http.ServeContent(w, r, filepath.Base(path), time.Time{}, f)
	return true
}

// pictureThumbKind files editor thumbnails apart from the entity covers the
// music API caches.
const pictureThumbKind = "editor"

// pictureThumbKey identifies the source image the thumbnail is built from. A
// file gets a hash of its path; embedded bytes get a hash of the bytes, since
// they have no path of their own.
func pictureThumbKey(rp resolvedPicture) string {
	if rp.filePath != "" {
		sum := sha256.Sum256([]byte(rp.filePath))
		return hex.EncodeToString(sum[:8])
	}
	sum := sha256.Sum256(rp.data)
	return hex.EncodeToString(sum[:8])
}

// loadFuncFor returns rp's lazy byte loader for the image cache, or nil when
// it holds neither a readable file nor bytes.
func loadFuncFor(rp resolvedPicture) func() ([]byte, error) {
	if rp.filePath != "" {
		path := rp.filePath
		return func() ([]byte, error) { return os.ReadFile(path) } //nolint:gosec // G304: path comes from the picture resolver (album directory), never from the request
	}
	if len(rp.data) > 0 {
		data := rp.data
		return func() ([]byte, error) { return data, nil }
	}
	return nil
}

func validSlot(slot string) bool {
	for _, s := range pictureSlots {
		if s == slot {
			return true
		}
	}
	return false
}

type applyPictureResult struct {
	OK     bool          `json:"ok"`
	Target string        `json:"target"`
	Type   string        `json:"type"`
	Rescan *rescanStatus `json:"rescan,omitempty"`
}

// applyPicture saves an image of one picture type to one slot: an art file in
// the album folder ("folder") or embedded in the tags of the given tracks
// ("embedded"). The image source is either an uploaded file part ("image") or a
// Cover Art Archive URL ("image_url").
func (h *Handler) applyPicture(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPictureRequestBytes)
	if err := r.ParseMultipartForm(pictureMultipartMemory); err != nil { //nolint:gosec // G120: body is bounded by http.MaxBytesReader on the previous line
		writeErr(w, http.StatusBadRequest, "validation_error", "invalid multipart form: "+err.Error())
		return
	}
	libID, perr := strconv.ParseUint(r.FormValue("library_id"), 10, 64)
	if perr != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "library_id required")
		return
	}
	target := r.FormValue("target")
	if !validSlot(target) {
		writeErr(w, http.StatusBadRequest, "validation_error", "target must be one of embedded, folder")
		return
	}
	pt, terr := requestedType(r)
	if terr != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", terr.Error())
		return
	}
	paths := r.Form["paths"]
	if len(paths) == 0 {
		writeErr(w, http.StatusBadRequest, "validation_error", "at least one path is required")
		return
	}
	libModel, err := h.Store.GetLibrary(uint(libID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	data, ext, status, err := h.pictureImageSource(r)
	if err != nil {
		// status 0 marks an upstream download failure: let the upstream mapping
		// pick the status and the human message.
		if status == 0 {
			writeUpstreamErr(w, err, "The image could not be downloaded. Try again in a moment.")
			return
		}
		writeErr(w, status, codeFor(status), err.Error())
		return
	}

	// applyPicture's own per-path validation predates ResolveAlbum and stays
	// strict, unlike the read/delete endpoints above: a save must not
	// silently write fewer files than the caller asked for. ResolveAlbum
	// itself is lenient — it skips an unresolvable entry rather than failing
	// the whole call, so pictures/pictureImage/deletePicture can fall back to
	// the browsed folder instead of erroring on a stray bad path — so that
	// leniency is not relied on here.
	for _, p := range paths {
		if _, rerr := metadataedit.ResolveInLibrary(libModel.Path, p); rerr != nil {
			writeErr(w, http.StatusBadRequest, "validation_error", rerr.Error())
			return
		}
	}
	al, aerr := metadataedit.ResolveAlbum(libModel.Path, paths)
	if aerr != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", aerr.Error())
		return
	}

	if status, serr := h.savePictureToSlot(target, pt, al, ext, data); serr != nil {
		writeErr(w, status, codeFor(status), serr.Error())
		return
	}

	// Re-index the folder's tracks: the embedded slot changed their tags, and
	// folder writes change which image the album should serve (reconcile
	// redetects album.CoverPath).
	rs := h.rescanSaved(r.Context(), libModel.ID, al.Tracks())
	writeJSON(w, http.StatusOK, applyPictureResult{OK: true, Target: target, Type: pt.ID, Rescan: rs})
}

// savePictureToSlot writes the image bytes to the requested slot, returning
// an HTTP status + error on failure (0, nil on success). Both slots are on
// disk; the DB catches up through the caller's rescan, which re-reads the tags
// and re-detects the album's cover file.
func (h *Handler) savePictureToSlot(target string, pt metadataedit.PictureType, al metadataedit.Album, ext string, data []byte) (int, error) {
	switch target {
	case "folder":
		// An album can span several directories (a multi-disc release laid out
		// as CD 1/, CD 2/): write the same art file into each of them, so every
		// disc folder carries the album's picture.
		for _, dir := range al.Dirs() {
			if _, err := metadataedit.WriteFolderPicture(dir, pt.FileBase, ext, data); err != nil {
				return http.StatusInternalServerError, err
			}
		}
	case "embedded":
		for _, abs := range al.Tracks() {
			if err := metadataedit.WriteEmbeddedPicture(abs, pt.ID, data, ""); err != nil {
				return http.StatusInternalServerError, err
			}
		}
	}
	return 0, nil
}

// deletePicture removes one type+slot cell. Embedded removal targets the
// tracks named by repeated paths params (the selected files), falling back to
// every track in the folder when none are given.
func (h *Handler) deletePicture(w http.ResponseWriter, r *http.Request) {
	lib, abs, status, err := h.resolveLibraryRel(r)
	if err != nil {
		writeErr(w, status, codeFor(status), err.Error())
		return
	}
	pt, terr := requestedType(r)
	if terr != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", terr.Error())
		return
	}
	paths := r.URL.Query()["paths"]
	// Resolved once, before the switch: this is both the selection the delete
	// acts on and the set handed to the post-delete re-index, so the two can
	// never disagree. Re-deriving it inside a case would mean a second
	// directory listing when the client sends no explicit paths.
	al, aerr := h.resolveAlbum(r.Context(), lib, abs, paths)
	if aerr != nil {
		writeErr(w, http.StatusInternalServerError, "internal", aerr.Error())
		return
	}
	switch r.URL.Query().Get("slot") {
	case "folder":
		// Mirrors the save fan-out: the art was written into every directory the
		// album spans, so remove it from each of them.
		if derr := al.DeleteFolderPicture(pt); derr != nil {
			writeErr(w, http.StatusInternalServerError, "internal", derr.Error())
			return
		}
	case "embedded":
		for _, trackAbs := range al.Tracks() {
			if werr := metadataedit.DeleteEmbeddedPicture(trackAbs, pt.ID); werr != nil {
				writeErr(w, http.StatusInternalServerError, "internal", werr.Error())
				return
			}
		}
	default:
		writeErr(w, http.StatusBadRequest, "validation_error", "slot must be one of embedded, folder")
		return
	}
	out := map[string]any{"ok": true}
	if rs := h.rescanSaved(r.Context(), lib.ID, al.Tracks()); rs != nil {
		out["rescan"] = rs
	}
	writeJSON(w, http.StatusOK, out)
}

// writeImage writes raw image bytes with a sniffed image content-type.
func writeImage(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", http.DetectContentType(data))
	_, _ = w.Write(data)
}

type pictureCandidateDTO struct {
	ID       string   `json:"id"`
	ThumbURL string   `json:"thumbUrl"`
	ImageURL string   `json:"imageUrl"`
	IsFront  bool     `json:"isFront"`
	Types    []string `json:"types"`
	Comment  string   `json:"comment"`
}

// pictureCandidates proxies the Cover Art Archive listing for a release (and
// optional release-group) MBID.
func (h *Handler) pictureCandidates(w http.ResponseWriter, r *http.Request) {
	mbid := r.URL.Query().Get("mbid")
	releaseGroup := r.URL.Query().Get("release_group")
	if mbid == "" && releaseGroup == "" {
		writeErr(w, http.StatusBadRequest, "validation_error", "mbid or release_group is required")
		return
	}
	if h.CoverArt == nil {
		writeJSON(w, http.StatusOK, []pictureCandidateDTO{})
		return
	}
	imgs, err := h.CoverArt.List(r.Context(), mbid, releaseGroup)
	if err != nil {
		writeUpstreamErr(w, err, "Cover art could not be loaded right now. Try again in a moment.")
		return
	}
	out := make([]pictureCandidateDTO, 0, len(imgs))
	for _, img := range imgs {
		out = append(out, pictureCandidateDTO{
			ID:       img.ID,
			ThumbURL: img.ThumbURL,
			ImageURL: img.ImageURL,
			IsFront:  img.IsFront,
			Types:    img.Types,
			Comment:  img.Comment,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// pictureImageSource returns the image bytes and normalized extension from
// either an uploaded "image" file part or a downloaded "image_url".
//
// status is the HTTP status to answer with on failure; a zero status alongside
// a non-nil err means the failure came from the external image host, which the
// caller maps through writeUpstreamErr.
func (h *Handler) pictureImageSource(r *http.Request) (data []byte, ext string, status int, err error) {
	if file, header, ferr := r.FormFile("image"); ferr == nil {
		defer func() { _ = file.Close() }()
		data, rerr := io.ReadAll(io.LimitReader(file, maxPictureRequestBytes))
		if rerr != nil {
			return nil, "", http.StatusBadRequest, rerr
		}
		return data, pictureExt(header.Filename, data), 0, nil
	}
	if url := r.FormValue("image_url"); url != "" {
		if h.CoverArt == nil {
			return nil, "", http.StatusBadRequest, errPictureSource
		}
		data, ext, derr := h.CoverArt.DownloadImage(r.Context(), url)
		if derr != nil {
			return nil, "", 0, derr
		}
		return data, ext, 0, nil
	}
	return nil, "", http.StatusBadRequest, errPictureSource
}

// pictureExt derives a jpg/png extension from the upload filename, falling
// back to sniffing the image bytes.
func pictureExt(filename string, data []byte) string {
	switch filepath.Ext(filename) {
	case ".png", ".PNG":
		return "png"
	case ".jpg", ".jpeg", ".JPG", ".JPEG":
		return "jpg"
	}
	if http.DetectContentType(data) == "image/png" {
		return "png"
	}
	return "jpg"
}
