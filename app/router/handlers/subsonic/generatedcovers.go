package subsonic

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"slices"
)

// renderGeneratedCover renders one candidate PNG. variation is folded into the
// seed so each index is a distinct but deterministic image; an unknown style
// falls back inside generateCover to the seed-picked style.
func renderGeneratedCover(seed, title, style string, variation, size int) ([]byte, error) {
	return generateCover(fmt.Sprintf("%s#%d", seed, variation), style, title, size)
}

// availableStyles reads the configured Available set (empty on error).
func (h *Handler) availableStyles() []string {
	cs, err := h.store.GetCoverSettings(coverStyleNames(coverGen))
	if err != nil {
		return nil
	}
	return cs.AvailableStyles
}

func (h *Handler) styleAvailable(style string) bool {
	return slices.Contains(h.availableStyles(), style)
}

// getGeneratedCoverPreview renders a single candidate as PNG. With id set the
// seed/title come from the entity (so it matches what a save would store);
// without id it uses a fixed demo seed and no title (the admin panel's per-style
// thumbnails). Admin-gated — it is an editor/admin tool.
func (h *Handler) getGeneratedCoverPreview(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	style := paramStr(r, "style")
	variation := paramInt(r, "variation", 0)
	size := quantizeCoverSize(paramInt(r, "size", maxCoverSize))

	seed, title := "Aether Sample", ""
	if idStr := paramStr(r, "id"); idStr != "" {
		kind, id, err := decodeID(idStr)
		if err != nil {
			writeError(w, 0, "invalid id")
			return
		}
		meta, ok := h.resolveCoverMeta(w, r, kind, id)
		if !ok {
			return
		}
		seed, title = meta.seed, meta.title
	}

	data, err := renderGeneratedCover(seed, title, style, variation, size)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(data)
}

type candidate struct {
	Style     string `json:"style"`
	Variation int    `json:"variation"`
}

// sampleCandidates returns n candidates drawn from styles, cycling styles and
// bumping the variation index so the grid mixes styles and looks. Empty styles
// or n <= 0 returns nil.
func sampleCandidates(styles []string, _ string, n int) []candidate {
	if len(styles) == 0 || n <= 0 {
		return nil
	}
	out := make([]candidate, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, candidate{
			Style:     styles[i%len(styles)],
			Variation: rand.IntN(1000),
		})
	}
	return out
}

const (
	candidatesDefault = 9
	candidatesMax     = 24
)

// getGeneratedCoverCandidates returns a grid of candidate style+variation pairs
// for the given entity. Admin-gated — it is an editor/admin tool.
func (h *Handler) getGeneratedCoverCandidates(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	idStr := paramStr(r, "id")
	kind, id, err := decodeID(idStr)
	if err != nil {
		writeError(w, 0, "invalid id")
		return
	}
	meta, ok := h.resolveCoverMeta(w, r, kind, id)
	if !ok {
		return
	}
	count := paramInt(r, "count", candidatesDefault)
	if count <= 0 {
		count = candidatesDefault
	}
	if count > candidatesMax {
		count = candidatesMax
	}
	cands := sampleCandidates(h.availableStyles(), meta.seed, count)
	writeResponse(w, map[string]any{
		"generatedCoverCandidates": map[string]any{"candidate": cands},
	})
}
