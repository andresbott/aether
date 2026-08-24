package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/andresbott/aether/app/router/handlers/httperr"
	"github.com/andresbott/aether/internal/metadataedit"
	"github.com/andresbott/aether/libs/acoustid"
	"gorm.io/gorm"
)

// IdentifyService resolves an audio file to MusicBrainz recording candidates
// by acoustic fingerprint. Satisfied by *identify.Identifier. A nil service on
// the Handler means identification is unavailable (fpcalc or the AcoustID key
// is missing) — the capabilities endpoint reports it and identify returns 503.
type IdentifyService interface {
	IdentifyFile(ctx context.Context, absPath string) ([]acoustid.Recording, error)
}

// defaultIdentifyUnavailableReason is used when identification is off but the
// application did not say why, so the UI never has to invent an explanation.
const defaultIdentifyUnavailableReason = "audio identification is not available on this server"

func (h *Handler) capabilities(w http.ResponseWriter, _ *http.Request) {
	body := map[string]any{"identify": h.Identifier != nil}
	if h.Identifier == nil {
		reason := h.IdentifyUnavailableReason
		if reason == "" {
			reason = defaultIdentifyUnavailableReason
		}
		body["identify_unavailable_reason"] = reason
	}
	writeJSON(w, http.StatusOK, body)
}

type identifyRequest struct {
	LibraryID uint     `json:"library_id"`
	Paths     []string `json:"paths"`
}

type identifyArtistDTO struct {
	Name string `json:"name"`
	MBID string `json:"mbid"`
}

type identifyReleaseDTO struct {
	ReleaseMBID      string `json:"release_mbid"`
	ReleaseGroupMBID string `json:"release_group_mbid"`
	Album            string `json:"album"`
	Year             int    `json:"year"`
	TrackNumber      int    `json:"track_number"`
	DiscNumber       int    `json:"disc_number"`
}

type identifyCandidateDTO struct {
	Score         float64              `json:"score"`
	RecordingMBID string               `json:"recording_mbid"`
	Title         string               `json:"title"`
	Artists       []identifyArtistDTO  `json:"artists"`
	Releases      []identifyReleaseDTO `json:"releases"`
}

type identifyResultDTO struct {
	Path       string                 `json:"path"`
	Candidates []identifyCandidateDTO `json:"candidates"`
	Error      string                 `json:"error,omitempty"`
}

func (h *Handler) identify(w http.ResponseWriter, r *http.Request) {
	if h.Identifier == nil {
		reason := h.IdentifyUnavailableReason
		if reason == "" {
			reason = defaultIdentifyUnavailableReason
		}
		writeErr(w, r, http.StatusServiceUnavailable, "identify_unavailable", reason)
		return
	}
	var body identifyRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, r, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	if body.LibraryID == 0 || len(body.Paths) == 0 {
		writeErr(w, r, http.StatusBadRequest, "validation_error", "library_id and paths are required")
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

	// Fingerprint + lookup sequentially: fpcalc is CPU-bound and the AcoustID
	// client is rate-limited, so parallelism buys nothing here. Per-path errors
	// are reported per row, not as a request failure.
	results := make([]identifyResultDTO, 0, len(body.Paths))
	for _, p := range body.Paths {
		abs, rerr := metadataedit.ResolveInLibrary(libModel.Path, p)
		if rerr != nil {
			results = append(results, identifyResultDTO{Path: p, Candidates: []identifyCandidateDTO{}, Error: rerr.Error()})
			continue
		}
		recs, ierr := h.Identifier.IdentifyFile(r.Context(), abs)
		if ierr != nil {
			results = append(results, identifyResultDTO{Path: p, Candidates: []identifyCandidateDTO{}, Error: ierr.Error()})
			continue
		}
		results = append(results, identifyResultDTO{Path: p, Candidates: toCandidateDTOs(recs)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func toCandidateDTOs(recs []acoustid.Recording) []identifyCandidateDTO {
	out := make([]identifyCandidateDTO, 0, len(recs))
	for _, rec := range recs {
		c := identifyCandidateDTO{
			Score:         rec.Score,
			RecordingMBID: rec.MBID,
			Title:         rec.Title,
			Artists:       make([]identifyArtistDTO, 0, len(rec.Artists)),
			Releases:      make([]identifyReleaseDTO, 0, len(rec.Release)),
		}
		for _, a := range rec.Artists {
			c.Artists = append(c.Artists, identifyArtistDTO{Name: a.Name, MBID: a.MBID})
		}
		for _, rel := range rec.Release {
			c.Releases = append(c.Releases, identifyReleaseDTO{
				ReleaseMBID:      rel.MBID,
				ReleaseGroupMBID: rel.ReleaseGroupMBID,
				Album:            rel.Title,
				Year:             rel.Year,
				TrackNumber:      rel.TrackNumber,
				DiscNumber:       rel.DiscNumber,
			})
		}
		out = append(out, c)
	}
	return out
}
