// Package assetstore stores entity image files durably under a metadata
// directory, keyed by a stable (kind, key) identity. The manual-vs-auto
// distinction is encoded in the filename: cover.<ext> is a manual upload
// (locked — never overwritten by the fetcher); cover.auto.<ext> is
// auto-fetched. This store holds image files only; no metadata sidecars.
package assetstore

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	KindArtist = "artist"
	KindRadio  = "radio"
	KindAlbum  = "album"
)

type Store struct {
	root string
}

func New(root string) *Store {
	return &Store{root: root}
}

var keyRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func (s *Store) entityDir(kind, key string) (string, error) {
	if !keyRe.MatchString(kind) || !keyRe.MatchString(key) || strings.Contains(key, "..") {
		return "", fmt.Errorf("assetstore: unsafe kind/key %q/%q", kind, key)
	}
	return filepath.Join(s.root, kind, key), nil
}

// Get returns the best image path for the entity, preferring a manual upload
// over an auto-fetched image.
func (s *Store) Get(kind, key string) (string, bool) {
	dir, err := s.entityDir(kind, key)
	if err != nil {
		return "", false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	var manual, auto string
	for _, e := range entries {
		name := e.Name()
		switch {
		case strings.HasPrefix(name, "cover.auto."):
			auto = filepath.Join(dir, name)
		case strings.HasPrefix(name, "cover."):
			manual = filepath.Join(dir, name)
		}
	}
	if manual != "" {
		return manual, true
	}
	if auto != "" {
		return auto, true
	}
	return "", false
}

func (s *Store) PutAuto(kind, key, ext string, data []byte) error {
	return s.put(kind, key, "cover.auto."+normExt(ext), "cover.auto.", data)
}

func (s *Store) PutManual(kind, key, ext string, data []byte) error {
	return s.put(kind, key, "cover."+normExt(ext), "", data)
}

// put writes data to dir/<name> atomically. removePrefix selects which existing
// files to clear first: "cover.auto." clears only auto variants; "" clears only
// manual variants (cover.* that are not cover.auto.*).
func (s *Store) put(kind, key, name, removePrefix string, data []byte) error {
	dir, err := s.entityDir(kind, key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("assetstore: mkdir: %w", err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		n := e.Name()
		isAuto := strings.HasPrefix(n, "cover.auto.")
		if removePrefix == "cover.auto." && isAuto {
			_ = os.Remove(filepath.Join(dir, n))
		}
		if removePrefix == "" && !isAuto && strings.HasPrefix(n, "cover.") {
			_ = os.Remove(filepath.Join(dir, n))
		}
	}
	final := filepath.Join(dir, name)
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

func (s *Store) Delete(kind, key string) error {
	dir, err := s.entityDir(kind, key)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
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
