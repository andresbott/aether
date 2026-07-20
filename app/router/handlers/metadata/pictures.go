package metadata

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/metadataedit"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/tags"
	"gorm.io/gorm"
)

const (
	pictureMultipartMemory = 1 << 20  // 1 MiB kept in memory; larger parts spill to temp files
	maxPictureRequestBytes = 12 << 20 // 12 MiB total request cap
	frontCoverType         = "Front Cover"
)

var errPictureSource = errors.New("an image file or image_url is required")

// pictureSlots lists the storage slots a picture may live in, in the editor's
// display/preference order (embedded first). This is NOT the app's serving
// precedence — the winning album cover served elsewhere stays db → folder →
// embedded (see handlers/subsonic).
var pictureSlots = []string{"embedded", "folder", "db"}

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
func (h *Handler) selectionPaths(lib *librarySummary, abs string, paths []string) []string {
	if len(paths) == 0 {
		return folderTrackPaths(lib, abs, h.Reader)
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if trackAbs, err := metadataedit.ResolveInLibrary(lib.Path, p); err == nil {
			out = append(out, trackAbs)
		}
	}
	return out
}

// folderTrackPaths returns the absolute paths of the audio tracks in abs.
func folderTrackPaths(lib *librarySummary, abs string, reader tags.Reader) []string {
	rows, _ := metadataedit.ListTracks(lib.Path, abs, reader)
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

// pictureForSlot resolves one type+slot cell, or ok=false when empty. paths
// are the selected tracks (embedded slot only; empty = whole folder).
func (h *Handler) pictureForSlot(lib *librarySummary, abs string, pt metadataedit.PictureType, slot string, paths []string) (resolvedPicture, bool) {
	switch slot {
	case "db":
		album, err := h.Store.GetAlbumByTrackDir(abs)
		if err != nil || h.Assets == nil {
			return resolvedPicture{}, false
		}
		key := strconv.FormatUint(uint64(album.ID), 10)
		if p, ok := h.Assets.GetNamed(assetstore.KindAlbum, key, pt.FileBase); ok {
			return resolvedPicture{filePath: p}, true
		}
	case "folder":
		if name := folderPictureName(abs, pt); name != "" {
			return resolvedPicture{detail: name, filePath: filepath.Join(abs, name)}, true
		}
	case "embedded":
		for _, trackAbs := range h.embeddedProbeOrder(lib, abs, pt, paths) {
			if data, ok, err := metadataedit.ReadEmbeddedPicture(trackAbs, pt.ID); err == nil && ok {
				return resolvedPicture{data: data}, true
			}
		}
	}
	return resolvedPicture{}, false
}

// embeddedProbeOrder lists the tracks to probe for an embedded picture,
// preferring the scanned album's flagged cover track for front covers.
func (h *Handler) embeddedProbeOrder(lib *librarySummary, abs string, pt metadataedit.PictureType, paths []string) []string {
	tracks := h.selectionPaths(lib, abs, paths)
	if pt.ID != frontCoverType {
		return tracks
	}
	if album, err := h.Store.GetAlbumByTrackDir(abs); err == nil {
		if tp, e := h.Store.GetCoverTrackPath(album.ID); e == nil && tp != "" {
			return append([]string{tp}, tracks...)
		}
	}
	return tracks
}

type pictureSlotDTO struct {
	Slot   string `json:"slot"` // "embedded" | "folder" | "db"
	Detail string `json:"detail,omitempty"`
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
	tracks := h.selectionPaths(lib, abs, r.URL.Query()["paths"])

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

	var albumKey string
	if album, aerr := h.Store.GetAlbumByTrackDir(abs); aerr == nil {
		albumKey = strconv.FormatUint(uint64(album.ID), 10)
	}

	out := make([]pictureDTO, 0, len(metadataedit.PictureTypes))
	for _, pt := range metadataedit.PictureTypes {
		slots := make([]pictureSlotDTO, 0, len(pictureSlots))
		if n := embeddedCount[pt.ID]; n > 0 {
			slots = append(slots, pictureSlotDTO{
				Slot:   "embedded",
				Detail: fmt.Sprintf("%d of %d files", n, len(tracks)),
			})
		}
		if name := folderPictureName(abs, pt); name != "" {
			slots = append(slots, pictureSlotDTO{Slot: "folder", Detail: name})
		}
		if albumKey != "" && h.Assets != nil {
			if _, ok := h.Assets.GetNamed(assetstore.KindAlbum, albumKey, pt.FileBase); ok {
				slots = append(slots, pictureSlotDTO{Slot: "db"})
			}
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
		writeErr(w, http.StatusBadRequest, "validation_error", "slot must be one of embedded, folder, db")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")

	rp, ok := h.pictureForSlot(lib, abs, pt, slot, r.URL.Query()["paths"])
	switch {
	case !ok:
		http.NotFound(w, r)
	case rp.filePath != "":
		http.ServeFile(w, r, rp.filePath)
	default:
		writeImage(w, rp.data)
	}
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
	OK     bool   `json:"ok"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

// applyPicture saves an image of one picture type to one slot: aether's
// managed store ("db"), an art file in the album folder ("folder"), or
// embedded in the tags of the given tracks ("embedded"). The image source is
// either an uploaded file part ("image") or a Cover Art Archive URL
// ("image_url").
func (h *Handler) applyPicture(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPictureRequestBytes)
	if err := r.ParseMultipartForm(pictureMultipartMemory); err != nil {
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
		writeErr(w, http.StatusBadRequest, "validation_error", "target must be one of embedded, folder, db")
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

	writeJSON(w, http.StatusOK, applyPictureResult{OK: true, Target: target, Type: pt.ID})
}

// savePictureToSlot writes the image bytes to the requested slot, returning
// an HTTP status + error on failure (0, nil on success). DB-side effects
// (album cover path, embedded-cover flag) only apply to the front cover —
// they feed the app-wide cover serving, which only knows one cover per album.
func (h *Handler) savePictureToSlot(target string, pt metadataedit.PictureType, resolved []string, ext string, data []byte) (int, error) {
	switch target {
	case "db":
		album, err := h.Store.GetAlbumByTrackPath(resolved[0])
		if err != nil {
			return http.StatusNotFound, errors.New("album not found for this folder; a library scan is required before saving a picture to the aether store")
		}
		key := strconv.FormatUint(uint64(album.ID), 10)
		if err := h.Assets.PutManualNamed(assetstore.KindAlbum, key, pt.FileBase, ext, data); err != nil {
			return http.StatusInternalServerError, err
		}
	case "folder":
		picPath, err := metadataedit.WriteFolderPicture(filepath.Dir(resolved[0]), pt.FileBase, ext, data)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		// Point the DB album at the new front cover so it serves immediately
		// (best effort — a not-yet-scanned folder still keeps the written file).
		if pt.ID == frontCoverType {
			if album, aerr := h.Store.GetAlbumByTrackPath(resolved[0]); aerr == nil {
				_ = h.Store.SetAlbumCoverPath(album.ID, picPath)
			}
		}
	case "embedded":
		for _, abs := range resolved {
			if err := metadataedit.WriteEmbeddedPicture(abs, pt.ID, data, ""); err != nil {
				return http.StatusInternalServerError, err
			}
			// Mark the flag so the embedded front cover serves without a rescan
			// (best effort — the file write is what matters).
			if pt.ID == frontCoverType {
				_ = h.Store.SetTrackHasEmbeddedCover(abs, true)
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
	switch r.URL.Query().Get("slot") {
	case "db":
		album, aerr := h.Store.GetAlbumByTrackDir(abs)
		if aerr != nil {
			writeErr(w, http.StatusNotFound, "not_found", "album not found for this folder")
			return
		}
		if h.Assets != nil {
			key := strconv.FormatUint(uint64(album.ID), 10)
			if derr := h.Assets.DeleteNamed(assetstore.KindAlbum, key, pt.FileBase); derr != nil {
				writeErr(w, http.StatusInternalServerError, "internal", derr.Error())
				return
			}
		}
	case "folder":
		if name := folderPictureName(abs, pt); name != "" {
			if rerr := os.Remove(filepath.Join(abs, name)); rerr != nil {
				writeErr(w, http.StatusInternalServerError, "internal", rerr.Error())
				return
			}
		}
		if pt.ID == frontCoverType {
			if album, aerr := h.Store.GetAlbumByTrackDir(abs); aerr == nil {
				_ = h.Store.SetAlbumCoverPath(album.ID, "")
			}
		}
	case "embedded":
		for _, trackAbs := range h.selectionPaths(lib, abs, r.URL.Query()["paths"]) {
			if werr := metadataedit.DeleteEmbeddedPicture(trackAbs, pt.ID); werr != nil {
				writeErr(w, http.StatusInternalServerError, "internal", werr.Error())
				return
			}
			if pt.ID == frontCoverType {
				_ = h.Store.SetTrackHasEmbeddedCover(trackAbs, false)
			}
		}
	default:
		writeErr(w, http.StatusBadRequest, "validation_error", "slot must be one of embedded, folder, db")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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
		writeErr(w, http.StatusBadGateway, "upstream_error", "cover art archive lookup failed: "+err.Error())
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
			return nil, "", http.StatusBadGateway, derr
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
