package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/andresbott/aether/internal/coverart"
	"github.com/andresbott/aether/internal/imagecache"
	"github.com/andresbott/aether/internal/metadataedit"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
	"github.com/andresbott/aether/internal/upstream"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// CoverArtClient looks up and downloads album covers from the Cover Art
// Archive. Satisfied by *coverart.Client.
type CoverArtClient interface {
	List(ctx context.Context, releaseMBID, releaseGroupMBID string) ([]coverart.CoverImage, error)
	DownloadImage(ctx context.Context, imageURL string) ([]byte, string, error)
}

// TrackRescanner re-reads the tags of specific files and updates the library
// index. Satisfied by *scanner.Scanner. It exists so the editor can make an
// edit visible in the music UI without waiting for a scan task.
type TrackRescanner interface {
	RescanPaths(ctx context.Context, libraryID uint, absPaths []string) (scanner.ScanStats, error)
}

// Handler serves the metadata editor API. It edits metadata that lives on disk —
// the songs' tags and the album folder's art files — and never writes to the
// library index itself: every write is followed by a rescan of the touched
// files. It depends on the library portion of the store (to resolve library_id
// to a root path), a tags.Reader for on-demand per-file reads, and a Cover Art
// Archive client as an online image source.
type Handler struct {
	Store    *store.Store
	Reader   tags.Reader
	CoverArt CoverArtClient
	// Images serves display-sized copies of the editor's picture cells, so a
	// grid of thumbnails does not download full-resolution scans. Optional: nil
	// makes every cell serve its original.
	Images *imagecache.Cache
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
	// RawTagReader reads a file's complete tag map; nil defaults to
	// taglib.ReadTags. Overridable for tests.
	RawTagReader func(absPath string) (map[string][]string, error)
	// UnsupportedReader lists a file's hidden-frame descriptors; nil defaults
	// to taglib.ReadUnsupported. Overridable for tests.
	UnsupportedReader func(absPath string) ([]string, error)
	// Rescan re-indexes the files a write touched, synchronously, before the
	// response is sent — so a 200 means the library index is current and the
	// UI can drop its caches without polling. nil disables re-indexing; the
	// file write still succeeds and the index catches up on the next scan.
	Rescan TrackRescanner
}

type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// rescanStatus reports the outcome of the post-write re-index. A failure is
// never fatal: the tags are already on disk, so the only consequence is that
// the library index lags until the next scan.
type rescanStatus struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// rescanSaved re-indexes absPaths, returning nil when re-indexing is disabled
// or there is nothing to do (the response then carries no "rescan" field).
//
// OK is true only when every path the library covers was re-indexed. The
// re-index swallows per-file failures (a lost SQLite write lock skips one
// track, an unreadable file lands in ScanStats.Errors) and still returns a nil
// error, so inspecting err alone would promise "the index is current" for a run
// that indexed nothing — exactly the overlapping-scan case this feature exists
// for.
func (h *Handler) rescanSaved(ctx context.Context, libraryID uint, absPaths []string) *rescanStatus {
	if h.Rescan == nil || len(absPaths) == 0 {
		return nil
	}
	stats, err := h.Rescan.RescanPaths(ctx, libraryID, absPaths)
	if err != nil {
		return &rescanStatus{Error: err.Error()}
	}
	if msg := incompleteRescanMessage(stats, len(absPaths)); msg != "" {
		return &rescanStatus{Error: msg}
	}
	return &rescanStatus{OK: true}
}

// incompleteRescanMessage describes a partial re-index ("" = every path the
// library covers made it). Per-file errors are summarised rather than
// concatenated: a folder-wide picture write can produce hundreds, and this text
// ends up in a toast.
//
// The shortfall is measured against the paths the scanner actually admitted
// (total minus the ones it deliberately skipped), never against total. The
// editor lists files the scanner does not index — it ignores the library's
// exclude patterns and its tag reader accepts extensions absent from the
// scanner's audio set — so a perfectly correct save routinely hands
// RescanPaths paths it will not index. Comparing against total would warn the
// user about a save that worked.
func incompleteRescanMessage(stats scanner.ScanStats, total int) string {
	if len(stats.Errors) > 0 {
		if len(stats.Errors) == 1 {
			return stats.Errors[0].Error()
		}
		return fmt.Sprintf("%d files could not be re-indexed; first error: %v",
			len(stats.Errors), stats.Errors[0])
	}
	indexable := total - stats.TracksSkipped
	if stats.TracksProcessed < indexable {
		return fmt.Sprintf("only %d of %d files were re-indexed", stats.TracksProcessed, indexable)
	}
	return ""
}

