package metadata

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/andresbott/aether/app/router/handlers/httperr"
	"github.com/andresbott/aether/internal/imagecache"
	"github.com/andresbott/aether/internal/imageinfo"
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
	return pictureTypeByIDOrDefault(r.FormValue("type"))
}

// pictureTypeByIDOrDefault validates a picture type ID against the registry,
// defaulting to Front Cover when id is empty. Shared by requestedType
// (the query/form-based endpoints) and removals (a JSON body, which has no
// r.FormValue to read from).
func pictureTypeByIDOrDefault(id string) (metadataedit.PictureType, error) {
	if id == "" {
		id = frontCoverType
	}
	pt, ok := metadataedit.PictureTypeByID(id)
	if !ok {
		return metadataedit.PictureType{}, fmt.Errorf("%w %q", errUnknownType, id)
	}
	return pt, nil
}

type pictureImageDTO struct {
	URL string `json:"url"`
	// ThumbURL requests the same source at inventoryThumbSize — the picture
	// grid's cell size — instead of full fidelity.
	ThumbURL string `json:"thumb_url,omitempty"`
}

type pictureSlotDTO struct {
	Slot   string `json:"slot"` // "embedded" | "folder"
	Detail string `json:"detail,omitempty"`
	// Mixed marks a folder slot whose art is not the same in every directory
	// the album spans (a multi-disc release): one disc folder is missing it or
	// holds a different image. The editor shows the first one and warns that
	// saving overwrites all of them.
	Mixed bool `json:"mixed,omitempty"`
	// PresentCount/TotalCount describe an embedded slot: how many of the
	// selected paths carry this picture type, out of how many were selected.
	// Both are omitted (the zero value) for a folder slot.
	PresentCount int `json:"present_count,omitempty"`
	TotalCount   int `json:"total_count,omitempty"`
	// Image is this cell's ready-to-render URLs. Always populated: inventory
	// only ever lists populated slots, and every populated slot has a
	// representative Source.
	Image pictureImageDTO `json:"image"`
	// Meta is the size/dimensions/format of the slot's representative image
	// (the folder file, or the embedded picture of the first track that has
	// one). nil when it could not be read.
	Meta *imageMetaDTO `json:"meta,omitempty"`
}

type pictureDTO struct {
	Type  string           `json:"type"`
	Slots []pictureSlotDTO `json:"slots"`
}

// pictureImagePath is the picture image endpoint's route, relative to
// wherever Routes is mounted (see the Routes doc comment in metadata.go).
// Both Routes (mounting it) and pictureImageRef (building the inventory's
// URLs) reuse this constant so the registered route and the generated URLs
// can never drift apart.
const pictureImagePath = "/metadata/pictures/image"

// inventoryThumbSize is the thumbnail size the picture grid's cells request
// (mirrors PICTURE_CELL_SIZE in PicturesSection.vue): the cells render at
// roughly 160 CSS pixels; 320 keeps them sharp on 2x displays while staying a
// fraction of a full-resolution scan.
const inventoryThumbSize = 320

// pictureImageRef builds one present slot's ready-to-render image URLs from
// its representative Source. The URLs are mount-relative — no scheme/host,
// and never a hard-coded /api/v1 prefix — so the handler stays agnostic to
// whatever prefix it is mounted under (the planned /admin reorg); the SPA
// prepends apiClient.defaults.baseURL when rendering (see serverPictureUrl in
// PicturesSection.vue).
func pictureImageRef(libID uint, src metadataedit.Source) pictureImageDTO {
	q := src.Values()
	q.Set("library_id", strconv.FormatUint(uint64(libID), 10))
	imgURL := pictureImagePath + "?" + q.Encode()
	q.Set("size", strconv.Itoa(inventoryThumbSize))
	thumbURL := pictureImagePath + "?" + q.Encode()
	return pictureImageDTO{URL: imgURL, ThumbURL: thumbURL}
}

