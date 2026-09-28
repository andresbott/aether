// Package libraryfilter is the friendly layer in front of store.ScopeOf. The
// compiler is pure and fails closed: a filter it cannot honor makes a library
// match nothing, silently. This package is what keeps such a filter from being
// stored in the first place — it checks a library's filters against the rules of
// each field and against the configured scan folders, normalizes what it
// accepts, and names every problem by JSON Pointer so a form can put the message
// on the row it belongs to.
//
// Normalizing stops where a value is matched against data the scanner
// recorded: genre, release_type and path values are stored as given, with
// whitespace deciding only blankness. store.ScopeOf compares them with what the
// scanner found — genre and path exactly (see the note on nonBlank in
// store/scope.go), release_type case-insensitively (releaseTypeClause) — and the
// scanner stores names and tags as it found them, so trimming one here would let
// a value the filter-options endpoint offers be silently unmatchable once saved.
// scan_folder, format and compilation are vocabularies this package defines
// itself, so those keep being trimmed and normalized outright.
package libraryfilter

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanfolder"
)

const (
	// MaxFilters bounds the filters of one library.
	MaxFilters = 20
	// MaxValues bounds the values of one filter.
	MaxValues = 100
	// MaxTotalValues bounds the values of all of a library's filters together,
	// because the cost of a scope compounds per filter: measured at 100k tracks
	// with 100-value path filters, CountTracks — which runs on every
	// GET /libraries and every /rest call scoped to the library — goes from 3 ms
	// with one such filter to 0.77 s with five and 11.5 s with twenty.
	MaxTotalValues = 200
)

// Issue is one problem with a filter list. Pointer is a JSON Pointer relative to
// the object that carries the "filters" key, e.g. "/filters/1/values/0".
type Issue struct {
	Pointer string
	Detail  string
}

// formatRe is a file extension as the scanner stores it: lowercase, no dot.
var formatRe = regexp.MustCompile(`^[a-z0-9]{1,8}$`)

// Validate checks filters and returns them normalized, then de-duplicated. It
// reports every problem it finds, not just the first; the returned filters are
// only meaningful when there are none. No filters at all is valid — such a
// library is the whole catalog.
//
// Normalization is field-specific (see valueChecks): genre, release_type and
// path are matched against data the scanner recorded, so they are trimmed only
// to decide blankness and are otherwise kept exactly as given. scan_folder,
// format and compilation name a vocabulary this package owns, so those are
// trimmed and normalized outright.
func Validate(filters []model.LibraryFilter, folders *scanfolder.Set) ([]model.LibraryFilter, []Issue) {
	if len(filters) > MaxFilters {
		return nil, []Issue{{Pointer: "/filters", Detail: fmt.Sprintf("a library takes at most %d filters", MaxFilters)}}
	}
	out := make([]model.LibraryFilter, 0, len(filters))
	var issues []Issue
	for i, f := range filters {
		base := fmt.Sprintf("/filters/%d", i)
		check, known := valueChecks[f.Field]
		if !known {
			issues = append(issues, Issue{Pointer: base + "/field", Detail: fmt.Sprintf("unknown filter field %q", f.Field)})
			continue
		}
		switch {
		case len(f.Values) == 0:
			issues = append(issues, Issue{Pointer: base + "/values", Detail: "a filter needs at least one value"})
			continue
		case len(f.Values) > MaxValues:
			issues = append(issues, Issue{Pointer: base + "/values", Detail: fmt.Sprintf("a filter takes at most %d values", MaxValues)})
			continue
		case f.Field == model.FilterCompilation && len(f.Values) != 1:
			issues = append(issues, Issue{Pointer: base + "/values", Detail: "a compilation filter takes exactly one value, true or false"})
			continue
		}
		values := make([]string, 0, len(f.Values))
		for j, raw := range f.Values {
			// raw, not pre-trimmed: whether and how much to trim is each
			// field's own call (see valueChecks), because some fields must
			// match scanned data verbatim.
			v, err := check(raw, folders)
			if err != nil {
				issues = append(issues, Issue{Pointer: fmt.Sprintf("%s/values/%d", base, j), Detail: err.Error()})
				continue
			}
			if !slices.Contains(values, v) {
				values = append(values, v)
			}
		}
		out = append(out, model.LibraryFilter{Field: f.Field, Values: values})
	}
	// Each filter is cheap on its own but they compound (see MaxTotalValues), so
	// the request is bounded as a whole too. Counted on what was SENT, not on
	// out, so de-duplication cannot hide an oversized body.
	total := 0
	for _, f := range filters {
		total += len(f.Values)
	}
	if total > MaxTotalValues {
		issues = append(issues, Issue{Pointer: "/filters", Detail: fmt.Sprintf("a library takes at most %d filter values in total", MaxTotalValues)})
	}
	return out, issues
}