// Routes mounts the handler under an already-subrouted mux.Router.
// Endpoints live beneath /metadata/.
func (h *Handler) Routes(r *mux.Router) {
	r.Path("/metadata/capabilities").Methods(http.MethodGet).HandlerFunc(h.capabilities)
	r.Path("/metadata/identify").Methods(http.MethodPost).HandlerFunc(h.identify)
	r.Path("/metadata/identify-album").Methods(http.MethodPost).HandlerFunc(h.identifyAlbum)
	r.Path("/metadata/folders").Methods(http.MethodGet).HandlerFunc(h.folders)
	r.Path("/metadata/tracks/raw").Methods(http.MethodGet).HandlerFunc(h.rawTags)
	r.Path("/metadata/tracks").Methods(http.MethodGet).HandlerFunc(h.tracks)
	r.Path("/metadata/tracks").Methods(http.MethodPut).HandlerFunc(h.updateTracks)
	r.Path("/metadata/pictures").Methods(http.MethodGet).HandlerFunc(h.pictures)
	r.Path("/metadata/pictures").Methods(http.MethodPost).HandlerFunc(h.applyPicture)
	r.Path("/metadata/pictures").Methods(http.MethodDelete).HandlerFunc(h.deletePicture)
	r.Path("/metadata/pictures/image").Methods(http.MethodGet).HandlerFunc(h.pictureImage)
	r.Path("/metadata/pictures/candidates").Methods(http.MethodGet).HandlerFunc(h.pictureCandidates)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, apiError{Error: msg, Code: code})
}

// writeUpstreamErr reports a failed call to an external service. The body
// carries the upstream package's human-readable sentence (or fallback for an
// error that isn't upstream-typed) — never a raw Go error — and the status
// mirrors the upstream condition so the UI can tell "retry later" (429/504)
// from "it's broken" (502).
func writeUpstreamErr(w http.ResponseWriter, err error, fallback string) {
	status := upstream.HTTPStatus(err)
	code := "upstream_error"
	if status == http.StatusTooManyRequests {
		code = "upstream_rate_limited"
	}
	writeErr(w, status, code, upstream.UserMessage(err, fallback))
}

func (h *Handler) resolveLibraryRel(r *http.Request) (lib *librarySummary, absPath string, httpStatus int, err error) {
	idStr := r.URL.Query().Get("library_id")
	id, perr := strconv.ParseUint(idStr, 10, 64)
	if perr != nil {
		return nil, "", http.StatusBadRequest, errors.New("library_id required")
	}
	libModel, gerr := h.Store.GetLibrary(uint(id))
	if gerr != nil {
		if errors.Is(gerr, gorm.ErrRecordNotFound) {
			return nil, "", http.StatusNotFound, gerr
		}
		return nil, "", http.StatusInternalServerError, gerr
	}
	rel := r.URL.Query().Get("path")
	abs, rerr := metadataedit.ResolveInLibrary(libModel.Path, rel)
	if rerr != nil {
		return nil, "", http.StatusBadRequest, rerr
	}
	return &librarySummary{
		ID:   libModel.ID,
		Path: libModel.Path,
	}, abs, 0, nil
}

type librarySummary struct {
	ID   uint
	Path string
}

type folderDTO struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	HasSubfolders bool   `json:"has_subfolders"`
}

// maxFolderSearchResults bounds a folder search so a one-letter query on a huge
// library returns a manageable response; the UI shows a "refine your search"
// hint when the result is truncated.
const maxFolderSearchResults = 500

func (h *Handler) folders(w http.ResponseWriter, r *http.Request) {
	_, abs, status, err := h.resolveLibraryRel(r)
	if err != nil {
		writeErr(w, status, codeFor(status), err.Error())
		return
	}
	// A `q` turns the endpoint into a filter: instead of one directory level it
	// walks the whole subtree under `abs` (the library root when no path is
	// given) and returns every folder whose name matches, so the picker can find
	// a deep folder without the user expanding to it first.
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		matches, truncated, err := metadataedit.SearchFolders(
			abs, q, metadataedit.ListFoldersOptions{}, maxFolderSearchResults)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		out := make([]folderDTO, 0, len(matches))
		for _, f := range matches {
			out = append(out, folderDTO{Name: f.Name, Path: f.Path, HasSubfolders: f.HasSubfolders})
		}
		writeJSON(w, http.StatusOK, map[string]any{"folders": out, "truncated": truncated})
		return
	}
	// No symlinks here, unlike the library path picker: this tree is confined to
	// the library root by ResolveInLibrary, which checks paths lexically, so a
	// followed link would be a way out of the root.
	folders, err := metadataedit.ListFolders(abs, metadataedit.ListFoldersOptions{})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]folderDTO, 0, len(folders))
	for _, f := range folders {
		out = append(out, folderDTO{Name: f.Name, Path: f.Path, HasSubfolders: f.HasSubfolders})
	}
	writeJSON(w, http.StatusOK, map[string]any{"folders": out})
}

