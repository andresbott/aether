// Package assetstore stores entity image files durably under a metadata
// directory, keyed by a stable (kind, key) identity plus an entry name (the
// default is "cover"; album entities also hold e.g. "back" or "booklet").
// The manual-vs-auto distinction is encoded in the filename: <name>.<ext> is
// a manual upload (locked — never overwritten by the fetcher);
// <name>.auto.<ext> is auto-fetched. This store holds image files only; no
// metadata sidecars.
package assetstore

import (
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

// Entry names are embedded in filenames as <name>.<ext> / <name>.auto.<ext>,
// so unlike keys they must not contain dots.
var nameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func (s *Store) entityDir(kind, key string) (string, error) {
	if !keyRe.MatchString(kind) || !keyRe.MatchString(key) || strings.Contains(key, "..") {
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

// Get returns the best primary ("cover") image path for the entity,
// preferring a manual upload over an auto-fetched image.
func (s *Store) Get(kind, key string) (string, bool) {
	return s.GetNamed(kind, key, DefaultName)
}

// GetNamed returns the best image path for one named entry of the entity,
// preferring a manual upload over an auto-fetched image.
func (s *Store) GetNamed(kind, key, name string) (string, bool) {
	path, _, ok := s.GetEntryNamed(kind, key, name)
	return path, ok
}

// GetEntry is Get plus whether the winning image is a manual upload rather than
// an auto-fetched one. Callers that show the image to a user need the
// distinction; the path alone does not carry it without re-parsing the filename.
func (s *Store) GetEntry(kind, key string) (path string, manual bool, ok bool) {
	return s.GetEntryNamed(kind, key, DefaultName)
}

// GetEntryNamed is GetNamed plus the manual-vs-auto flag.
func (s *Store) GetEntryNamed(kind, key, name string) (path string, manual bool, ok bool) {
	dir, err := s.entityDir(kind, key)
	if err != nil || !nameRe.MatchString(name) {
		return "", false, false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false, false
	}
	var manualPath, autoPath string
	for _, e := range entries {
		base, isAuto, ok := splitEntry(e.Name())
		if !ok || base != name {
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
	return s.put(kind, key, DefaultName, true, ext, data)
}

func (s *Store) PutManual(kind, key, ext string, data []byte) error {
	return s.put(kind, key, DefaultName, false, ext, data)
}

// PutManualNamed stores a manual upload under a named entry of the entity.
func (s *Store) PutManualNamed(kind, key, name, ext string, data []byte) error {
	return s.put(kind, key, name, false, ext, data)
}

// put writes data to the entry's file atomically, clearing only the same
// entry's variants of the same kind first (auto clears auto, manual clears
// manual) so a manual upload never destroys the auto fallback and vice versa.
func (s *Store) put(kind, key, name string, auto bool, ext string, data []byte) error {
	dir, err := s.entityDir(kind, key)
	if err != nil {
		return err
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("assetstore: unsafe entry name %q", name)
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("assetstore: mkdir: %w", err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if base, isAuto, ok := splitEntry(e.Name()); ok && base == name && isAuto == auto {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	filename := name + "." + normExt(ext)
	if auto {
		filename = name + ".auto." + normExt(ext)
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

// DeleteNamed removes one named entry (both manual and auto variants),
// leaving the entity's other entries in place.
func (s *Store) DeleteNamed(kind, key, name string) error {
	dir, err := s.entityDir(kind, key)
	if err != nil {
		return err
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("assetstore: unsafe entry name %q", name)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("assetstore: read dir: %w", err)
	}
	for _, e := range entries {
		if base, _, ok := splitEntry(e.Name()); ok && base == name {
			if rerr := os.Remove(filepath.Join(dir, e.Name())); rerr != nil {
				return fmt.Errorf("assetstore: remove: %w", rerr)
			}
		}
	}
	return nil
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
