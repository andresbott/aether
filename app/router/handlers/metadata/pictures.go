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
	"github.com/andresbott/aether/internal/scanner"
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
	detail   string // e.g. the folder filename or "4 of 10 files"
	filePath string // serve this file (db / folder slots)
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

// selectionPaths resolves the request's repeated paths params (the selected
// tracks) to absolute paths, falling back to every track in the folder.
func (h *Handler) selectionPaths(ctx context.Context, lib *librarySummary, abs string, paths []string) []string {
	if len(paths) == 0 {
		return folderTrackPaths(ctx, lib, abs, h.Reader)
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if trackAbs, err := metadataedit.ResolveInLibrary(lib.Path, p); err == nil {
			out = append(out, trackAbs)
		}
	}
	return out
}

// selectionDirs returns the distinct directories the selected tracks live in,
// in first-seen order. An album is not necessarily one folder — a multi-disc
// release is usually laid out as CD 1/, CD 2/ subfolders — so folder art is
// resolved over every directory the selection spans, with the first one acting
// as the album's primary (representative) folder. Falls back to the requested
// folder when the selection is empty.
func (h *Handler) selectionDirs(ctx context.Context, lib *librarySummary, abs string, paths []string) []string {
	dirs := distinctDirs(h.selectionPaths(ctx, lib, abs, paths))
	if len(dirs) == 0 {
		return []string{abs}
	}
	return dirs
}

// distinctDirs returns the parent directories of the given track paths, in
// first-seen order without duplicates.
func distinctDirs(trackPaths []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, 2)
	for _, p := range trackPaths {
		dir := filepath.Dir(p)
		if !seen[dir] {
			seen[dir] = true
			out = append(out, dir)
		}
	}
	return out
}

// folderTrackPaths returns the absolute paths of the audio tracks in abs.
func folderTrackPaths(ctx context.Context, lib *librarySummary, abs string, reader tags.Reader) []string {
	rows, _ := metadataedit.ListTracks(ctx, lib.Path, abs, reader)
	out := make([]string, 0, len(rows))
	for _, t := range rows {
		if trackAbs, err := metadataedit.ResolveInLibrary(lib.Path, t.Path); err == nil {
			out = append(out, trackAbs)
		}
	}
	return out
}

// folderPictureName returns the folder-art filename for a picture type: front
// covers keep the scanner's loose matching (folder.jpg, front.png, ... all
// count), every other type matches its exact <base>.jpg/png convention.
func folderPictureName(dir string, pt metadataedit.PictureType) string {
	if pt.ID == frontCoverType {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return ""
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if !e.IsDir() {
				names = append(names, e.Name())
			}
		}
		return scanner.BestCover(names)
	}
	return metadataedit.FolderPictureName(dir, pt.FileBase)
}

// folderPictureAcross reports the album's folder art for a type across every
// directory the selection spans. detail/path come from the first directory that
// holds the picture (the album's representative image); mixed is true when the
// directories do not all hold the same bytes — i.e. one disc folder is missing
// the art or carries a different image — so the editor can flag it.
func folderPictureAcross(dirs []string, pt metadataedit.PictureType) (name, path string, mixed, ok bool) {
	var firstSum [sha256.Size]byte
	for _, dir := range dirs {
		n := folderPictureName(dir, pt)
		if n == "" {
			mixed = true // this folder lacks the album's art
			continue
		}
		p := filepath.Join(dir, n)
		sum, serr := fileSum(p)
		if !ok {
			name, path, firstSum, ok = n, p, sum, true
			continue
		}
		if serr != nil || sum != firstSum {
			mixed = true
		}
	}
	// A picture nowhere at all is simply absent, not mixed.
	if !ok {
		return "", "", false, false
	}
	return name, path, mixed, true
}

// fileSum returns the SHA-256 of a file's contents, used to tell whether the
// disc folders of one album hold the same image.
func fileSum(path string) ([sha256.Size]byte, error) {
	f, err := os.Open(path) //nolint:gosec // G304: path is built from a validated album directory plus a filename read from that directory
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer func() { _ = f.Close() }()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return [sha256.Size]byte{}, err
	}
	var sum [sha256.Size]byte
	copy(sum[:], hasher.Sum(nil))
	return sum, nil
}

