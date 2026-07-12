package metadata

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/metadataedit"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/tags"
	"go.senan.xyz/taglib"
	"gorm.io/gorm"
)

const (
	coverMultipartMemory = 1 << 20  // 1 MiB kept in memory; larger parts spill to temp files
	maxCoverRequestBytes = 12 << 20 // 12 MiB total request cap
)

var errCoverSource = errors.New("an image file or image_url is required")

// resolvedCover describes where a folder's current cover comes from. Exactly one
// of filePath / data is set when source != "".
type resolvedCover struct {
	source   string // "db" | "folder" | "embedded" | ""
	detail   string // e.g. the folder cover filename
	filePath string // serve this file (db / folder sources)
	data     []byte // serve these bytes (embedded source)
}

// coverSources lists the sources a folder's cover may live in, in the app's
// display/precedence order.
var coverSources = []string{"db", "folder", "embedded"}

// coverForSource resolves the folder's cover from a single source, or ok=false
// when that source has no cover.
func (h *Handler) coverForSource(lib *librarySummary, abs, source string) (resolvedCover, bool) {
	switch source {
	case "db":
		album, err := h.Store.GetAlbumByTrackDir(abs)
		if err != nil || h.Assets == nil {
			return resolvedCover{}, false
		}
		if p, ok := h.Assets.Get(assetstore.KindAlbum, strconv.FormatUint(uint64(album.ID), 10)); ok {
			return resolvedCover{source: "db", filePath: p}, true
		}
	case "folder":
		if best := folderCoverName(abs); best != "" {
			return resolvedCover{source: "folder", detail: best, filePath: filepath.Join(abs, best)}, true
		}
	case "embedded":
		if tp := h.embeddedCoverTrack(lib, abs); tp != "" {
			if data, e := taglib.ReadImage(tp); e == nil && len(data) > 0 {
				return resolvedCover{source: "embedded", data: data}, true
			}
		}
	}
	return resolvedCover{}, false
}

// embeddedCoverTrack returns the path of a track in the folder that carries an
// embedded picture, preferring the scanned album's flagged track.
func (h *Handler) embeddedCoverTrack(lib *librarySummary, abs string) string {
	if album, err := h.Store.GetAlbumByTrackDir(abs); err == nil {
		if tp, e := h.Store.GetCoverTrackPath(album.ID); e == nil && tp != "" {
			return tp
		}
	}
	for _, trackAbs := range folderTrackPaths(lib, abs, h.Reader) {
		if data, ierr := taglib.ReadImage(trackAbs); ierr == nil && len(data) > 0 {
			return trackAbs
		}
	}
	return ""
}

// resolveCover finds a folder's current (winning) cover in precedence order.
func (h *Handler) resolveCover(lib *librarySummary, abs string) resolvedCover {
	for _, source := range coverSources {
		if rc, ok := h.coverForSource(lib, abs, source); ok {
			return rc
		}
	}
	return resolvedCover{}
}

