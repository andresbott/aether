package libraries

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/andresbott/aether/app/router/handlers/httperr"
	"github.com/andresbott/aether/internal/metadataedit"
)

type browseFolderDTO struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	HasSubfolders bool   `json:"has_subfolders"`
	IsSymlink     bool   `json:"is_symlink"`
}

// browse lists the subdirectories of an absolute server path, for the library
// path picker in the admin UI. Defaults to the filesystem root. Dot-directories
// are omitted unless show_hidden is set, since music libraries occasionally
// live under one (e.g. a hidden mount point).
//
// Symlinked directories are listed and navigable: pointing a library at a
// symlink is normal (mounted disks, curated collections of links), and the
// picker browses arbitrary absolute server paths anyway, so a link leaving the
// current subtree grants no access the caller didn't already have. The path is
// reported as typed, not resolved, so the stored library path stays the symlink
// the admin picked.
func (h *Handler) browse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}
	showHidden, err := parseBoolParam(r.URL.Query().Get("show_hidden"))
	if err != nil {
		httperr.Write(w, r, http.StatusBadRequest, "validation_error", "show_hidden must be a boolean")
		return
	}
	if !filepath.IsAbs(path) {
		httperr.Write(w, r, http.StatusBadRequest, "validation_error", "path must be absolute")
		return
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		httperr.Write(w, r, http.StatusBadRequest, "validation_error", "path is not a readable directory")
		return
	}
	folders, err := metadataedit.ListFolders(path, metadataedit.ListFoldersOptions{
		IncludeHidden:   showHidden,
		IncludeSymlinks: true,
	})
	if err != nil {
		httperr.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
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
