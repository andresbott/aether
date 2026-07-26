// internal/tags/taglib.go
package tags

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"go.senan.xyz/taglib"
)

var taglibExtensions = map[string]bool{
	".mp3": true, ".flac": true, ".aac": true, ".m4a": true, ".m4b": true,
	".ogg": true, ".opus": true, ".wma": true, ".wav": true, ".wv": true, ".ape": true, ".aiff": true,
}

type TaglibReader struct{}

func (TaglibReader) CanRead(absPath string) bool {
	return taglibExtensions[strings.ToLower(filepath.Ext(absPath))]
}

func (TaglibReader) Read(absPath string) (Metadata, error) {
	props, err := taglib.ReadProperties(absPath)
	if err != nil {
		return Metadata{}, fmt.Errorf("read properties: %w", err)
	}
	raw, err := taglib.ReadTags(absPath)
	if err != nil {
		return Metadata{}, fmt.Errorf("read tags: %w", err)
	}
	m := Metadata{
		Duration: props.Length,
		Bitrate:  int(props.BitRate),
		HasCover: hasFrontCover(props.Images),
	}
	m.Title = first(raw, "title", "TITLE")
	m.Artist = values(raw, "artist", "ARTIST")
	m.AlbumArtist = values(raw, "albumartist", "ALBUMARTIST", "album_artist", "ALBUM_ARTIST")
	m.Album = first(raw, "album", "ALBUM")
	m.Genre = values(raw, "genre", "GENRE")
	m.Year = parseYear(raw)
	m.TrackNumber = parseInt(first(raw, "tracknumber", "TRACKNUMBER", "track", "TRACK"))
	m.DiscNumber = parseInt(first(raw, "discnumber", "DISCNUMBER", "disc", "DISC"))
	m.DiscSubtitle = first(raw, "discsubtitle", "DISCSUBTITLE", "TSST")
	m.MBRecordingID = first(raw, "musicbrainz_trackid", "MUSICBRAINZ_TRACKID")
	m.MBReleaseID = first(raw, "musicbrainz_albumid", "MUSICBRAINZ_ALBUMID")
	m.MBReleaseGroupID = first(raw, "musicbrainz_releasegroupid", "MUSICBRAINZ_RELEASEGROUPID")
	m.MBArtistID = allOrSingle(raw, "musicbrainz_artistid", "MUSICBRAINZ_ARTISTID", "MusicBrainz Artist Id")
	m.MBAlbumArtistID = allOrSingle(raw, "musicbrainz_albumartistid", "MUSICBRAINZ_ALBUMARTISTID", "MusicBrainz Album Artist Id")
	m.Lyrics = first(raw, "lyrics", "LYRICS", "USLT")
	m.Compilation = parseBool(first(raw, "compilation", "COMPILATION", "TCMP"))
	m.ReleaseType = first(raw, "musicbrainz_albumtype", "MUSICBRAINZ_ALBUMTYPE", "RELEASETYPE")
	m.ReplayGain = ReplayGain{
		TrackGain: parseDBPtr(first(raw, "replaygain_track_gain", "REPLAYGAIN_TRACK_GAIN")),
		TrackPeak: parseFloatPtr(first(raw, "replaygain_track_peak", "REPLAYGAIN_TRACK_PEAK")),
		AlbumGain: parseDBPtr(first(raw, "replaygain_album_gain", "REPLAYGAIN_ALBUM_GAIN")),
		AlbumPeak: parseFloatPtr(first(raw, "replaygain_album_peak", "REPLAYGAIN_ALBUM_PEAK")),
	}
	return m, nil
}

func first(tags map[string][]string, keys ...string) string {
	for _, k := range keys {
		if vs, ok := tags[k]; ok && len(vs) > 0 && vs[0] != "" {
			return vs[0]
		}
	}
	return ""
}

func values(tags map[string][]string, keys ...string) []string {
	for _, k := range keys {
		if vs, ok := tags[k]; ok && len(vs) > 0 {
			return vs
		}
	}
	return nil
}

func parseInt(s string) int {
	s = strings.Split(s, "/")[0]
	i, _ := strconv.Atoi(strings.TrimSpace(s))
	return i
}

func parseBool(s string) bool {
	b, _ := strconv.ParseBool(s)
	return b
}

func parseYear(tags map[string][]string) int {
	for _, key := range []string{"originaldate", "ORIGINALDATE", "date", "DATE", "year", "YEAR"} {
		if v := first(tags, key); v != "" {
			if len(v) >= 4 {
				if y, err := strconv.Atoi(v[:4]); err == nil && y > 0 {
					return y
				}
			}
		}
	}
	return 0
}

func parseDBPtr(s string) *float64 {
	if s == "" {
		return nil
	}
	s = strings.ToLower(s)
	s = strings.TrimSuffix(s, " db")
	s = strings.TrimSuffix(s, "db")
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return nil
	}
	return &f
}

func parseFloatPtr(s string) *float64 {
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return nil
	}
	return &f
}
