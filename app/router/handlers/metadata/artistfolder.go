package metadata

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/andresbott/aether/internal/artistimage"
	"github.com/andresbott/aether/internal/imageinfo"
	"github.com/andresbott/aether/internal/metadataedit"
	"gorm.io/gorm"
)

// artistImageBase is the folder-picture base name for an artist portrait. It is
// the highest-priority artist-image name (see internal/artistimage folderdetect),
// so it wins over any stray folder.jpg/artistthumb already in the folder.
const artistImageBase = "artist"

var errArtistImageSource = errors.New("an image file or an online pick (mbid + url) is required")

var (
	errArtistImageNotConfigured = errors.New("artist image search is not configured")
	// errURLNotCandidate is the SSRF guard's refusal, kept as a sentinel so
	// downloadArtistPick can map it to 400 even when it surfaces from inside the
	// download cache's load closure.
	errURLNotCandidate = errors.New("url is not among the candidates for this artist")
	errNoArtistImage   = errors.New("no image found for this artist")
)

// artistFolderDTO reports whether the SELECTED folder is an artist folder and, if
// so, the artist name (its own basename) and the artist image it already holds.
type artistFolderDTO struct {
	Eligible     bool   `json:"eligible"`
	Artist       string `json:"artist,omitempty"`
	Path         string `json:"path,omitempty"`
	CurrentImage string `json:"current_image,omitempty"`
	// CurrentImageMeta is the size/dimensions/format of CurrentImage, or nil
	// when the folder holds no artist image.
	CurrentImageMeta *imageMetaDTO `json:"current_image_meta,omitempty"`
}

// artistFolder reports whether the selected folder is an artist folder — one
// whose sub-folders hold albums tagged with an album artist matching the folder's
// own name (metadataedit.IsArtistFolder). This is a pure filesystem+tags question
// (no library index), so the editor can offer an artist image for a folder the
// moment it is selected, independent of any track selection.
func (h *Handler) artistFolder(w http.ResponseWriter, r *http.Request) {
	lib, abs, status, err := h.resolveLibraryRel(r)
	if err != nil {
		writeErr(w, status, codeFor(status), err.Error())
		return
	}
	// Resolve the artist folder from the selected folder — the folder itself, or
	// the nearest ancestor that is an artist folder — so selecting an album, or a
	// disc sub-folder like "CD 1", also finds the artist folder above it.
	dir, ok := metadataedit.ArtistFolderFor(r.Context(), lib.Path, abs, h.Reader)
	if !ok {
		writeJSON(w, http.StatusOK, artistFolderDTO{Eligible: false})
		return
	}
	rel, rerr := filepath.Rel(lib.Path, dir)
	if rerr != nil {
		writeErr(w, http.StatusInternalServerError, "internal", rerr.Error())
		return
	}
	current := ""
	var meta *imageMetaDTO
	if img := artistimage.BestInDir(dir); img != "" {
		current = filepath.Base(img)
		if info, ierr := imageinfo.DescribeFile(img); ierr == nil {
			m := toImageMeta(info)
			meta = &m
		}
	}
	writeJSON(w, http.StatusOK, artistFolderDTO{
		Eligible:         true,
		Artist:           filepath.Base(dir),
		Path:             filepath.ToSlash(rel),
		CurrentImage:     current,
		CurrentImageMeta: meta,
	})
}

// artistImage serves the selected artist folder's current image (the file
// artistimage.Detect would pick), 404 when there is none. It reports the on-disk
// file directly rather than the DB-resolved cover, so the editor previews exactly
// what it manages.
func (h *Handler) artistImage(w http.ResponseWriter, r *http.Request) {
	_, abs, status, err := h.resolveLibraryRel(r)
	if err != nil {
		writeErr(w, status, codeFor(status), err.Error())
		return
	}
	img := artistimage.BestInDir(abs)
	if img == "" {
		http.NotFound(w, r)
		return
	}
	// The editor busts this URL explicitly after each change, so no-cache keeps a
	// replaced image from lingering in the browser cache.
	w.Header().Set("Cache-Control", "no-cache")
	//nolint:gosec // G703: img is BestInDir of a folder confined to the library root by ResolveInLibrary, not a request path
	http.ServeFile(w, r, img)
}

