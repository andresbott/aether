// Package assetstore stores entity image files durably under a metadata
// directory, keyed by a stable (kind, key) identity. Each entity holds a single
// image, "cover". The manual-vs-auto distinction is encoded in the filename:
// cover.<ext> is a manual upload (locked — never overwritten by the fetcher);
// cover.auto.<ext> is auto-fetched. This store holds image files only; no
// metadata sidecars.
package assetstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	KindArtist   = "artist"
	KindRadio    = "radio"
	KindAlbum    = "album"
	KindPlaylist = "playlist"
	KindGenre    = "genre"
)

type Store struct {
	root string
}

func New(root string) *Store {
	return &Store{root: root}
}

var keyRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// KeySafe reports whether s is safe as an entity key: non-empty, matches
// ^[A-Za-z0-9._-]+$, contains no "..", unchanged by filepath.Clean, and not
// exactly ".". This is the single predicate assetkey delegates to; the
// constraint is load-bearing because entityDir uses the key as a directory
// component under the store root, so a traversal or path-equivalence escape
// would break containment.
func KeySafe(s string) bool {
	return s != "" && keyRe.MatchString(s) && !strings.Contains(s, "..") && filepath.Clean(s) == s && s != "."
}

func (s *Store) entityDir(kind, key string) (string, error) {
	if !KeySafe(kind) || !KeySafe(key) {
		return "", fmt.Errorf("assetstore: unsafe kind/key %q/%q", kind, key)
	}
	return filepath.Join(s.root, kind, key), nil
}

// DefaultName is the entry name of an entity's primary image.
const DefaultName = "cover"

// splitEntry parses a stored filename into its entry name and whether it is
// the auto-fetched variant. Filenames are <name>.<ext> or <name>.auto.<ext>;
// anything else (e.g. temp files) returns ok=false.
func splitEntry(filename string) (name string, auto bool, ok bool) {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	if base == filename || base == "" {
		return "", false, false
	}
	if rest, found := strings.CutSuffix(base, ".auto"); found {
		return rest, true, rest != ""
	}
	if strings.Contains(base, ".") {
		return "", false, false
	}
	return base, false, true
}

// Get returns the best image path for the entity, preferring a manual upload
// over an auto-fetched image.
func (s *Store) Get(kind, key string) (string, bool) {
	path, _, ok := s.GetEntry(kind, key)
	return path, ok
}

// GetEntry is Get plus whether the winning image is a manual upload rather than
// an auto-fetched one. Callers that show the image to a user need the
// distinction; the path alone does not carry it without re-parsing the filename.
func (s *Store) GetEntry(kind, key string) (path string, manual bool, ok bool) {
	dir, err := s.entityDir(kind, key)
	if err != nil {
		return "", false, false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false, false
	}
	var manualPath, autoPath string
	for _, e := range entries {
		base, isAuto, ok := splitEntry(e.Name())
		if !ok || base != DefaultName {
			continue
		}
		if isAuto {
			autoPath = filepath.Join(dir, e.Name())
		} else {
			manualPath = filepath.Join(dir, e.Name())
		}
	}
	if manualPath != "" {
		return manualPath, true, true
	}
	if autoPath != "" {
		return autoPath, false, true
	}
	return "", false, false
}

func (s *Store) PutAuto(kind, key, ext string, data []byte) error {
	return s.put(kind, key, true, ext, data)
}

func (s *Store) PutManual(kind, key, ext string, data []byte) error {
	return s.put(kind, key, false, ext, data)
}

// put writes data to the cover file atomically, clearing only the same variant
// first (auto clears auto, manual clears manual) so a manual upload never
// destroys the auto fallback and vice versa.
func (s *Store) put(kind, key string, auto bool, ext string, data []byte) error {
	dir, err := s.entityDir(kind, key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("assetstore: mkdir: %w", err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if base, isAuto, ok := splitEntry(e.Name()); ok && base == DefaultName && isAuto == auto {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	filename := DefaultName + "." + normExt(ext)
	if auto {
		filename = DefaultName + ".auto." + normExt(ext)
	}
	final := filepath.Join(dir, filename)
	tmp, err := os.CreateTemp(dir, "asset-*.tmp")
	if err != nil {
		return fmt.Errorf("assetstore: temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("assetstore: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("assetstore: close: %w", err)
	}
	if err := os.Rename(tmpName, final); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("assetstore: rename: %w", err)
	}
	return nil
}

// Delete removes the entity's whole directory, every named entry included.
func (s *Store) Delete(kind, key string) error {
	dir, err := s.entityDir(kind, key)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

// ErrKeyOccupied reports that a Rekey destination already holds images. The
// move is refused rather than completed: an orphaned directory is recoverable,
// a destroyed upload is not.
var ErrKeyOccupied = errors.New("assetstore: destination key already holds images")

// Rekey moves an entity's whole directory from oldKey to newKey, carrying every
// named entry and both the manual and auto variants. It exists because an
// entity's key changes whenever its natural identity does, and the image must
// follow it wherever that continuity is provable (see the re-key hooks in
// internal/scanner and the radio stream-URL edit).
//
// It is a directory rename, which is why it is both cheaper and less lossy than
// reading the primary image and re-Putting it — that approach silently drops
// named entries and the auto variant.
//
// Missing source, or oldKey == newKey: no-op, nil. Destination holding images:
// ErrKeyOccupied, nothing moved.
func (s *Store) Rekey(kind, oldKey, newKey string) error {
	if oldKey == newKey {
		return nil
	}
	oldDir, err := s.entityDir(kind, oldKey)
	if err != nil {
		return err
	}
	newDir, err := s.entityDir(kind, newKey)
	if err != nil {
		return err
	}
	if _, err := os.Stat(oldDir); err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to move
		}
		return err
	}
	// An existing-but-empty destination is not an upload worth protecting;
	// remove it so the rename can proceed.
	if entries, rerr := os.ReadDir(newDir); rerr == nil {
		if len(entries) > 0 {
			return fmt.Errorf("%w: %s/%s", ErrKeyOccupied, kind, newKey)
		}
		if err := os.Remove(newDir); err != nil {
			return err
		}
	} else if !os.IsNotExist(rerr) {
		return rerr
	}
	if err := os.MkdirAll(filepath.Dir(newDir), 0o750); err != nil {
		return err
	}
	return os.Rename(oldDir, newDir)
}

func normExt(ext string) string {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	if ext == "jpeg" {
		ext = "jpg"
	}
	if ext != "jpg" && ext != "png" {
		ext = "jpg"
	}
	return ext
}
