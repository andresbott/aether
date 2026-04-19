// internal/scanner/config.go
package scanner

import (
	"strings"
)

type MusicPath struct {
	Alias string
	Path  string
}

func ParseMusicPath(s string) MusicPath {
	s = strings.TrimSpace(s)
	if parts := strings.SplitN(s, "->", 2); len(parts) == 2 {
		return MusicPath{
			Alias: strings.TrimSpace(parts[0]),
			Path:  strings.TrimSpace(parts[1]),
		}
	}
	return MusicPath{Path: s}
}

type MultiValueMode int

const (
	MVNone  MultiValueMode = iota
	MVMulti
	MVDelim
)

func ParseMultiValueMode(s string) (MultiValueMode, string) {
	s = strings.TrimSpace(s)
	switch {
	case s == "" || s == "none":
		return MVNone, ""
	case s == "multi":
		return MVMulti, ""
	case strings.HasPrefix(s, "delim "):
		return MVDelim, strings.TrimPrefix(s, "delim ")
	default:
		return MVNone, ""
	}
}

func ApplyMultiValue(mode MultiValueMode, delim string, single string, multi []string) []string {
	switch mode {
	case MVMulti:
		if len(multi) > 0 {
			return multi
		}
		if single != "" {
			return []string{single}
		}
		return nil
	case MVDelim:
		if single == "" {
			return nil
		}
		parts := strings.Split(single, delim)
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	default:
		if single == "" {
			return nil
		}
		return []string{single}
	}
}

type MultiValueConfig struct {
	GenreMode        MultiValueMode
	GenreDelim       string
	ArtistMode       MultiValueMode
	ArtistDelim      string
	AlbumArtistMode  MultiValueMode
	AlbumArtistDelim string
}

type Config struct {
	MusicPaths      []MusicPath
	ExcludePatterns []string
	TagReadWorkers  int
	FollowSymlinks  bool
	MultiValue      MultiValueConfig
}
