package libraries

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/andresbott/aether/internal/covergen"
)

func validatePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path does not exist: %s", abs)
		}
		return "", fmt.Errorf("cannot stat path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", abs)
	}
	f, err := os.Open(abs) //nolint:gosec // G304: abs is an admin-configured library root being validated, not an attacker-controlled path
	if err != nil {
		return "", fmt.Errorf("path is not readable: %w", err)
	}
	_ = f.Close()
	return abs, nil
}

func validateExcludePatterns(patterns []string) error {
	for _, p := range patterns {
		if _, err := regexp.Compile(p); err != nil {
			return fmt.Errorf("invalid regex %q: %w", p, err)
		}
	}
	return nil
}

func validateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if len(name) > 200 {
		return fmt.Errorf("name too long (max 200 chars)")
	}
	return nil
}

func validateDefaultView(v string) error {
	switch v {
	case "", "albums", "artists":
		return nil
	default:
		return fmt.Errorf("invalid default_view: %q (allowed: albums, artists)", v)
	}
}

func validateCoverStyle(v string) error {
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

func validateIcon(v string) error {
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
