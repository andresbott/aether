// Package scanfolders serves the read-only list of the directories the server
// scans. They are declared in the config file and nowhere else, so there is
// nothing to create, change or delete here: the endpoint exists so the admin UI
// can show what is configured and the metadata editor can offer the folders to
// browse.
package scanfolders

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/internal/store"
	"github.com/go-bumbu/http/problemjson"
	"github.com/gorilla/mux"
)

// probeTimeout bounds each root's availability check, so a dead mount costs the
// request this long at most — the probes run side by side.
const probeTimeout = 2 * time.Second

type Handler struct {
	Folders *scanfolder.Set
	Store   *store.Store
	// Problems writes this handler's application/problem+json error responses.
	Problems *problemjson.Writer
}

type folderDTO struct {
	Name            string   `json:"name"`
	Path            string   `json:"path"`
	ExcludePatterns []string `json:"exclude_patterns"`
	FollowSymlinks  bool     `json:"follow_symlinks"`
	// Available is false when the root cannot be scanned right now; Problem then
	// says why (missing, not a directory, a symlinked root, a mount that does not
	// answer).
	Available bool   `json:"available"`
	Problem   string `json:"problem,omitempty"`
	// TrackCount counts the tracks stamped with this folder's name.
	TrackCount int64 `json:"track_count"`
}

func (h *Handler) Routes(r *mux.Router) {
	r.Path("/scan-folders").Methods(http.MethodGet).HandlerFunc(h.list)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	counts, err := h.Store.TrackCountsByScanFolder()
	if err != nil {
		h.Problems.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	folders := h.Folders.All()
	out := make([]folderDTO, len(folders))
	var wg sync.WaitGroup
	for i, f := range folders {
		patterns := f.ExcludePatterns
		if patterns == nil {
			patterns = []string{}
		}
		out[i] = folderDTO{
			Name:            f.Name,
			Path:            f.Path,
			ExcludePatterns: patterns,
			FollowSymlinks:  f.FollowSymlinks,
			Available:       true,
			TrackCount:      counts[f.Name],
		}
		wg.Add(1)
		go func(i int, f scanfolder.Folder) {
			defer wg.Done()
			// Each goroutine writes its own element only.
			if err := f.AvailableWithin(probeTimeout); err != nil {
				out[i].Available = false
				out[i].Problem = err.Error()
			}
		}(i, f)
	}
	wg.Wait()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string][]folderDTO{"scan_folders": out})
}
