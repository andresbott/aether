package libraries

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/andresbott/aether/internal/model"
	"github.com/go-bumbu/http/problemjson"
)

// Validators for the fields the libraries API accepts on create/update: name,
// views, default_view and icon.

// valueError marks a validator failure as well-formed-but-invalid: the field
// is present but its value fails a business rule (too long, an unknown enum,
// ...) — as opposed to an outright missing required field. The /api/v0
// handler (libraries.go's validateDTO) uses this distinction to answer 422
// instead of 400.
type valueError struct{ msg string }

func (e *valueError) Error() string { return e.msg }

// isValueError reports whether err was constructed as a valueError.
func isValueError(err error) bool {
	var e *valueError
	return errors.As(err, &e)
}

// ValidateName verifies the library name is present and of sane length.
func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if len(name) > 200 {
		return &valueError{"name too long (max 200 chars)"}
	}
	return nil
}

// ValidateViews checks a library's views and returns them normalized: each
// view once, in model.LibraryViews order. It reports every problem, each at a
// pointer into the request's views array.
func ValidateViews(views []model.LibraryView) ([]model.LibraryView, []problemjson.FieldError) {
	if len(views) == 0 {
		return nil, []problemjson.FieldError{{Pointer: "/views", Detail: "a library needs at least one view"}}
	}
	all := model.LibraryViews()
	var problems []problemjson.FieldError
	for i, v := range views {
		if !slices.Contains(all, v) {
			problems = append(problems, problemjson.FieldError{
				Pointer: "/views/" + strconv.Itoa(i),
				Detail:  fmt.Sprintf("unknown view %q (allowed: discover, artists, releases)", v),
			})
		}
	}
	if len(problems) > 0 {
		return nil, problems
	}
	out := make([]model.LibraryView, 0, len(all))
	for _, v := range all {
		if slices.Contains(views, v) {
			out = append(out, v)
		}
	}
	return out, nil
}

// ValidateDefaultView verifies v is one of the library's views ("" = the first
// of them).
func ValidateDefaultView(v model.LibraryView, views []model.LibraryView) error {
	if v == "" || slices.Contains(views, v) {
		return nil
	}
	return fmt.Errorf("default_view %q is not one of the library's views", v)
}

// iconNameRe matches PrimeIcons names without the "pi pi-" prefix, e.g. "folder-open".
var iconNameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateIcon verifies v is a PrimeIcons name without the "pi pi-" prefix.
func ValidateIcon(v string) error {
	if v == "" {
		return nil
	}
	if len(v) > 100 {
		return fmt.Errorf("icon name too long (max 100 chars)")
	}
	if !iconNameRe.MatchString(v) {
		return fmt.Errorf("invalid icon: %q (expected a PrimeIcons name like \"folder\" or \"folder-open\")", v)
	}
	return nil
}
