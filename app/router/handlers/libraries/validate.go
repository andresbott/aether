package libraries

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	f, err := os.Open(abs)
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

func validateMultiValue(s string) error {
	s = strings.TrimSpace(s)
	switch {
	case s == "" || s == "none" || s == "multi":
		return nil
	case strings.HasPrefix(s, "delim "):
		if strings.TrimPrefix(s, "delim ") == "" {
			return fmt.Errorf("delim mode requires a separator")
		}
		return nil
	default:
		return fmt.Errorf("invalid multi-value grammar: %q", s)
	}
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
