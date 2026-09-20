package libraries

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/andresbott/aether/internal/metadataedit"
	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/go-bumbu/http/problemjson"
)

type browseFolderDTO struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	HasSubfolders bool   `json:"has_subfolders"`
	IsSymlink     bool   `json:"is_symlink"`
}

// browse lists the subdirectories of an absolute server path, for the
// library path FILTER's picker in the admin UI (a library no longer names a
// directory of its own — see docs/agents/architecture.md's "Scan folders"
// section). Confined to the configured scan folders: a path must be spelled
// under one of their roots, else 422 at /path.
//
// An omitted/empty path lists the roots themselves, straight from h.Folders,
// WITHOUT touching the filesystem: a root that is not mounted right now must
// still be listed so the admin can see and pick it, and stat-ing every root
// on a request path risks hanging the request on a dead network mount (the
// same lesson the scan preflight already learned — see
// scanfolder.Folder.AvailableWithin). has_subfolders/is_symlink are therefore
// assumed (true/false) rather than probed for a root entry.
//
// Containment is checked LEXICALLY on purpose — scanfolder.Set.Containing,
// the same first-probe shape internal/pathguard uses before ever resolving a
// symlink: this offers what is spelled under a root without resolving
// anything, so a symlink INSIDE a root stays listed and navigable even
// though it may resolve elsewhere, rather than being refused for pointing
// outside. The reported path is the one typed/followed, not resolved, so
// pointing a library's path filter at a symlink keeps that symlink as the
// stored value.
func (h *Handler) browse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	showHidden, err := parseBoolParam(r.URL.Query().Get("show_hidden"))
	if err != nil {
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", "show_hidden must be a boolean")
		return
	}
	if path == "" {
		writeJSON(w, http.StatusOK, map[string]any{"path": "", "folders": rootFolderDTOs(h.Folders)})
		return
	}
	if !filepath.IsAbs(path) {
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", "path must be absolute")
		return
	}
	path = filepath.Clean(path)
	if _, ok := h.Folders.Containing(path); !ok {
		h.Problems.WriteValidation(w, r, "path is outside every configured scan folder",
			problemjson.FieldError{Pointer: "/path", Detail: "path is outside every configured scan folder"})
		return
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", "path is not a readable directory")
		return
	}
	folders, err := metadataedit.ListFolders(path, metadataedit.ListFoldersOptions{
		IncludeHidden:   showHidden,
		IncludeSymlinks: true,
	})
	if err != nil {
		h.Problems.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]browseFolderDTO, 0, len(folders))
	for _, f := range folders {
		out = append(out, browseFolderDTO{
			Name:          f.Name,
			Path:          filepath.Join(path, f.Name),
			HasSubfolders: f.HasSubfolders,
			IsSymlink:     f.IsSymlink,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path, "folders": out})
}

// rootFolderDTOs lists the configured scan folders as browse entries,
// without touching the filesystem (see browse's doc comment): has_subfolders
// is always true and is_symlink always false, since telling either apart
// would mean stat-ing every root. A root that is not mounted right now is
// still listed — that is a scan/availability concern (GET /scan-folders,
// scanfolder.Folder.Available), not the picker's.
func rootFolderDTOs(folders *scanfolder.Set) []browseFolderDTO {
	roots := folders.All()
	out := make([]browseFolderDTO, 0, len(roots))
	for _, f := range roots {
		out = append(out, browseFolderDTO{Name: f.Name, Path: f.Path, HasSubfolders: true, IsSymlink: false})
	}
	return out
}
