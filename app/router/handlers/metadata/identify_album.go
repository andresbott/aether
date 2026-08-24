package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/andresbott/aether/app/router/handlers/httperr"
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

// pathErrorDTO is one requested file that produced no identification, with the
// short reason a person reads. It covers both kinds: a path refused before
// identification (outside the library) and a file that reached the resolver but
// could not be fingerprinted or looked up.
//
// The reason is always one of albumidentify's fixed sentences — never a Go error
// — because this body is rendered verbatim in the dialog and a raw error would
// leak server paths and fpcalc's stderr to the client.
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
		writeErr(w, r, http.StatusServiceUnavailable, "identify_unavailable", reason)
		return
	}
	var body identifyAlbumRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, r, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	if body.LibraryID == 0 || len(body.Paths) < minAlbumIdentifyPaths {
		writeErr(w, r, http.StatusBadRequest, "validation_error",
			"library_id and at least two paths are required")
		return
	}
	if len(body.Paths) > maxSelectionPaths {
		httperr.WriteValidation(w, r, errTooManyPaths.Error(), httperr.FieldError{Pointer: "/paths", Detail: errTooManyPaths.Error()})
		return
	}
	libModel, err := h.Store.GetLibrary(body.LibraryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeErr(w, r, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeErr(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	inputs := make([]albumidentify.Input, 0, len(body.Paths))
	pathErrors := make([]pathErrorDTO, 0)
	for _, p := range body.Paths {
		abs, rerr := metadataedit.ResolveInLibrary(libModel.Path, p)
		if rerr != nil {
			// The resolution error quotes the rejected path and the library root;
			// the user only needs to know the file is not eligible.
			pathErrors = append(pathErrors, pathErrorDTO{
				Path: p, Error: albumidentify.ReasonOutsideLibrary,
			})
			continue
		}
		album, title, trackNo, discNo := h.currentTags(r.Context(), abs)
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
		httperr.WriteUpstream(w, r, err, "album identification is temporarily unavailable")
		return
	}
	if options == nil {
		options = []albumidentify.AlbumOption{}
	}
	// Merge the resolver's per-file failures with the paths refused before
	// identification: to the user they are one list of "files this did not cover",
	// and the resolver already reduced each to a short reason.
	for _, fe := range fileErrors {
		pathErrors = append(pathErrors, pathErrorDTO{Path: fe.Path, Error: fe.Error})
	}
	writeJSON(w, http.StatusOK, map[string]any{"options": options, "errors": pathErrors})
}

// currentTags reads the tag values albumidentify uses as ranking and gap-fill
// hints. A read failure is silent: the hints are optional, and the file's real
// problem (if any) shows up on its assignment row.
func (h *Handler) currentTags(ctx context.Context, absPath string) (album, title string, trackNumber, discNumber int) {
	if h.Reader == nil {
		return "", "", 0, 0
	}
	meta, err := h.Reader.Read(ctx, absPath)
	if err != nil {
		return "", "", 0, 0
	}
	return meta.Album, meta.Title, meta.TrackNumber, meta.DiscNumber
}