type trackDTO struct {
	Path             string   `json:"path"`
	Name             string   `json:"name"`
	Title            string   `json:"title"`
	Artists          []string `json:"artists"`
	AlbumArtists     []string `json:"album_artists"`
	Album            string   `json:"album"`
	Genres           []string `json:"genres"`
	Year             int      `json:"year"`
	TrackNumber      int      `json:"track_number"`
	DiscNumber       int      `json:"disc_number"`
	DiscSubtitle     string   `json:"disc_subtitle"`
	Compilation      bool     `json:"compilation"`
	MBArtistIDs      []string `json:"mb_artist_ids"`
	MBAlbumArtistIDs []string `json:"mb_album_artist_ids"`
	MBRecordingID    string   `json:"mb_recording_id"`
	MBReleaseID      string   `json:"mb_release_id"`
	MBReleaseGroupID string   `json:"mb_release_group_id"`
	Error            string   `json:"error,omitempty"`
}

func (h *Handler) tracks(w http.ResponseWriter, r *http.Request) {
	lib, abs, status, err := h.resolveLibraryRel(r)
	if err != nil {
		writeErr(w, status, codeFor(status), err.Error())
		return
	}
	rows, err := metadataedit.ListTracks(r.Context(), lib.Path, abs, h.Reader)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]trackDTO, 0, len(rows))
	for _, t := range rows {
		out = append(out, trackDTO{
			Path:             t.Path,
			Name:             t.Name,
			Title:            t.Title,
			Artists:          t.Artists,
			AlbumArtists:     t.AlbumArtists,
			Album:            t.Album,
			Genres:           t.Genres,
			Year:             t.Year,
			TrackNumber:      t.TrackNumber,
			DiscNumber:       t.DiscNumber,
			DiscSubtitle:     t.DiscSubtitle,
			Compilation:      t.Compilation,
			MBArtistIDs:      t.MBArtistIDs,
			MBAlbumArtistIDs: t.MBAlbumArtistIDs,
			MBRecordingID:    t.MBRecordingID,
			MBReleaseID:      t.MBReleaseID,
			MBReleaseGroupID: t.MBReleaseGroupID,
			Error:            t.Error,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"tracks": out})
}

type updateRequest struct {
	LibraryID uint     `json:"library_id"`
	Paths     []string `json:"paths"`
	Fields    fields   `json:"fields"`
}

type fields struct {
	Title            *string            `json:"title,omitempty"`
	Album            *string            `json:"album,omitempty"`
	Artists          *[]string          `json:"artists,omitempty"`
	AlbumArtists     *[]string          `json:"album_artists,omitempty"`
	Genres           *[]string          `json:"genres,omitempty"`
	Year             *int               `json:"year,omitempty"`
	TrackNumber      *int               `json:"track_number,omitempty"`
	DiscNumber       *int               `json:"disc_number,omitempty"`
	DiscSubtitle     *string            `json:"disc_subtitle,omitempty"`
	Compilation      *bool              `json:"compilation,omitempty"`
	ArtistMBIDs      *map[string]string `json:"artist_mbids,omitempty"`
	AlbumArtistMBIDs *map[string]string `json:"album_artist_mbids,omitempty"`
	MBRecordingID    *string            `json:"mb_recording_id,omitempty"`
	MBReleaseID      *string            `json:"mb_release_id,omitempty"`
	MBReleaseGroupID *string            `json:"mb_release_group_id,omitempty"`
	// RawTags are free-form key -> values edits from the raw editor; an empty
	// value list deletes the key. Managed keys are rejected.
	RawTags *map[string][]string `json:"raw_tags,omitempty"`
	// RemoveUnsupported lists hidden-frame descriptors to delete, as returned
	// by the raw-tags endpoint's `unsupported` field. Descriptors a file does
	// not carry are ignored.
	RemoveUnsupported *[]string `json:"remove_unsupported,omitempty"`
}

