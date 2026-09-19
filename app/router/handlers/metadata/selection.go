package metadata

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/andresbott/aether/internal/metadataedit"
	"github.com/andresbott/aether/internal/store"
	"github.com/go-bumbu/http/problemjson"
)

// librarySummary is the resolved library a request addresses: its id, its name
// and its root path. The name is what the post-write re-index is addressed by —
// a scan folder is identified by name, and until the editor addresses scan
// folders directly a library stands in for the folder of the same name.
type librarySummary struct {
	ID   uint
	Name string
	Path string
}

// pictureSelection is the request body of a picture-selection POST endpoint:
// the library and the selected track paths (library-relative), carried in
// the body rather than the URL so a large multi-disc selection can never
// overflow a header buffer (the production 431 this redesign fixes). See
// docs/superpowers/specs/2026-08-22-metadata-picture-api-header-safe-redesign.md.
//
// Type/Slot address one picture cell within the selection; only removals
// sets them, defaulting Type to Front Cover like the other picture endpoints
// when empty — inventory and raw-tags leave both at their zero value.
type pictureSelection struct {
	LibraryID uint     `json:"library_id"`
	Paths     []string `json:"paths"`
	Type      string   `json:"type,omitempty"`
	Slot      string   `json:"slot,omitempty"`
}

// resolveLibraryRel resolves the {library_id, path} query pair to the library
// and the absolute path of `path` within it. It is the query-string counterpart
// to the selection body helpers, shared by the folder/track browse endpoints
// and the single-file image serves.
func resolveLibraryRel(st *store.Store, r *http.Request) (lib *librarySummary, absPath string, httpStatus int, err error) {
	idStr := r.URL.Query().Get("library_id")
	id, perr := strconv.ParseUint(idStr, 10, 64)
	if perr != nil {
		return nil, "", http.StatusBadRequest, errors.New("library_id required")
	}
	libModel, gerr := st.GetLibrary(uint(id))
	if gerr != nil {
		if errors.Is(gerr, store.ErrNotFound) {
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
		Name: libModel.Name,
		Path: libModel.Path,
	}, abs, 0, nil
}

// checkPaths is the single owner of the paths[] bounds shared by every
// endpoint that accepts a {library_id, paths[]} selection. An empty selection
// (below minPaths) or one over maxSelectionPaths is well-formed-but-invalid
// input: it answers a 422 ValidationProblem itemising /paths, writes that
// response, and returns false. A valid paths[] returns true and writes
// nothing. minPaths is 1 for every endpoint except identify-album (2). Keeping
// the bound here is what stops the empty-vs-over-cap status from drifting the
// way it did when each handler hand-rolled the check.
func checkPaths(w http.ResponseWriter, r *http.Request, paths []string, minPaths int, pw *problemjson.Writer) bool {
	if len(paths) < minPaths {
		detail := errNoSelection.Error()
		if minPaths > 1 {
			detail = fmt.Sprintf("at least %d paths are required", minPaths)
		}
		pw.WriteValidation(w, r, detail, problemjson.FieldError{Pointer: "/paths", Detail: detail})
		return false
	}
	if len(paths) > maxSelectionPaths {
		pw.WriteValidation(w, r, errTooManyPaths.Error(), problemjson.FieldError{Pointer: "/paths", Detail: errTooManyPaths.Error()})
		return false
	}
	return true
}

// resolveLibrary is the single owner of the library_id lookup error mapping: a
// missing library is 404, any other store failure 500. It writes the failure
// itself and returns ok=false. library_id == 0 is not special-cased — no
// library has id 0, so the lookup answers 404, which matches the schema
// (library_id has minimum 0 and is therefore a well-formed value: "no such
// library" is a 404, not a 400).
func resolveLibrary(st *store.Store, w http.ResponseWriter, r *http.Request, id uint, pw *problemjson.Writer) (*librarySummary, bool) {
	libModel, err := st.GetLibrary(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			pw.Write(w, r, http.StatusNotFound, "not_found", err.Error())
			return nil, false
		}
		pw.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
		return nil, false
	}
	return &librarySummary{ID: libModel.ID, Name: libModel.Name, Path: libModel.Path}, true
}

// resolveSelection is the one-call validation path for a decoded selection:
// paths[] shape (checkPaths) then the library_id lookup (resolveLibrary). It
// owns every failure response and returns the resolved library with ok=true
// only when the caller may proceed. Every {library_id, paths[]} endpoint runs
// through here so status code and error-body shape are defined in one place.
func resolveSelection(st *store.Store, w http.ResponseWriter, r *http.Request, id uint, paths []string, minPaths int, pw *problemjson.Writer) (*librarySummary, bool) {
	if !checkPaths(w, r, paths, minPaths, pw) {
		return nil, false
	}
	return resolveLibrary(st, w, r, id, pw)
}

// decodeSelection decodes a picture-selection POST body and validates it
// through resolveSelection. The body is wrapped in http.MaxBytesReader(w,
// r.Body, maxSelectionBodyBytes) before decoding — defense in depth against a
// pathologically large body, now that a multi-disc selection travels in the
// body instead of the URL — so an over-cap body fails json.Decode and is
// reported through the same malformed-JSON 400 branch as any other unparseable
// body. On any failure it has already written the response and returns
// ok=false; callers only check ok.
func decodeSelection(st *store.Store, w http.ResponseWriter, r *http.Request, pw *problemjson.Writer) (*librarySummary, pictureSelection, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxSelectionBodyBytes)
	var sel pictureSelection
	if derr := json.NewDecoder(r.Body).Decode(&sel); derr != nil {
		pw.Write(w, r, http.StatusBadRequest, "validation_error", "invalid JSON: "+derr.Error())
		return nil, pictureSelection{}, false
	}
	lib, ok := resolveSelection(st, w, r, sel.LibraryID, sel.Paths, 1, pw)
	if !ok {
		return nil, pictureSelection{}, false
	}
	return lib, sel, true
}
