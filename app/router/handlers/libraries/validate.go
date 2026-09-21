package libraries

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Validators for the fields the libraries API accepts on create/update: name,
// default_view and icon.

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

// ValidateDefaultView verifies v is an allowed default view ("" = albums).
func ValidateDefaultView(v string) error {
	switch v {
	case "", "albums", "artists":
		return nil
	default:
		return fmt.Errorf("invalid default_view: %q (allowed: albums, artists)", v)
	}
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
