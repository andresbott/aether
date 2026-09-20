// Package libraryfilter is the friendly layer in front of store.ScopeOf. The
// compiler is pure and fails closed: a filter it cannot honor makes a library
// match nothing, silently. This package is what keeps such a filter from being
// stored in the first place — it checks a library's filters against the rules of
// each field and against the configured scan folders, normalizes what it
// accepts, and names every problem by JSON Pointer so a form can put the message
// on the row it belongs to.
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
)

// Issue is one problem with a filter list. Pointer is a JSON Pointer relative to
// the object that carries the "filters" key, e.g. "/filters/1/values/0".
type Issue struct {
	Pointer string
	Detail  string
}

// formatRe is a file extension as the scanner stores it: lowercase, no dot.
var formatRe = regexp.MustCompile(`^[a-z0-9]{1,8}$`)

// Validate checks filters and returns them normalized: values trimmed and
// de-duplicated, paths cleaned, formats lowercased. It reports every problem it
// finds, not just the first; the returned filters are only meaningful when
// there are none. No filters at all is valid — such a library is the whole
// catalog.
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
			issues = append(issues, Issue{base + "/field", fmt.Sprintf("unknown filter field %q", f.Field)})
			continue
		}
		switch {
		case len(f.Values) == 0:
			issues = append(issues, Issue{base + "/values", "a filter needs at least one value"})
			continue
		case len(f.Values) > MaxValues:
			issues = append(issues, Issue{base + "/values", fmt.Sprintf("a filter takes at most %d values", MaxValues)})
			continue
		case f.Field == model.FilterCompilation && len(f.Values) != 1:
			issues = append(issues, Issue{base + "/values", "a compilation filter takes exactly one value, true or false"})
			continue
		}
		values := make([]string, 0, len(f.Values))
		for j, raw := range f.Values {
			v, err := check(strings.TrimSpace(raw), folders)
			if err != nil {
				issues = append(issues, Issue{fmt.Sprintf("%s/values/%d", base, j), err.Error()})
				continue
			}
			if !slices.Contains(values, v) {
				values = append(values, v)
			}
		}
		out = append(out, model.LibraryFilter{Field: f.Field, Values: values})
	}
	return out, issues
}

// valueChecks holds, per field, the rule one trimmed value must pass; it returns
// the value as it is stored.
var valueChecks = map[model.LibraryFilterField]func(string, *scanfolder.Set) (string, error){
	model.FilterScanFolder: func(v string, folders *scanfolder.Set) (string, error) {
		if v == "" {
			return "", errEmpty
		}
		if _, ok := folders.ByName(v); !ok {
			return "", fmt.Errorf("scan folder %q is not configured", v)
		}
		return v, nil
	},
	model.FilterPath: func(v string, _ *scanfolder.Set) (string, error) {
		if v == "" {
			return "", errEmpty
		}
		if !filepath.IsAbs(v) {
			return "", fmt.Errorf("path %q must be absolute", v)
		}
		return filepath.Clean(v), nil
	},
	model.FilterFormat: func(v string, _ *scanfolder.Set) (string, error) {
		v = strings.ToLower(v)
		if !formatRe.MatchString(v) {
			return "", fmt.Errorf("%q is not a file extension (expected something like flac or mp3)", v)
		}
		return v, nil
	},
	// The empty string is a real value here: it selects albums with no release type.
	model.FilterReleaseType: func(v string, _ *scanfolder.Set) (string, error) { return v, nil },
	model.FilterCompilation: func(v string, _ *scanfolder.Set) (string, error) {
		if v != "true" && v != "false" {
			return "", fmt.Errorf("a compilation filter is true or false, not %q", v)
		}
		return v, nil
	},
	model.FilterGenre: func(v string, _ *scanfolder.Set) (string, error) {
		if v == "" {
			return "", errEmpty
		}
		return v, nil
	},
}

var errEmpty = errors.New("a value must not be empty")

// Dangling reports the scan_folder values that name no configured folder. It is
// what a stored library looks like after its folder was renamed or removed in
// the config file: nothing is wrong with the row, that value just matches
// nothing until the filter is edited.
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
					Detail:  fmt.Sprintf("scan folder %q is not configured any more; this value matches nothing", v),
				})
			}
		}
	}
	return out
}