type updateResult struct {
	Path  string `json:"path"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// validateUpdateRequest returns a validation error message ("" = valid).
func validateUpdateRequest(body updateRequest) string {
	if body.LibraryID == 0 || len(body.Paths) == 0 {
		return "library_id and paths are required"
	}
	// MB-ID maps are keyed by the current artist names; changing the name field
	// in the same request would write a positionally-misaligned MB-ID tag.
	// Reject so a corrupt tag is never written — the two edits must be saved
	// separately.
	if body.Fields.Artists != nil && body.Fields.ArtistMBIDs != nil {
		return "cannot change artist names and set artist MusicBrainz IDs in the same request; save them separately"
	}
	if body.Fields.AlbumArtists != nil && body.Fields.AlbumArtistMBIDs != nil {
		return "cannot change album-artist names and set album-artist MusicBrainz IDs in the same request; save them separately"
	}
	// The raw editor must not touch keys the structured editor owns — its
	// edits would bypass the per-field patch logic (MB-ID alignment,
	// multi-value policies) and silently corrupt those tags.
	if body.Fields.RawTags != nil {
		for key := range *body.Fields.RawTags {
			if metadataedit.IsManagedTag(key) {
				return "tag " + key + " is managed by the metadata editor; edit it through the form fields"
			}
		}
	}
	// Embedded cover art lives in unsupported data too (APIC/covr/...) but is
	// managed through the cover endpoints; refuse to delete it as a hidden
	// frame.
	if body.Fields.RemoveUnsupported != nil {
		for _, d := range *body.Fields.RemoveUnsupported {
			if metadataedit.IsCoverDescriptor(d) {
				return "frame " + d + " is embedded cover art; manage it through the cover editor"
			}
		}
	}
	return ""
}

func (h *Handler) updateTracks(w http.ResponseWriter, r *http.Request) {
	var body updateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	if msg := validateUpdateRequest(body); msg != "" {
		writeErr(w, http.StatusBadRequest, "validation_error", msg)
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
	patch := metadataedit.Patch{
		Title:           body.Fields.Title,
		Album:           body.Fields.Album,
		Artists:         body.Fields.Artists,
		AlbumArtists:    body.Fields.AlbumArtists,
		Genres:          body.Fields.Genres,
		Year:            body.Fields.Year,
		TrackNumber:     body.Fields.TrackNumber,
		DiscNumber:      body.Fields.DiscNumber,
		DiscSubtitle:    body.Fields.DiscSubtitle,
		Compilation:     body.Fields.Compilation,
		ArtistMBID:      body.Fields.ArtistMBIDs,
		AlbumArtistMBID: body.Fields.AlbumArtistMBIDs,
		// Recording/album MB IDs are scalars with no positional coupling to
		// artist names, so they need no rename-vs-ID rejection rule.
		MBRecordingID:     body.Fields.MBRecordingID,
		MBReleaseID:       body.Fields.MBReleaseID,
		MBReleaseGroupID:  body.Fields.MBReleaseGroupID,
		Raw:               body.Fields.RawTags,
		RemoveUnsupported: body.Fields.RemoveUnsupported,
	}
	needMB := body.Fields.ArtistMBIDs != nil || body.Fields.AlbumArtistMBIDs != nil

	resolved := make([]string, 0, len(body.Paths))
	for _, p := range body.Paths {
		abs, err := metadataedit.ResolveInLibrary(libModel.Path, p)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
		resolved = append(resolved, abs)
	}

	results := make([]updateResult, 0, len(resolved))
	written := make([]string, 0, len(resolved))
	anyOK := false
	for i, abs := range resolved {
		var cur metadataedit.CurrentTags
		if needMB {
			meta, rerr := h.Reader.Read(r.Context(), abs)
			if rerr != nil {
				results = append(results, updateResult{Path: body.Paths[i], OK: false, Error: rerr.Error()})
				continue
			}
			cur = metadataedit.CurrentTags{
				Artists:          meta.Artist,
				ArtistMBIDs:      meta.MBArtistID,
				AlbumArtists:     meta.AlbumArtist,
				AlbumArtistMBIDs: meta.MBAlbumArtistID,
			}
		}
		if err := metadataedit.WriteMetadata(abs, patch, cur); err != nil {
			results = append(results, updateResult{Path: body.Paths[i], OK: false, Error: err.Error()})
			continue
		}
		results = append(results, updateResult{Path: body.Paths[i], OK: true})
		written = append(written, abs)
		anyOK = true
	}
	status := http.StatusOK
	if !anyOK {
		status = http.StatusInternalServerError
	}
	out := map[string]any{"results": results}
	// Only the files that were actually written need re-indexing.
	if rs := h.rescanSaved(r.Context(), libModel.ID, written); rs != nil {
		out["rescan"] = rs
	}
	writeJSON(w, status, out)
}

func codeFor(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "validation_error"
	case http.StatusNotFound:
		return "not_found"
	default:
		return "internal"
	}
}
