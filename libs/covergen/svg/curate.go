package svg

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Validate reports whether src satisfies the svg asset contract: parseable,
// viewBox="0 0 100 100", uses at least one sentinel colour, and rasterizes to a
// motif that isn't a full-canvas fill (rejected only when all four corners are
// opaque). A motif may legitimately bleed to one or more edges — e.g. a
// horizon/wave shape anchored to the bottom — as long as it doesn't cover the
// entire canvas.
func Validate(src []byte) error {
	if !bytes.Contains(src, []byte(`viewBox="0 0 100 100"`)) {
		return errors.New(`svg: missing viewBox="0 0 100 100"`)
	}
	if !hasSentinel(src) {
		return errors.New("svg: uses no sentinel colours")
	}
	img, err := rasterizeMotif(src, 64, 64)
	if err != nil {
		return fmt.Errorf("svg: rasterize: %w", err)
	}
	corners := [][2]int{{0, 0}, {63, 0}, {0, 63}, {63, 63}}
	allOpaque := true
	for _, c := range corners {
		if img.RGBAAt(c[0], c[1]).A == 0 {
			allOpaque = false
			break
		}
	}
	if allOpaque {
		return errors.New("svg: background not transparent (all four corners opaque — looks like a full-canvas fill)")
	}
	return nil
}

// safeName returns the basename of file, rejecting anything with a path
// separator, traversal, or a non-.svg extension.
func safeName(file string) (string, error) {
	name := filepath.Base(file)
	if name != file || name == "." || name == ".." || !strings.HasSuffix(name, ".svg") {
		return "", fmt.Errorf("svg: invalid file name %q", file)
	}
	return name, nil
}

// Promote validates candidatesDir/file and, if it passes, copies it into
// assetsDir and removes it from candidatesDir.
func Promote(candidatesDir, assetsDir, file string) error {
	name, err := safeName(file)
	if err != nil {
		return err
	}
	src, err := os.ReadFile(filepath.Join(candidatesDir, name)) //nolint:gosec // G304: name is validated by safeName (rejects separators/traversal/non-.svg), never a raw request path
	if err != nil {
		return err
	}
	if err := Validate(src); err != nil {
		return err
	}
	if err := os.MkdirAll(assetsDir, 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(assetsDir, name), src, 0o600); err != nil { //nolint:gosec // G703: name is validated by safeName (rejects separators/traversal/non-.svg), never a raw request path
		return err
	}
	return os.Remove(filepath.Join(candidatesDir, name))
}

// Reject deletes candidatesDir/file.
func Reject(candidatesDir, file string) error {
	name, err := safeName(file)
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(candidatesDir, name))
}

// Demote moves assetsDir/file back into candidatesDir (no validation).
func Demote(assetsDir, candidatesDir, file string) error {
	name, err := safeName(file)
	if err != nil {
		return err
	}
	src, err := os.ReadFile(filepath.Join(assetsDir, name)) //nolint:gosec // G304: name is validated by safeName (rejects separators/traversal/non-.svg), never a raw request path
	if err != nil {
		return err
	}
	if err := os.MkdirAll(candidatesDir, 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(candidatesDir, name), src, 0o600); err != nil { //nolint:gosec // G703: name is validated by safeName (rejects separators/traversal/non-.svg), never a raw request path
		return err
	}
	return os.Remove(filepath.Join(assetsDir, name))
}