// artistImageResult is the response of a successful artist-image write.
type artistImageResult struct {
	OK     bool          `json:"ok"`
	Path   string        `json:"path"` // library-relative path of the written file
	Rescan *rescanStatus `json:"rescan,omitempty"`
}

// setArtistImage writes an artist portrait as artist.<ext> into the SELECTED
// folder. The image is either an uploaded file ("image") or an online pick
// ("mbid" + "url") downloaded from the providers. Nothing is written to the
// library index: the DB catches up through a targeted rescan of one track under
// the folder, whose reconcile pass detects the file as the artist's image (a soft
// fallback — a managed/DB image still wins).
func (h *Handler) setArtistImage(w http.ResponseWriter, r *http.Request) {
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
	if strings.Trim(strings.TrimSpace(r.FormValue("path")), "/") == "" {
		writeErr(w, http.StatusBadRequest, "validation_error", "path required")
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
	abs, rerr := metadataedit.ResolveInLibrary(libModel.Path, r.FormValue("path"))
	if rerr != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", rerr.Error())
		return
	}

	data, ext, status, serr := h.artistImageSource(r)
	if serr != nil {
		// status 0 marks an upstream download failure: let the upstream mapping
		// pick the status and the human message.
		if status == 0 {
			writeUpstreamErr(w, serr, "The image could not be downloaded. Try again in a moment.")
			return
		}
		code := codeFor(status)
		if status == http.StatusServiceUnavailable {
			code = "not_configured"
		}
		writeErr(w, status, code, serr.Error())
		return
	}

	written, werr := metadataedit.WriteFolderPicture(abs, artistImageBase, ext, data)
	if werr != nil {
		writeErr(w, http.StatusInternalServerError, "internal", werr.Error())
		return
	}
	rel, _ := filepath.Rel(libModel.Path, written)

	writeJSON(w, http.StatusOK, artistImageResult{
		OK:     true,
		Path:   filepath.ToSlash(rel),
		Rescan: h.rescanArtistFolder(r, libModel.ID, abs),
	})
}

// deleteArtistImage removes the selected folder's current artist image (the file
// the serve endpoint returns), 404 when there is none, then rescans so the
// scanner's reconcile clears (or re-detects) artist.ImagePath.
func (h *Handler) deleteArtistImage(w http.ResponseWriter, r *http.Request) {
	lib, abs, status, err := h.resolveLibraryRel(r)
	if err != nil {
		writeErr(w, status, codeFor(status), err.Error())
		return
	}
	img := artistimage.BestInDir(abs)
	if img == "" {
		http.NotFound(w, r)
		return
	}
	if rerr := os.Remove(img); rerr != nil { //nolint:gosec // G703: img is BestInDir of a folder confined to the library root by ResolveInLibrary
		writeErr(w, http.StatusInternalServerError, "internal", rerr.Error())
		return
	}
	out := map[string]any{"ok": true}
	if rs := h.rescanArtistFolder(r, lib.ID, abs); rs != nil {
		out["rescan"] = rs
	}
	writeJSON(w, http.StatusOK, out)
}

// rescanArtistFolder re-indexes one representative track under the artist folder,
// which is enough for the scanner's reconcile to re-probe the artist and pick up
// (or drop) the folder image — without re-indexing the whole discography. Returns
// nil when re-indexing is disabled or the folder has no readable track.
func (h *Handler) rescanArtistFolder(r *http.Request, libraryID uint, absDir string) *rescanStatus {
	p, ok := metadataedit.FirstAudioPath(absDir, h.Reader)
	if !ok {
		return nil
	}
	return h.rescanSaved(r.Context(), libraryID, []string{p})
}

