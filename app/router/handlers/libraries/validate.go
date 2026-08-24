package libraries

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/andresbott/aether/internal/covergen"
)

// The validators are exported because config-provisioned libraries
// (app/cmd/libraries.go) must be held to exactly the same rules as ones
// created through this API — a config typo should fail as loudly as a bad
// request, and a second copy of these rules would drift.

// valueError marks a validator failure as well-formed-but-invalid: the field
// is present but its value fails a business rule (too long, not a usable
// directory, ...) — as opposed to an outright missing required field. The
// /api/v1 handlers (libraries.go's validateDTO) use this distinction to
// answer 422 instead of 400; app/cmd's config-load path only ever checks
// err != nil, so it is unaffected.
type valueError struct{ msg string }

func (e *valueError) Error() string { return e.msg }

// isValueError reports whether err was constructed as a valueError.
func isValueError(err error) bool {
	var e *valueError
	return errors.As(err, &e)
}

// ValidatePath resolves path to an absolute one and verifies it is an existing,
// readable directory.
func ValidatePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", &valueError{fmt.Sprintf("invalid path: %v", err)}
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", &valueError{fmt.Sprintf("path does not exist: %s", abs)}
		}
		return "", &valueError{fmt.Sprintf("cannot stat path: %v", err)}
	}
	if !info.IsDir() {
		return "", &valueError{fmt.Sprintf("path is not a directory: %s", abs)}
	}
	f, err := os.Open(abs) //nolint:gosec // G304: abs is an admin-configured library root being validated, not an attacker-controlled path
	if err != nil {
		return "", &valueError{fmt.Sprintf("path is not readable: %v", err)}
	}
	_ = f.Close()
	return abs, nil
}

// ValidateExcludePatterns verifies every pattern compiles as a Go regexp.
func ValidateExcludePatterns(patterns []string) error {
	for _, p := range patterns {
		if _, err := regexp.Compile(p); err != nil {
			return fmt.Errorf("invalid regex %q: %w", p, err)
		}
	}
	return nil
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

// ValidateCoverStyle verifies v is "auto"/"" or a known covergen style.
func ValidateCoverStyle(v string) error {
	if v == "" || v == "auto" {
		return nil
	}
	if _, ok := covergen.ParseStyle(v); !ok {
		names := make([]string, 0, len(covergen.Styles()))
		for _, s := range covergen.Styles() {
			names = append(names, s.String())
		}
		return fmt.Errorf("invalid cover_style: %q (allowed: auto, %s)", v, strings.Join(names, ", "))
	}
	return nil
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
