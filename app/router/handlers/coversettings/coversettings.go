package coversettings

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/libs/covergen"
	"github.com/go-bumbu/http/problemjson"
	"github.com/gorilla/mux"
)

type Handler struct {
	Store    *store.Store
	Gen      *covergen.Generator
	Problems *problemjson.Writer
}

func (h *Handler) Routes(r *mux.Router) {
	r.Path("/settings/generated-covers").Methods(http.MethodGet).HandlerFunc(h.get)
	r.Path("/settings/generated-covers").Methods(http.MethodPut).HandlerFunc(h.put)
}

type styleInfo struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}
type response struct {
	Styles    []styleInfo `json:"styles"`
	Default   []string    `json:"default"`
	Available []string    `json:"available"`
}
type input struct {
	Default   []string `json:"default"`
	Available []string `json:"available"`
}

func (h *Handler) styleNames() []string {
	names := make([]string, 0)
	for _, s := range h.Gen.Styles() {
		names = append(names, s.Name())
	}
	return names
}

// label Title-cases a style name (classic → Classic). A small override map can
// be added here if a style ever needs a non-derivable label.
func label(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func (h *Handler) styleInfos() []styleInfo {
	out := make([]styleInfo, 0)
	for _, n := range h.styleNames() {
		out = append(out, styleInfo{Name: n, Label: label(n)})
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	cs, err := h.Store.GetCoverSettings(h.styleNames())
	if err != nil {
		h.Problems.Write(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, response{Styles: h.styleInfos(), Default: cs.DefaultStyles, Available: cs.AvailableStyles})
}

func (h *Handler) put(w http.ResponseWriter, r *http.Request) {
	var in input
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", "invalid body")
		return
	}
	known := h.styleNames()
	clean := func(names []string) ([]string, bool) {
		seen := map[string]bool{}
		out := make([]string, 0, len(names))
		for _, n := range names {
			if !slices.Contains(known, n) {
				return nil, false
			}
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
		return out, true
	}
	def, okD := clean(in.Default)
	avail, okA := clean(in.Available)
	if !okD || !okA {
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", "unknown style name")
		return
	}
	if len(def) == 0 {
		h.Problems.Write(w, r, http.StatusBadRequest, "validation_error", "at least one default style is required")
		return
	}
	if err := h.Store.SetCoverSettings(model.CoverSettings{DefaultStyles: def, AvailableStyles: avail}); err != nil {
		h.Problems.Write(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, response{Styles: h.styleInfos(), Default: def, Available: avail})
}