// folderCoverName returns the best cover image filename directly in dir, or "".
func folderCoverName(dir string) string {
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

// currentCover serves a folder cover image. With ?source=db|folder|embedded it
// serves that specific source; otherwise it serves the winning cover. 404 when
// the requested cover does not exist.
func (h *Handler) currentCover(w http.ResponseWriter, r *http.Request) {
	lib, abs, status, err := h.resolveLibraryRel(r)
	if err != nil {
		writeErr(w, status, codeFor(status), err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-cache")

	var rc resolvedCover
	if source := r.URL.Query().Get("source"); source != "" {
		rc, _ = h.coverForSource(lib, abs, source)
	} else {
		rc = h.resolveCover(lib, abs)
	}
	switch {
	case rc.filePath != "":
		http.ServeFile(w, r, rc.filePath)
	case len(rc.data) > 0:
		writeImage(w, rc.data)
	default:
		http.NotFound(w, r)
	}
}

type coverSourceDTO struct {
	Source string `json:"source"` // "db" | "folder" | "embedded"
	Detail string `json:"detail,omitempty"`
}

// coverInfo reports every source that currently holds a cover for the folder.
func (h *Handler) coverInfo(w http.ResponseWriter, r *http.Request) {
	lib, abs, status, err := h.resolveLibraryRel(r)
	if err != nil {
		writeErr(w, status, codeFor(status), err.Error())
		return
	}
	out := make([]coverSourceDTO, 0, len(coverSources))
	for _, source := range coverSources {
		if rc, ok := h.coverForSource(lib, abs, source); ok {
			out = append(out, coverSourceDTO{Source: rc.source, Detail: rc.detail})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": out})
}

// deleteCover removes the folder's cover from a specific source: the managed
// store ("db"), the folder image file ("folder"), or the embedded art. Embedded
// removal targets the tracks named by repeated paths params (the selected files),
// falling back to every track in the folder when none are given.
func (h *Handler) deleteCover(w http.ResponseWriter, r *http.Request) {
	lib, abs, status, err := h.resolveLibraryRel(r)
	if err != nil {
		writeErr(w, status, codeFor(status), err.Error())
		return
	}
	switch r.URL.Query().Get("source") {
	case "db":
		album, aerr := h.Store.GetAlbumByTrackDir(abs)
		if aerr != nil {
			writeErr(w, http.StatusNotFound, "not_found", "album not found for this folder")
			return
		}
		if h.Assets != nil {
			_ = h.Assets.Delete(assetstore.KindAlbum, strconv.FormatUint(uint64(album.ID), 10))
		}
	case "folder":
		if best := folderCoverName(abs); best != "" {
			if rerr := os.Remove(filepath.Join(abs, best)); rerr != nil {
				writeErr(w, http.StatusInternalServerError, "internal", rerr.Error())
				return
			}
		}
		if album, aerr := h.Store.GetAlbumByTrackDir(abs); aerr == nil {
			_ = h.Store.SetAlbumCoverPath(album.ID, "")
		}
	case "embedded":
		for _, trackAbs := range h.embeddedDeleteTargets(lib, abs, r.URL.Query()["paths"]) {
			if werr := metadataedit.WriteEmbeddedCover(trackAbs, nil); werr != nil {
				writeErr(w, http.StatusInternalServerError, "internal", werr.Error())
				return
			}
			_ = h.Store.SetTrackHasEmbeddedCover(trackAbs, false)
		}
	default:
		writeErr(w, http.StatusBadRequest, "validation_error", "source must be one of db, folder, embedded")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// embeddedDeleteTargets resolves the tracks an embedded-cover removal applies to:
// the given (library-relative) selection, or every folder track when empty.
func (h *Handler) embeddedDeleteTargets(lib *librarySummary, abs string, paths []string) []string {
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

// writeImage writes raw image bytes with a sniffed image content-type.
func writeImage(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", http.DetectContentType(data))
	_, _ = w.Write(data)
}

type coverCandidateDTO struct {
	ID       string   `json:"id"`
	ThumbURL string   `json:"thumbUrl"`
	ImageURL string   `json:"imageUrl"`
	IsFront  bool     `json:"isFront"`
	Types    []string `json:"types"`
	Comment  string   `json:"comment"`
}

// coverCandidates proxies the Cover Art Archive listing for a release (and
// optional release-group) MBID.
func (h *Handler) coverCandidates(w http.ResponseWriter, r *http.Request) {
	mbid := r.URL.Query().Get("mbid")
	releaseGroup := r.URL.Query().Get("release_group")
	if mbid == "" && releaseGroup == "" {
		writeErr(w, http.StatusBadRequest, "validation_error", "mbid or release_group is required")
		return
	}
	if h.CoverArt == nil {
		writeJSON(w, http.StatusOK, []coverCandidateDTO{})
		return
	}
	imgs, err := h.CoverArt.List(r.Context(), mbid, releaseGroup)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "upstream_error", "cover art archive lookup failed: "+err.Error())
		return
	}
	out := make([]coverCandidateDTO, 0, len(imgs))
	for _, img := range imgs {
		out = append(out, coverCandidateDTO{
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

type applyCoverResult struct {
	OK     bool   `json:"ok"`
	Target string `json:"target"`
}

// applyCover saves a cover image to one of three targets: aether's managed
// store ("db"), a cover.jpg/png in the album folder ("folder"), or embedded in
// the id3 of the given tracks ("embedded"). The image source is either an
// uploaded file part ("image") or a Cover Art Archive URL ("image_url").
func (h *Handler) applyCover(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCoverRequestBytes)
	if err := r.ParseMultipartForm(coverMultipartMemory); err != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "invalid multipart form: "+err.Error())
		return
	}
	libID, perr := strconv.ParseUint(r.FormValue("library_id"), 10, 64)
	if perr != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "library_id required")
		return
	}
	target := r.FormValue("target")
	if target != "db" && target != "folder" && target != "embedded" {
		writeErr(w, http.StatusBadRequest, "validation_error", "target must be one of db, folder, embedded")
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

	data, ext, status, err := h.coverImageSource(r)
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

	if status, serr := h.saveCoverToTarget(target, resolved, ext, data); serr != nil {
		writeErr(w, status, codeFor(status), serr.Error())
		return
	}

	writeJSON(w, http.StatusOK, applyCoverResult{OK: true, Target: target})
}

// saveCoverToTarget writes the cover bytes to the requested target, returning an
// HTTP status + error on failure (0, nil on success).
func (h *Handler) saveCoverToTarget(target string, resolved []string, ext string, data []byte) (int, error) {
	switch target {
	case "db":
		album, err := h.Store.GetAlbumByTrackPath(resolved[0])
		if err != nil {
			return http.StatusNotFound, errors.New("album not found for this folder; a library scan is required before saving a cover to the aether store")
		}
		if err := h.Assets.PutManual(assetstore.KindAlbum, strconv.FormatUint(uint64(album.ID), 10), ext, data); err != nil {
			return http.StatusInternalServerError, err
		}
	case "folder":
		coverPath, err := metadataedit.WriteFolderCover(filepath.Dir(resolved[0]), ext, data)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		// Point the DB album at the new file so it serves immediately (best
		// effort — a not-yet-scanned folder still keeps the written file).
		if album, aerr := h.Store.GetAlbumByTrackPath(resolved[0]); aerr == nil {
			_ = h.Store.SetAlbumCoverPath(album.ID, coverPath)
		}
	case "embedded":
		for _, abs := range resolved {
			if err := metadataedit.WriteEmbeddedCover(abs, data); err != nil {
				return http.StatusInternalServerError, err
			}
			// Mark the flag so the embedded art serves without a rescan (best
			// effort — the file write is what matters).
			_ = h.Store.SetTrackHasEmbeddedCover(abs, true)
		}
	}
	return 0, nil
}

// coverImageSource returns the cover bytes and normalized extension from either
// an uploaded "image" file part or a downloaded "image_url".
func (h *Handler) coverImageSource(r *http.Request) (data []byte, ext string, status int, err error) {
	if file, header, ferr := r.FormFile("image"); ferr == nil {
		defer func() { _ = file.Close() }()
		data, rerr := io.ReadAll(io.LimitReader(file, maxCoverRequestBytes))
		if rerr != nil {
			return nil, "", http.StatusBadRequest, rerr
		}
		return data, coverExt(header.Filename, data), 0, nil
	}
	if url := r.FormValue("image_url"); url != "" {
		if h.CoverArt == nil {
			return nil, "", http.StatusBadRequest, errCoverSource
		}
		data, ext, derr := h.CoverArt.DownloadImage(r.Context(), url)
		if derr != nil {
			return nil, "", http.StatusBadGateway, derr
		}
		return data, ext, 0, nil
	}
	return nil, "", http.StatusBadRequest, errCoverSource
}

// coverExt derives a jpg/png extension from the upload filename, falling back to
// sniffing the image bytes.
func coverExt(filename string, data []byte) string {
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
