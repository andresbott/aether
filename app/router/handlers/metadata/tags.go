package metadata

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/andresbott/aether/internal/metadataedit"
	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/internal/tags"
	"github.com/go-bumbu/http/problemjson"
	"github.com/gorilla/mux"
)

// TagsHandler serves the structured and raw tag-editing endpoints of the
// metadata editor: browsing folders, reading a selection's tracks, writing a
// structured patch, and the raw tag map. Every write enqueues a background
// re-index of the touched files so the edit becomes visible in the music UI
// without the request blocking on it; it never writes to the library index
// directly.
type TagsHandler struct {
	// Folders is the set of configured scan folders the editor can address.
	Folders *scanfolder.Set
	Reader  tags.Reader
	// Reindex enqueues a background re-index of the files a write touched; nil
	// disables it.
	Reindex Reindexer
	// RawTagReader reads a file's complete tag map; nil defaults to
	// tags.ReadRawTags. Overridable for tests.
	RawTagReader func(absPath string) (map[string][]string, error)
	// UnsupportedReader lists a file's hidden-frame descriptors; nil defaults
	// to tags.ReadUnsupported. Overridable for tests.
	UnsupportedReader func(absPath string) ([]string, error)
	// Problems writes this handler's application/problem+json error responses.
	Problems *problemjson.Writer
}

// Routes mounts the tag-editing endpoints under an already-subrouted
// mux.Router. Endpoints live beneath /metadata/.
func (h *TagsHandler) Routes(r *mux.Router) {
	r.Path("/metadata/folders").Methods(http.MethodGet).HandlerFunc(h.folders)
	r.Path("/metadata/tracks/raw-tags").Methods(http.MethodPost).HandlerFunc(h.rawTags)
	r.Path("/metadata/tracks").Methods(http.MethodGet).HandlerFunc(h.tracks)
	r.Path("/metadata/tracks").Methods(http.MethodPut).HandlerFunc(h.updateTracks)
}

type folderDTO struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	HasSubfolders bool   `json:"has_subfolders"`
}

// maxFolderSearchResults bounds a folder search so a one-letter query on a huge
// scan folder returns a manageable response; the UI shows a "refine your search"
// hint when the result is truncated.
const maxFolderSearchResults = 500