// pictureForSlot resolves one type+slot cell, or ok=false when empty. paths
// are the selected tracks; they also determine the directories a folder
// picture is looked up in (empty = the requested folder).
func (h *Handler) pictureForSlot(ctx context.Context, lib *librarySummary, abs string, pt metadataedit.PictureType, slot string, paths []string) (resolvedPicture, bool) {
	switch slot {
	case "folder":
		if name, path, _, found := folderPictureAcross(h.selectionDirs(ctx, lib, abs, paths), pt); found {
			return resolvedPicture{detail: name, filePath: path}, true
		}
	case "embedded":
		// Probed in selection order. There is deliberately no "prefer the
		// scanned album's cover track" step: that was a DB lookup, and the
		// editor no longer reads the library index.
		for _, trackAbs := range h.selectionPaths(ctx, lib, abs, paths) {
			if data, ok, err := metadataedit.ReadEmbeddedPicture(trackAbs, pt.ID); err == nil && ok {
				return resolvedPicture{data: data}, true
			}
		}
	}
	return resolvedPicture{}, false
}

type pictureSlotDTO struct {
	Slot   string `json:"slot"` // "embedded" | "folder" | "db"
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
	tracks := h.selectionPaths(r.Context(), lib, abs, r.URL.Query()["paths"])

	// One taglib properties read per track, counting pictures per type.
	embeddedCount := map[string]int{}
	for _, trackAbs := range tracks {
		images, lerr := metadataedit.ListEmbeddedPictures(trackAbs)
		if lerr != nil {
			continue
		}
		seen := map[string]bool{}
		for _, img := range images {
			if !seen[img.Type] {
				seen[img.Type] = true
				embeddedCount[img.Type]++
			}
		}
	}

	// Folder art is looked up in every directory the selection spans, so a
	// multi-disc album reports the art of its disc folders as one cell.
	dirs := h.selectionDirs(r.Context(), lib, abs, r.URL.Query()["paths"])

	out := make([]pictureDTO, 0, len(metadataedit.PictureTypes))
	for _, pt := range metadataedit.PictureTypes {
		slots := make([]pictureSlotDTO, 0, len(pictureSlots))
		if n := embeddedCount[pt.ID]; n > 0 {
			slots = append(slots, pictureSlotDTO{
				Slot:   "embedded",
				Detail: fmt.Sprintf("%d of %d files", n, len(tracks)),
			})
		}
		if name, _, mixed, found := folderPictureAcross(dirs, pt); found {
			slots = append(slots, pictureSlotDTO{Slot: "folder", Detail: name, Mixed: mixed})
		}
		if len(slots) > 0 {
			out = append(out, pictureDTO{Type: pt.ID, Slots: slots})
		}
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

	rp, ok := h.pictureForSlot(r.Context(), lib, abs, pt, slot, r.URL.Query()["paths"])
	if !ok {
		http.NotFound(w, r)
		return
	}

	// A size means "this is a grid thumbnail" — serve an optimized derivative.
	// Without one the original is served verbatim, because the editor also
	// fetches this URL to copy an image into another slot, and a copy must carry
	// the full-fidelity bytes rather than a downscaled re-encode.
	if size := requestedPictureSize(r); size > 0 && h.Images != nil {
		if h.servePictureThumb(w, r, pt, slot, rp, size) {
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
	w http.ResponseWriter, r *http.Request, pt metadataedit.PictureType, slot string, rp resolvedPicture, size int,
) bool {
	// Keyed by the picture's identity — type and slot — under a kind of its own,
	// so editor thumbnails never collide with the covers served by /rest.
	name := pt.FileBase + "_" + slot
	fingerprint, load := pictureCacheSource(rp)
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

// pictureCacheSource returns the fingerprint and lazy loader for a resolved
// picture, or a nil loader when it holds neither a readable file nor bytes.
func pictureCacheSource(rp resolvedPicture) (string, func() ([]byte, error)) {
	if rp.filePath != "" {
		info, err := os.Stat(rp.filePath)
		if err != nil {
			return "", nil
		}
		path := rp.filePath
		return fmt.Sprintf("file|%s|%d|%d", path, info.Size(), info.ModTime().UnixNano()),
			func() ([]byte, error) { return os.ReadFile(path) } //nolint:gosec // G304: path comes from the picture resolver (asset store or album directory), never from the request
	}
	if len(rp.data) > 0 {
		data := rp.data
		sum := sha256.Sum256(data)
		return "bytes|" + hex.EncodeToString(sum[:12]), func() ([]byte, error) { return data, nil }
	}
	return "", nil
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

// applyPicture saves an image of one picture type to one slot: aether's
// managed store ("db"), an art file in the album folder ("folder"), or
// embedded in the tags of the given tracks ("embedded"). The image source is
// either an uploaded file part ("image") or a Cover Art Archive URL
// ("image_url").
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

	resolved := make([]string, 0, len(paths))
	for _, p := range paths {
		abs, rerr := metadataedit.ResolveInLibrary(libModel.Path, p)
		if rerr != nil {
			writeErr(w, http.StatusBadRequest, "validation_error", rerr.Error())
			return
		}
		resolved = append(resolved, abs)
	}

	if status, serr := h.savePictureToSlot(target, pt, resolved, ext, data); serr != nil {
		writeErr(w, status, codeFor(status), serr.Error())
		return
	}

	// Re-index the folder's tracks: the embedded slot changed their tags, and
	// folder/db writes change which image the album should serve (reconcile
	// redetects album.CoverPath).
	rs := h.rescanSaved(r.Context(), libModel.ID, resolved)
	writeJSON(w, http.StatusOK, applyPictureResult{OK: true, Target: target, Type: pt.ID, Rescan: rs})
}

// savePictureToSlot writes the image bytes to the requested slot, returning
// an HTTP status + error on failure (0, nil on success). DB-side effects
// (album cover path, embedded-cover flag) only apply to the front cover —
// they feed the app-wide cover serving, which only knows one cover per album.
// savePictureToSlot writes the image bytes to the requested slot, returning
// an HTTP status + error on failure (0, nil on success). Both slots are on
// disk; the DB catches up through the caller's rescan, which re-reads the tags
// and re-detects the album's cover file.
func (h *Handler) savePictureToSlot(target string, pt metadataedit.PictureType, resolved []string, ext string, data []byte) (int, error) {
	switch target {
	case "folder":
		// An album can span several directories (a multi-disc release laid out
		// as CD 1/, CD 2/): write the same art file into each of them, so every
		// disc folder carries the album's picture.
		for _, dir := range distinctDirs(resolved) {
			if _, err := metadataedit.WriteFolderPicture(dir, pt.FileBase, ext, data); err != nil {
				return http.StatusInternalServerError, err
			}
		}
	case "embedded":
		for _, abs := range resolved {
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
	rescanPaths := h.selectionPaths(r.Context(), lib, abs, paths)
	switch r.URL.Query().Get("slot") {
	case "folder":
		// Mirrors the save fan-out: the art was written into every directory the
		// album spans, so remove it from each of them.
		for _, dir := range h.selectionDirs(r.Context(), lib, abs, paths) {
			name := folderPictureName(dir, pt)
			if name == "" {
				continue
			}
			if rerr := os.Remove(filepath.Join(dir, name)); rerr != nil { //nolint:gosec // G703: dir derives from paths validated by ResolveInLibrary (rejects traversal); name is a bare filename from the folder listing
				writeErr(w, http.StatusInternalServerError, "internal", rerr.Error())
				return
			}
		}
	case "embedded":
		for _, trackAbs := range rescanPaths {
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
	if rs := h.rescanSaved(r.Context(), lib.ID, rescanPaths); rs != nil {
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