// artistImageSource returns the image bytes and normalized extension from either
// an uploaded "image" file part or an online pick ("mbid" + "url") downloaded
// from the providers. status is the HTTP status to answer with on failure; a zero
// status with a non-nil err means the failure came from an external host, mapped
// through writeUpstreamErr.
func (h *Handler) artistImageSource(r *http.Request) (data []byte, ext string, status int, err error) {
	if file, header, ferr := r.FormFile("image"); ferr == nil {
		defer func() { _ = file.Close() }()
		b, rerr := io.ReadAll(io.LimitReader(file, maxPictureRequestBytes))
		if rerr != nil {
			return nil, "", http.StatusBadRequest, rerr
		}
		return b, pictureExt(header.Filename, b), 0, nil
	}
	mbid := strings.TrimSpace(r.FormValue("mbid"))
	imgURL := strings.TrimSpace(r.FormValue("url"))
	if mbid == "" || imgURL == "" {
		return nil, "", http.StatusBadRequest, errArtistImageSource
	}
	return h.downloadArtistPick(r.Context(), mbid, imgURL)
}

// downloadArtistPick downloads an online artist-image pick — an MBID plus a
// candidate URL — through the provider chain. It re-lists the MBID's candidates
// and refuses any URL not among them (SSRF guard), so only a provider-offered
// image is ever fetched from an arbitrary host. Shared by the write and the
// pre-save metadata probe. A zero status with a non-nil err marks an upstream
// download failure for writeUpstreamErr.
func (h *Handler) downloadArtistPick(ctx context.Context, mbid, imgURL string) (data []byte, ext string, status int, err error) {
	if h.ArtistImages == nil {
		return nil, "", http.StatusServiceUnavailable, errArtistImageNotConfigured
	}
	// The list + SSRF validation + download run inside the cache's load closure,
	// so a repeat pick (probe then save) served from cache skips the re-list too.
	// A URL is only ever cached after it was validated as a candidate here.
	b, dext, derr := h.Downloads.GetOrLoad(imgURL, func() ([]byte, string, error) {
		cands, lerr := h.ArtistImages.List(ctx, mbid)
		if lerr != nil {
			return nil, "", lerr
		}
		for _, c := range cands {
			if c.FullURL == imgURL {
				return h.ArtistImages.Download(ctx, c.Provider, imgURL)
			}
		}
		return nil, "", errURLNotCandidate
	})
	if derr != nil {
		if errors.Is(derr, errURLNotCandidate) {
			return nil, "", http.StatusBadRequest, derr
		}
		return nil, "", 0, derr
	}
	if len(b) == 0 {
		return nil, "", http.StatusNotFound, errNoArtistImage
	}
	return b, dext, 0, nil
}

// artistImageCandidateInfo downloads a candidate artist portrait — an MBID plus
// a provider-offered URL — and reports its size, dimensions and format, so the
// editor can show what an online pick will write before the user saves. It uses
// the same SSRF-guarded download as the write; nothing is persisted.
func (h *Handler) artistImageCandidateInfo(w http.ResponseWriter, r *http.Request) {
	mbid := strings.TrimSpace(r.URL.Query().Get("mbid"))
	imgURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if mbid == "" || imgURL == "" {
		writeErr(w, http.StatusBadRequest, "validation_error", "mbid and url are required")
		return
	}
	data, _, status, err := h.downloadArtistPick(r.Context(), mbid, imgURL)
	if err != nil {
		if status == 0 {
			writeUpstreamErr(w, err, "The image could not be downloaded. Try again in a moment.")
			return
		}
		code := codeFor(status)
		if status == http.StatusServiceUnavailable {
			code = "not_configured"
		}
		writeErr(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toImageMeta(imageinfo.Describe(data)))
}