// inventory reports, for every registry type present somewhere in the
// selection, which slots hold it and the ready-to-render URL of each
// populated cell. The selection is exactly paths[] — the user's selected
// tracks — carried in the request body rather than the URL: this is the
// endpoint the production 431 was reported against (a large multi-disc
// selection, as a repeated ?paths= query param, overflowed the forward_auth
// proxy's header buffer). Embedded presence is counted over paths[]; folder
// art is resolved across the distinct directories paths[] spans.
func (h *Handler) inventory(w http.ResponseWriter, r *http.Request) {
	lib, sel, status, err := h.decodeSelection(w, r)
	if err != nil {
		writeSelectionErr(w, r, status, err)
		return
	}
	// decodeSelection already guarantees sel.Paths is non-empty (or it would
	// have failed above with errNoSelection), and ResolveAlbum only ever
	// errors on an empty selection, so this cannot fail here.
	al, _ := metadataedit.ResolveAlbum(lib.Path, sel.Paths)

	matrix := al.Matrix(r.Context(), h.Reader)
	out := make([]pictureDTO, 0, len(matrix))
	for _, ts := range matrix {
		slots := make([]pictureSlotDTO, 0, len(ts.Slots))
		for _, sl := range ts.Slots {
			slots = append(slots, pictureSlotDTO{
				Slot:         sl.Slot,
				Detail:       sl.Detail, // "" for embedded, the filename for folder
				Mixed:        sl.Mixed,
				PresentCount: sl.PresentCount,
				TotalCount:   sl.TotalCount,
				Image:        pictureImageRef(lib.ID, sl.Source),
				Meta:         slotMeta(al, sl.Source),
			})
		}
		out = append(out, pictureDTO{Type: ts.Type.ID, Slots: slots})
	}
	writeJSON(w, http.StatusOK, map[string]any{"pictures": out})
}

// slotMeta describes a slot's representative image — the embedded picture's
// bytes, or the folder art file — via the same Source the inventory hands the
// image endpoint. nil when the source cannot be opened or read.
func slotMeta(al metadataedit.Album, s metadataedit.Source) *imageMetaDTO {
	data, filePath, _, err := al.Open(s)
	if err != nil {
		return nil
	}
	if filePath != "" {
		info, derr := imageinfo.DescribeFile(filePath)
		if derr != nil {
			return nil
		}
		m := toImageMeta(info)
		return &m
	}
	m := toImageMeta(imageinfo.Describe(data))
	return &m
}

