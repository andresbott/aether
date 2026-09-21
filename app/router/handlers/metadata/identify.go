package metadata

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/andresbott/aether/internal/metadataedit"
	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/internal/tags"
	"github.com/andresbott/aether/libs/acoustid"
	"github.com/go-bumbu/http/problemjson"
	"github.com/gorilla/mux"
)

// IdentifyService resolves an audio file to MusicBrainz recording candidates
// by acoustic fingerprint. Satisfied by *identify.Identifier. A nil service on
// the handler means identification is unavailable (fpcalc or the AcoustID key
// is missing) — the capabilities endpoint reports it and identify returns 503.
type IdentifyService interface {
	IdentifyFile(ctx context.Context, absPath string) ([]acoustid.Recording, error)
}

// defaultIdentifyUnavailableReason is used when identification is off but the
// application did not say why, so the UI never has to invent an explanation.
const defaultIdentifyUnavailableReason = "audio identification is not available on this server"

// IdentifyHandler serves the acoustic-identification endpoints of the metadata
// editor: per-file identify, album identify, and the capabilities probe that
// reports whether identification is available. It reads files (Reader, for the
// album-identify ranking hints) and resolves selections against the configured
// scan folders, but never writes — so it carries no reindexer.
type IdentifyHandler struct {
	// Folders is the set of configured scan folders the editor can address.
	Folders *scanfolder.Set
	Reader  tags.Reader
	// Identifier is optional: nil disables the identify endpoint and is
	// reported through /metadata/capabilities.
	Identifier IdentifyService
	// IdentifyUnavailableReason explains, in user-facing terms, why Identifier
	// is nil (missing fpcalc binary, missing AcoustID key, ...). Surfaced
	// through /metadata/capabilities so the UI can say what is missing instead
	// of silently hiding the feature. Ignored when Identifier is set.
	IdentifyUnavailableReason string
	// AlbumIdentifier maps a multi-file selection onto one release. Optional:
	// nil makes /metadata/identify-album answer 503, exactly as a nil
	// Identifier does for /metadata/identify.
	AlbumIdentifier AlbumIdentifyService
	// Problems writes this handler's application/problem+json error responses.
	Problems *problemjson.Writer
}

// Routes mounts the identify endpoints under an already-subrouted mux.Router.
// Endpoints live beneath /metadata/.
func (h *IdentifyHandler) Routes(r *mux.Router) {
	r.Path("/metadata/capabilities").Methods(http.MethodGet).HandlerFunc(h.capabilities)
	r.Path("/metadata/identify").Methods(http.MethodPost).HandlerFunc(h.identify)
	r.Path("/metadata/identify-album").Methods(http.MethodPost).HandlerFunc(h.identifyAlbum)
}

func (h *IdentifyHandler) capabilities(w http.ResponseWriter, _ *http.Request) {
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
	ScanFolder string   `json:"scan_folder"`
	Paths      []string `json:"paths"`
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

func (h *IdentifyHandler) identify(w http.ResponseWriter, r *http.Request) {
	if h.Identifier == nil {
		reason := h.IdentifyUnavailableReason
		if reason == "" {
			reason = defaultIdentifyUnavailableReason
		}
		h.Problems.Write(w, r, http.StatusServiceUnavailable, "identify_unavailable", reason)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxSelectionBodyBytes)
	var body identifyRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	folder, ok := resolveSelection(h.Folders, w, r, body.ScanFolder, body.Paths, 1, h.Problems)
	if !ok {
		return
	}

	// Fingerprint + lookup sequentially: fpcalc is CPU-bound and the AcoustID
	// client is rate-limited, so parallelism buys nothing here. Per-path errors
	// are reported per row, not as a request failure.
	results := make([]identifyResultDTO, 0, len(body.Paths))
	for _, p := range body.Paths {
		abs, rerr := metadataedit.ResolveInRoot(folder.Path, p)
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
