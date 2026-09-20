package libraries

import "net/http"

type filterOptionsDTO struct {
	// ScanFolders names the configured scan folders, in name order — what a
	// scan_folder filter may name.
	ScanFolders []string `json:"scan_folders"`
	// Formats, Genres and ReleaseTypes are the catalog's current vocabulary
	// for the data-driven filter fields (see store.FilterOptions). Always
	// arrays, never null, even when the catalog is empty.
	Formats      []string `json:"formats"`
	Genres       []string `json:"genres"`
	ReleaseTypes []string `json:"release_types"`
}

// filterOptions lists the values a filter builder can offer: the configured
// scan-folder names alongside the formats, genres and release types actually
// present in the catalog, so the builder can offer real values instead of
// free-text input.
func (h *Handler) filterOptions(w http.ResponseWriter, r *http.Request) {
	opts, err := h.Store.FilterOptions()
	if err != nil {
		h.Problems.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	folders := h.Folders.All()
	names := make([]string, 0, len(folders))
	for _, f := range folders {
		names = append(names, f.Name)
	}
	writeJSON(w, http.StatusOK, filterOptionsDTO{
		ScanFolders:  names,
		Formats:      nonNilStrings(opts.Formats),
		Genres:       nonNilStrings(opts.Genres),
		ReleaseTypes: nonNilStrings(opts.ReleaseTypes),
	})
}

// nonNilStrings coerces a nil slice to a non-nil empty one, so the API always
// emits a JSON array rather than null — the same coercion modelToDTO applies
// to a library's filters.
func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