func (h *TagsHandler) folders(w http.ResponseWriter, r *http.Request) {
	_, abs, status, err := resolveFolderRel(h.Folders, r)
	if err != nil {
		h.Problems.Write(w, r, status, codeFor(status), err.Error())
		return
	}
	// A `q` turns the endpoint into a filter: instead of one directory level it
	// walks the whole subtree under `abs` (the scan folder root when no path is
	// given) and returns every folder whose name matches, so the picker can find
	// a deep folder without the user expanding to it first.
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		matches, truncated, err := metadataedit.SearchFolders(
			abs, q, metadataedit.ListFoldersOptions{}, maxFolderSearchResults)
		if err != nil {
			h.Problems.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
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
	// the scan folder root by ResolveInLibrary, which checks paths lexically, so
	// a followed link would be a way out of the root.
	folders, err := metadataedit.ListFolders(abs, metadataedit.ListFoldersOptions{})
	if err != nil {
		h.Problems.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
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
	ReleaseTypes     []string `json:"release_types"`
	MBArtistIDs      []string `json:"mb_artist_ids"`
	MBAlbumArtistIDs []string `json:"mb_album_artist_ids"`
	MBRecordingID    string   `json:"mb_recording_id"`
	MBReleaseID      string   `json:"mb_release_id"`
	MBReleaseGroupID string   `json:"mb_release_group_id"`
	Error            string   `json:"error,omitempty"`
}

func (h *TagsHandler) tracks(w http.ResponseWriter, r *http.Request) {
	folder, abs, status, err := resolveFolderRel(h.Folders, r)
	if err != nil {
		h.Problems.Write(w, r, status, codeFor(status), err.Error())
		return
	}
	rows, err := metadataedit.ListTracks(r.Context(), folder.Path, abs, h.Reader)
	if err != nil {
		h.Problems.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
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
			ReleaseTypes:     t.ReleaseTypes,
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
	ScanFolder string   `json:"scan_folder"`
	Paths      []string `json:"paths"`
	Fields     fields   `json:"fields"`
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
	ReleaseTypes     *[]string          `json:"release_types,omitempty"`
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

// validateUpdateFields returns a validation error message ("" = valid) for the
// endpoint-specific `fields` payload of an update request. The {scan_folder,
// paths[]} selection is validated separately by resolveSelection; this checks
// only what is unique to updateTracks.
func validateUpdateFields(f fields) string {
	// Every field in `fields` is an optional pointer, so an omitted `fields`
	// key and a present-but-empty `fields: {}` both decode to the zero value:
	// a request that writes nothing yet reports every row ok. The spec marks
	// fields required; reject the no-op so contract and behavior agree.
	if f == (fields{}) {
		return "fields must set at least one value to write"
	}
	// MB-ID maps are keyed by the current artist names; changing the name field
	// in the same request would write a positionally-misaligned MB-ID tag.
	// Reject so a corrupt tag is never written — the two edits must be saved
	// separately.
	if f.Artists != nil && f.ArtistMBIDs != nil {
		return "cannot change artist names and set artist MusicBrainz IDs in the same request; save them separately"
	}
	if f.AlbumArtists != nil && f.AlbumArtistMBIDs != nil {
		return "cannot change album-artist names and set album-artist MusicBrainz IDs in the same request; save them separately"
	}
	// The raw editor must not touch keys the structured editor owns — its
	// edits would bypass the per-field patch logic (MB-ID alignment,
	// multi-value policies) and silently corrupt those tags.
	if f.RawTags != nil {
		for key := range *f.RawTags {
			if metadataedit.IsManagedTag(key) {
				return "tag " + key + " is managed by the metadata editor; edit it through the form fields"
			}
		}
	}
	// Embedded cover art lives in unsupported data too (APIC/covr/...) but is
	// managed through the cover endpoints; refuse to delete it as a hidden
	// frame.
	if f.RemoveUnsupported != nil {
		for _, d := range *f.RemoveUnsupported {
			if metadataedit.IsCoverDescriptor(d) {
				return "frame " + d + " is embedded cover art; manage it through the cover editor"
			}
		}
	}
	return ""
}

// updateTracks applies one structured patch to every file in the selection
// and reports one outcome per row. HTTP status describes the request, the
// body describes the work: a malformed body, a scan folder that is not
// configured, an over-cap selection or an escaping path is rejected before
// any row is attempted and answers problem+json; once the request is
// accepted the response is always 200 — a file that cannot be written is
// that row's error, never a transport status, even when every row failed.
// Files are written incrementally, so the per-row ledger is the only honest
// report of what is now on disk; a 5xx would invite a retry that re-writes
// the files that did land. Same rule as rawTags; see
// docs/agents/api-conventions.md, "Batch endpoints".
func (h *TagsHandler) updateTracks(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxSelectionBodyBytes)
	var body updateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	if msg := validateUpdateFields(body.Fields); msg != "" {
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", msg)
		return
	}
	folder, ok := resolveSelection(h.Folders, w, r, body.ScanFolder, body.Paths, 1, h.Problems)
	if !ok {
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
		ReleaseTypes:    body.Fields.ReleaseTypes,
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
		abs, err := metadataedit.ResolveInLibrary(folder.Path, p)
		if err != nil {
			h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
		resolved = append(resolved, abs)
	}

	results := make([]updateResult, 0, len(resolved))
	written := make([]string, 0, len(resolved))
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
	}
	out := map[string]any{"results": results}
	// Only the files that were actually written need re-indexing; enqueueReindex
	// returns nil for an empty list, so an all-failed batch carries no reindex.
	if rx := enqueueReindex(r.Context(), h.Reindex, folder.Name, written); rx != nil {
		out["reindex"] = rx
	}
	// A partial write of an album-identity edit leaves the album inconsistent on
	// disk: the files that wrote carry the new identity, the ones that failed
	// keep the old one. The scanner then sees a split and creates a NEW album row
	// for the written files, stranding the old row's manual cover, stars and
	// created_at on the unwritten remnant. Warn the user; re-saving once every
	// file is writable reunites them. See internal/scanner/albumcontinuity.go.
	if identityEdit(patch) && len(written) > 0 && len(written) < len(resolved) {
		out["warning"] = albumMovedWarning
	}
	writeJSON(w, http.StatusOK, out)
}

// albumMovedWarning is surfaced when a partial write of an album-identity edit
// may have split the album; see updateTracks.
const albumMovedWarning = "Some files could not be updated, so this album is now half-renamed on disk. " +
	"Its manual cover art, stars and other saved details may have moved to a new album — " +
	"re-save the whole album once every file is writable to reunite it."

// identityEdit reports whether a patch changes any field that determines which
// album a track belongs to (album name, album artist, MusicBrainz release id).
// These are the only edits a partial failure can use to split an album; every
// other field resolves back to the same album row. See AlbumIdentityOf.
func identityEdit(p metadataedit.Patch) bool {
	return p.Album != nil || p.AlbumArtists != nil || p.MBReleaseID != nil
}
