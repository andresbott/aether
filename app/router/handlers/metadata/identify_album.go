package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/andresbott/aether/internal/albumidentify"
	"github.com/andresbott/aether/internal/metadataedit"
	"gorm.io/gorm"
)

// AlbumIdentifyService maps a set of files onto a single MusicBrainz release,
// returning the candidate albums ranked best-first plus per-file errors.
// Satisfied by *albumidentify.Resolver.
type AlbumIdentifyService interface {
	Resolve(ctx context.Context, inputs []albumidentify.Input) ([]albumidentify.AlbumOption, []albumidentify.FileError, error)
}

// minAlbumIdentifyPaths: identifying "the album" of a single file is just
// per-file identification, which /metadata/identify already does better.
const minAlbumIdentifyPaths = 2

type identifyAlbumRequest struct {
	LibraryID uint     `json:"library_id"`
	Paths     []string `json:"paths"`
}

// pathErrorDTO is one requested path that never reached the resolver (it
// escaped the library root, or was otherwise unusable).
type pathErrorDTO struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

func (h *Handler) identifyAlbum(w http.ResponseWriter, r *http.Request) {
	if h.AlbumIdentifier == nil {
		reason := h.IdentifyUnavailableReason
		if reason == "" {
			reason = defaultIdentifyUnavailableReason
		}
		writeErr(w, http.StatusServiceUnavailable, "identify_unavailable", reason)
		return
	}
	var body identifyAlbumRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	if body.LibraryID == 0 || len(body.Paths) < minAlbumIdentifyPaths {
		writeErr(w, http.StatusBadRequest, "validation_error",
			"library_id and at least two paths are required")
		return
	}
	if len(body.Paths) > maxIdentifyPaths {
		writeErr(w, http.StatusBadRequest, "validation_error", "too many paths in one request")
		return
	}
	libModel, err := h.Store.GetLibrary(body.LibraryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	inputs := make([]albumidentify.Input, 0, len(body.Paths))
	pathErrors := make([]pathErrorDTO, 0)
	for _, p := range body.Paths {
		abs, rerr := metadataedit.ResolveInLibrary(libModel.Path, p)
		if rerr != nil {
			pathErrors = append(pathErrors, pathErrorDTO{Path: p, Error: rerr.Error()})
			continue
		}
		album, title, trackNo, discNo := h.currentTags(abs)
		inputs = append(inputs, albumidentify.Input{
			Path:               p,
			AbsPath:            abs,
			CurrentAlbum:       album,
			CurrentTitle:       title,
			CurrentTrackNumber: trackNo,
			CurrentDiscNumber:  discNo,
		})
	}
	if len(inputs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"options": []albumidentify.AlbumOption{},
			"errors":  pathErrors,
		})
		return
	}

	options, fileErrors, err := h.AlbumIdentifier.Resolve(r.Context(), inputs)
	if err != nil {
		// The resolver only fails when the outbound lookups do; that is an
		// upstream problem, not a bad request.
		writeUpstreamErr(w, err, "album identification is temporarily unavailable")
		return
	}
	if options == nil {
		options = []albumidentify.AlbumOption{}
	}
	// Merge fingerprint errors with path errors so the dialog shows both.
	for _, fe := range fileErrors {
		pathErrors = append(pathErrors, pathErrorDTO{Path: fe.Path, Error: fe.Error})
	}
	writeJSON(w, http.StatusOK, map[string]any{"options": options, "errors": pathErrors})
}

// currentTags reads the tag values albumidentify uses as ranking and gap-fill
// hints. A read failure is silent: the hints are optional, and the file's real
// problem (if any) shows up on its assignment row.
func (h *Handler) currentTags(absPath string) (album, title string, trackNumber, discNumber int) {
	if h.Reader == nil {
		return "", "", 0, 0
	}
	meta, err := h.Reader.Read(absPath)
	if err != nil {
		return "", "", 0, 0
	}
	return meta.Album, meta.Title, meta.TrackNumber, meta.DiscNumber
}
