package libraries

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/andresbott/aether/internal/metadataedit"
)

type browseFolderDTO struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	HasSubfolders bool   `json:"has_subfolders"`
}

// browse lists the visible subdirectories of an absolute server path, for the
// library path picker in the admin UI. Defaults to the filesystem root.
func (h *Handler) browse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}
	if !filepath.IsAbs(path) {
		writeError(w, http.StatusBadRequest, "validation_error", "path must be absolute")
		return
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		writeError(w, http.StatusBadRequest, "validation_error", "path is not a readable directory")
		return
	}
	folders, err := metadataedit.ListFolders(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]browseFolderDTO, 0, len(folders))
	for _, f := range folders {
		out = append(out, browseFolderDTO{
			Name:          f.Name,
			Path:          filepath.Join(path, f.Name),
			HasSubfolders: f.HasSubfolders,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path, "folders": out})
}