// Dangling reports the scan_folder values that name no configured folder. It is
// what a stored library looks like after its folder was renamed or removed in
// the config file: nothing is wrong with the row, and the value still selects
// the tracks that carry the old marker — until the next scan re-stamps or
// removes them, after which it matches nothing.
func Dangling(filters []model.LibraryFilter, folders *scanfolder.Set) []Issue {
	var out []Issue
	for i, f := range filters {
		if f.Field != model.FilterScanFolder {
			continue
		}
		for j, v := range f.Values {
			if _, ok := folders.ByName(v); !ok {
				out = append(out, Issue{
					Pointer: fmt.Sprintf("/filters/%d/values/%d", i, j),
					Detail:  fmt.Sprintf("scan folder %q is not configured any more; after the next scan this value matches nothing", v),
				})
			}
		}
	}
	return out
}

// valueChecks holds, per field, the rule one raw value must pass before it is
// stored; it returns the value as it is stored. scan_folder, format and
// compilation are vocabularies this package defines, so their checks trim and
// normalize the input outright. genre, release_type and path are matched
// against data the scanner recorded (see the verbatim-binding note on nonBlank
// in store/scope.go), so their checks trim only to decide blankness and
// otherwise return the value unchanged.
var valueChecks = map[model.LibraryFilterField]func(string, *scanfolder.Set) (string, error){
	model.FilterScanFolder: func(v string, folders *scanfolder.Set) (string, error) {
		v = strings.TrimSpace(v)
		if v == "" {
			return "", errEmpty
		}
		if _, ok := folders.ByName(v); !ok {
			return "", fmt.Errorf("scan folder %q is not configured", v)
		}
		return v, nil
	},
	// path is matched against file_path as the scanner recorded it (see
	// pathClause), so only blankness is decided on the trimmed value: IsAbs is
	// checked on the RAW value, so a leading space is honestly reported as
	// "not absolute" instead of silently accepted, and Clean is applied to the
	// raw value too — it collapses "." segments and slashes but never strips
	// whitespace, since a directory may legitimately end in a space.
	model.FilterPath: func(v string, _ *scanfolder.Set) (string, error) {
		if strings.TrimSpace(v) == "" {
			return "", errEmpty
		}
		if !filepath.IsAbs(v) {
			return "", fmt.Errorf("path %q must be absolute", v)
		}
		return filepath.Clean(v), nil
	},
	model.FilterFormat: func(v string, _ *scanfolder.Set) (string, error) {
		v = strings.ToLower(strings.TrimSpace(v))
		if !formatRe.MatchString(v) {
			return "", fmt.Errorf("%q is not a file extension (expected something like flac or mp3)", v)
		}
		return v, nil
	},
	// The empty string is a real value here: it selects albums with no release
	// type (releaseTypeClause). A typed value is compared with the album's tags
	// case-insensitively but otherwise as recorded — padding included — so it is
	// stored as given and trimmed only to decide blankness.
	model.FilterReleaseType: func(v string, _ *scanfolder.Set) (string, error) {
		if strings.TrimSpace(v) == "" {
			return "", nil
		}
		return v, nil
	},
	model.FilterCompilation: func(v string, _ *scanfolder.Set) (string, error) {
		v = strings.TrimSpace(v)
		if v != "true" && v != "false" {
			return "", fmt.Errorf("a compilation filter is true or false, not %q", v)
		}
		return v, nil
	},
	// genre is matched against the track's tag exactly as the scanner stored
	// it (nonBlank in store/scope.go keeps it verbatim), so a genre really
	// tagged "Rock " must stay selectable: trim only to decide blankness.
	model.FilterGenre: func(v string, _ *scanfolder.Set) (string, error) {
		if strings.TrimSpace(v) == "" {
			return "", errEmpty
		}
		return v, nil
	},
}

var errEmpty = errors.New("a value must not be empty")