// pictureImage serves the bytes of one resolved Source: the single
// representative file an inventory cell already picked (pictureImageRef). It
// never re-resolves an album selection — a Source addresses exactly one
// file, so this stays a bounded, header-safe GET even for a deep multi-disc
// path. 404 when the source cannot be opened (a stale link: the embedded
// picture was removed, or the folder file no longer exists); 422 on a type
// or slot value that fails validation (well-formed but invalid); 400 on a
// missing slot or a bad file reference.
func (h *Handler) pictureImage(w http.ResponseWriter, r *http.Request) {
	// path is absent from this endpoint's query (see the Routes doc comment);
	// resolveLibraryRel tolerates that (an empty path resolves to the library
	// root itself), so it is reused here purely for the library_id lookup.
	lib, _, status, err := h.resolveLibraryRel(r)
	if err != nil {
		writeErr(w, r, status, codeFor(status), err.Error())
		return
	}
	pt, terr := requestedType(r)
	if terr != nil {
		httperr.WriteValidation(w, r, terr.Error(), httperr.FieldError{Pointer: "/type", Detail: terr.Error()})
		return
	}
	slot := r.URL.Query().Get("slot")
	if slot == "" {
		writeErr(w, r, http.StatusBadRequest, "validation_error", "slot is required")
		return
	}
	if !validSlot(slot) {
		httperr.WriteValidation(w, r, errUnknownSlot.Error(), httperr.FieldError{Pointer: "/slot", Detail: errUnknownSlot.Error()})
		return
	}
	w.Header().Set("Cache-Control", "no-cache")

	_, src, derr := metadataedit.DecodeSource(lib.Path, r.URL.Query())
	if derr != nil {
		writeErr(w, r, http.StatusBadRequest, "validation_error", derr.Error())
		return
	}
	// type/slot go through the registry validation above (which, unlike
	// DecodeSource's raw query read, defaults an omitted type to Front
	// Cover); this keeps that default working for the one field that has one.
	src.TypeID = pt.ID
	src.Slot = slot

	data, filePath, fingerprint, operr := metadataedit.OpenSource(lib.Path, src)
	if operr != nil {
		http.NotFound(w, r)
		return
	}

	// A strong ETag off the resolved content lets a client that already has
	// this exact image skip the download entirely, and retires the old ?t=
	// cache-bust hack's reason for existing (a bust value still works — it is
	// just another query param the server ignores).
	w.Header().Set("ETag", `"`+fingerprint+`"`)
	if match := r.Header.Get("If-None-Match"); match != "" && strings.Contains(match, fingerprint) {
		w.WriteHeader(http.StatusNotModified)
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
		http.ServeFile(w, r, rp.filePath) //nolint:gosec // G703: rp.filePath is resolved via metadataedit.OpenSource, which resolves Source.RelPath through ResolveInLibrary — confined lexically to the library root (rejects absolute paths and ".." escapes via filepath.Rel; does not resolve symlinks)
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
	Slot   string        `json:"slot"`
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
		writeErr(w, r, http.StatusBadRequest, "validation_error", "invalid multipart form: "+err.Error())
		return
	}
	libID, perr := strconv.ParseUint(r.FormValue("library_id"), 10, 64)
	if perr != nil {
		writeErr(w, r, http.StatusBadRequest, "validation_error", "library_id required")
		return
	}
	slot := r.FormValue("slot")
	if slot == "" {
		writeErr(w, r, http.StatusBadRequest, "validation_error", "slot is required")
		return
	}
	if !validSlot(slot) {
		httperr.WriteValidation(w, r, errUnknownSlot.Error(), httperr.FieldError{Pointer: "/slot", Detail: errUnknownSlot.Error()})
		return
	}
	pt, terr := requestedType(r)
	if terr != nil {
		httperr.WriteValidation(w, r, terr.Error(), httperr.FieldError{Pointer: "/type", Detail: terr.Error()})
		return
	}
	paths := r.Form["paths"]
	if len(paths) == 0 {
		httperr.WriteValidation(w, r, errNoSelection.Error(), httperr.FieldError{Pointer: "/paths", Detail: errNoSelection.Error()})
		return
	}
	// Mirrors decodeSelection's cap for the JSON-body picture-selection
	// endpoints (inventory, raw-tags, removals): applyPicture reads its
	// paths[] from a multipart form instead, so it needs its own count guard
	// to keep the bound from drifting between the two decoding paths.
	if len(paths) > maxSelectionPaths {
		httperr.WriteValidation(w, r, errTooManyPaths.Error(), httperr.FieldError{Pointer: "/paths", Detail: errTooManyPaths.Error()})
		return
	}
	libModel, err := h.Store.GetLibrary(uint(libID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeErr(w, r, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeErr(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	data, ext, status, err := h.pictureImageSource(r)
	if err != nil {
		// status 0 marks an upstream download failure: let the upstream mapping
		// pick the status and the human message.
		if status == 0 {
			httperr.WriteUpstream(w, r, err, "The image could not be downloaded. Try again in a moment.")
			return
		}
		writeErr(w, r, status, codeFor(status), err.Error())
		return
	}

	// applyPicture's own per-path validation predates ResolveAlbum and stays
	// strict, unlike the read/delete endpoints above: a save must not
	// silently write fewer files than the caller asked for. ResolveAlbum
	// itself is lenient — it skips an unresolvable entry rather than failing
	// the whole call — so that leniency is not relied on here.
	for _, p := range paths {
		if _, rerr := metadataedit.ResolveInLibrary(libModel.Path, p); rerr != nil {
			writeErr(w, r, http.StatusBadRequest, "validation_error", rerr.Error())
			return
		}
	}
	// Every paths[] entry already resolved individually above, and paths is
	// non-empty (checked earlier), so ResolveAlbum — which only ever errors
	// on an empty selection — cannot fail here.
	al, _ := metadataedit.ResolveAlbum(libModel.Path, paths)

	if status, serr := h.savePictureToSlot(slot, pt, al, ext, data); serr != nil {
		writeErr(w, r, status, codeFor(status), serr.Error())
		return
	}

	// Re-index the folder's tracks: the embedded slot changed their tags, and
	// folder writes change which image the album should serve (reconcile
	// redetects album.CoverPath).
	rs := h.rescanSaved(r.Context(), libModel.ID, al.Tracks())
	writeJSON(w, http.StatusOK, applyPictureResult{OK: true, Slot: slot, Type: pt.ID, Rescan: rs})
}

// savePictureToSlot writes the image bytes to the requested slot, returning
// an HTTP status + error on failure (0, nil on success). Both slots are on
// disk; the DB catches up through the caller's rescan, which re-reads the tags
// and re-detects the album's cover file.
func (h *Handler) savePictureToSlot(slot string, pt metadataedit.PictureType, al metadataedit.Album, ext string, data []byte) (int, error) {
	switch slot {
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

// removals clears one type+slot cell across the selection: POST, not
// DELETE-with-body, so the client never has to attach a body to a DELETE verb
// (see docs/superpowers/specs/2026-08-22-metadata-picture-api-header-safe-redesign.md).
// Folder removal fans out across every directory the selection spans
// (mirroring the save fan-out for a multi-disc album); embedded removal
// targets exactly the selected tracks. Clearing an already-empty cell still
// answers {ok:true} — removing a file that is not there, or a picture a
// track never had, is a no-op, not an error.
func (h *Handler) removals(w http.ResponseWriter, r *http.Request) {
	lib, sel, status, err := h.decodeSelection(w, r)
	if err != nil {
		writeSelectionErr(w, r, status, err)
		return
	}
	pt, terr := pictureTypeByIDOrDefault(sel.Type)
	if terr != nil {
		httperr.WriteValidation(w, r, terr.Error(), httperr.FieldError{Pointer: "/type", Detail: terr.Error()})
		return
	}
	if sel.Slot == "" {
		writeErr(w, r, http.StatusBadRequest, "validation_error", "slot is required")
		return
	}
	if !validSlot(sel.Slot) {
		httperr.WriteValidation(w, r, errUnknownSlot.Error(), httperr.FieldError{Pointer: "/slot", Detail: errUnknownSlot.Error()})
		return
	}
	// Resolved once: this is both the selection the removal acts on and the
	// set handed to the post-removal re-index, so the two can never disagree.
	// decodeSelection already guarantees sel.Paths is non-empty (or it would
	// have failed above with errNoSelection), and ResolveAlbum only ever
	// errors on an empty selection, so this cannot fail here.
	al, _ := metadataedit.ResolveAlbum(lib.Path, sel.Paths)
	switch sel.Slot {
	case "folder":
		// Mirrors the save fan-out: the art was written into every directory the
		// album spans, so remove it from each of them.
		if derr := al.DeleteFolderPicture(pt); derr != nil {
			writeErr(w, r, http.StatusInternalServerError, "internal", derr.Error())
			return
		}
	case "embedded":
		for _, trackAbs := range al.Tracks() {
			if werr := metadataedit.DeleteEmbeddedPicture(trackAbs, pt.ID); werr != nil {
				writeErr(w, r, http.StatusInternalServerError, "internal", werr.Error())
				return
			}
		}
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
	_, _ = w.Write(data) //nolint:gosec // G705: writes raw image bytes with a sniffed image/* (or application/octet-stream) Content-Type set immediately above, never text/html, so this is not an HTML/XSS sink
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
		writeErr(w, r, http.StatusBadRequest, "validation_error", "mbid or release_group is required")
		return
	}
	if h.CoverArt == nil {
		writeJSON(w, http.StatusOK, []pictureCandidateDTO{})
		return
	}
	imgs, err := h.CoverArt.List(r.Context(), mbid, releaseGroup)
	if err != nil {
		httperr.WriteUpstream(w, r, err, "Cover art could not be loaded right now. Try again in a moment.")
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

// pictureCandidateInfo downloads a Cover Art Archive candidate and reports its
// size, dimensions and format, so the editor can show what an online pick will
// write before the user saves. It mirrors the write's image_url download;
// nothing is persisted.
func (h *Handler) pictureCandidateInfo(w http.ResponseWriter, r *http.Request) {
	imgURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if imgURL == "" {
		writeErr(w, r, http.StatusBadRequest, "validation_error", "url is required")
		return
	}
	if h.CoverArt == nil {
		writeErr(w, r, http.StatusServiceUnavailable, "not_configured", "cover art search is not configured")
		return
	}
	data, _, derr := h.Downloads.GetOrLoad(imgURL, func() ([]byte, string, error) {
		return h.CoverArt.DownloadImage(r.Context(), imgURL)
	})
	if derr != nil {
		httperr.WriteUpstream(w, r, derr, "Cover art could not be loaded right now. Try again in a moment.")
		return
	}
	if len(data) == 0 {
		writeErr(w, r, http.StatusNotFound, "not_found", "no image found")
		return
	}
	writeJSON(w, http.StatusOK, toImageMeta(imageinfo.Describe(data)))
}

// pictureImageSource returns the image bytes and normalized extension from
// either an uploaded "image" file part or a downloaded "image_url".
//
// status is the HTTP status to answer with on failure; a zero status alongside
// a non-nil err means the failure came from the external image host, which the
// caller maps through httperr.WriteUpstream.
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
		data, ext, derr := h.Downloads.GetOrLoad(url, func() ([]byte, string, error) {
			return h.CoverArt.DownloadImage(r.Context(), url)
		})
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
