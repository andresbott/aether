package metadata

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/andresbott/aether/internal/metadataedit"
	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/go-bumbu/http/problemjson"
)

// pictureSelection is the request body of a picture-selection POST endpoint:
// the scan folder and the selected track paths (folder-relative), carried in
// the body rather than the URL so a large multi-disc selection can never
// overflow a header buffer (the production 431 this redesign fixes). See
// docs/superpowers/specs/2026-08-22-metadata-picture-api-header-safe-redesign.md.
//
// Type/Slot address one picture cell within the selection; only removals
// sets them, defaulting Type to Front Cover like the other picture endpoints
// when empty — inventory and raw-tags leave both at their zero value.
type pictureSelection struct {
	ScanFolder string   `json:"scan_folder"`
	Paths      []string `json:"paths"`
	Type       string   `json:"type,omitempty"`
	Slot       string   `json:"slot,omitempty"`
}

// lookupFolder is the single owner of the scan_folder lookup mapping: a missing
// name is a missing required field (400), a name that is not configured is 404.
func lookupFolder(folders *scanfolder.Set, name string) (*scanfolder.Folder, int, error) {
	if name == "" {
		return nil, http.StatusBadRequest, errors.New("scan_folder required")
	}
	f, ok := folders.ByName(name)
	if !ok {
		return nil, http.StatusNotFound, fmt.Errorf("scan folder %q is not configured", name)
	}
	return &f, 0, nil
}

// resolveFolderRel resolves the {scan_folder, path} query pair to the scan
// folder (via lookupFolder) and the absolute path of `path` within it. It is the
// query-string counterpart to the selection body helpers, shared by the
// folder/track browse endpoints and the single-file image serves.
func resolveFolderRel(folders *scanfolder.Set, r *http.Request) (folder *scanfolder.Folder, absPath string, httpStatus int, err error) {
	f, status, err := lookupFolder(folders, r.URL.Query().Get("scan_folder"))
	if err != nil {
		return nil, "", status, err
	}
	abs, rerr := metadataedit.ResolveInLibrary(f.Path, r.URL.Query().Get("path"))
	if rerr != nil {
		return nil, "", http.StatusBadRequest, rerr
	}
	return f, abs, 0, nil
}

// checkPaths is the single owner of the paths[] bounds shared by every
// endpoint that accepts a {scan_folder, paths[]} selection. An empty selection
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

// resolveFolder is lookupFolder's write-the-response form, for the endpoints
// that take the name from a body or a form rather than the query string. The
// mapping itself lives in lookupFolder; this only renders its failure.
func resolveFolder(folders *scanfolder.Set, w http.ResponseWriter, r *http.Request, name string, pw *problemjson.Writer) (*scanfolder.Folder, bool) {
	f, status, err := lookupFolder(folders, name)
	if err != nil {
		pw.Write(w, r, status, codeFor(status), err.Error())
		return nil, false
	}
	return f, true
}

// resolveSelection is the one-call validation path for a decoded selection:
// paths[] shape (checkPaths) then the scan_folder lookup (resolveFolder). It
// owns every failure response and returns the resolved folder with ok=true only
// when the caller may proceed. Every {scan_folder, paths[]} endpoint runs
// through here so status code and error-body shape are defined in one place.
func resolveSelection(folders *scanfolder.Set, w http.ResponseWriter, r *http.Request, name string, paths []string, minPaths int, pw *problemjson.Writer) (*scanfolder.Folder, bool) {
	if !checkPaths(w, r, paths, minPaths, pw) {
		return nil, false
	}
	return resolveFolder(folders, w, r, name, pw)
}

// decodeSelection decodes a picture-selection POST body and validates it
// through resolveSelection. The body is wrapped in http.MaxBytesReader(w,
// r.Body, maxSelectionBodyBytes) before decoding — defense in depth against a
// pathologically large body, now that a multi-disc selection travels in the
// body instead of the URL — so an over-cap body fails json.Decode and is
// reported through the same malformed-JSON 400 branch as any other unparseable
// body. On any failure it has already written the response and returns
// ok=false; callers only check ok.
func decodeSelection(folders *scanfolder.Set, w http.ResponseWriter, r *http.Request, pw *problemjson.Writer) (*scanfolder.Folder, pictureSelection, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxSelectionBodyBytes)
	var sel pictureSelection
	if derr := json.NewDecoder(r.Body).Decode(&sel); derr != nil {
		pw.Write(w, r, http.StatusBadRequest, "validation_error", "invalid JSON: "+derr.Error())
		return nil, pictureSelection{}, false
	}
	folder, ok := resolveSelection(folders, w, r, sel.ScanFolder, sel.Paths, 1, pw)
	if !ok {
		return nil, pictureSelection{}, false
	}
	return folder, sel, true
}
