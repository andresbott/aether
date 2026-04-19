// internal/tags/ffprobe.go
package tags

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var ffprobeExtensions = map[string]bool{
	".mp3": true, ".flac": true, ".ogg": true, ".opus": true, ".m4a": true, ".m4b": true,
	".aac": true, ".wma": true, ".wav": true, ".wv": true, ".ape": true, ".aiff": true,
	".mka": true, ".mpc": true, ".oga": true, ".tak": true, ".tta": true, ".dsf": true,
	".webm": true, ".spx": true, ".w64": true, ".rf64": true,
}

type FFProbeReader struct{}

func (FFProbeReader) CanRead(absPath string) bool {
	return ffprobeExtensions[strings.ToLower(filepath.Ext(absPath))]
}

func (FFProbeReader) Read(absPath string) (Metadata, error) {
	out, err := exec.Command(
		"ffprobe", "-hide_banner", "-v", "0", "-i", absPath,
		"-show_entries", "format:stream=codec_type", "-of", "json",
	).Output()
	if err != nil {
		return Metadata{}, fmt.Errorf("ffprobe exec: %w", err)
	}

	var d struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
		} `json:"streams"`
		Format struct {
			Duration string            `json:"duration"`
			BitRate  string            `json:"bit_rate"`
			Tags     map[string]string `json:"tags"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &d); err != nil {
		return Metadata{}, fmt.Errorf("ffprobe json: %w", err)
	}

	durSecs, _ := strconv.ParseFloat(d.Format.Duration, 64)
	bitrate, _ := strconv.Atoi(d.Format.BitRate)

	tags := make(map[string][]string, len(d.Format.Tags))
	for k, v := range d.Format.Tags {
		tags[k] = []string{v}
	}

	var hasCover bool
	for _, s := range d.Streams {
		if s.CodecType == "video" {
			hasCover = true
			break
		}
	}

	m := Metadata{
		Duration: time.Duration(durSecs) * time.Second,
		Bitrate:  bitrate / 1000,
		HasCover: hasCover,
	}
	m.Title = first(tags, "title", "TITLE")
	m.Artist = splitSemicolon(first(tags, "artist", "ARTIST"))
	m.AlbumArtist = splitSemicolon(first(tags, "album_artist", "ALBUMARTIST"))
	m.Album = first(tags, "album", "ALBUM")
	m.Genre = splitSemicolon(first(tags, "genre", "GENRE"))
	m.Year = parseYear(tags)
	m.TrackNumber = parseInt(first(tags, "track", "TRACK"))
	m.DiscNumber = parseInt(first(tags, "disc", "DISC"))
	m.DiscSubtitle = first(tags, "TSST", "discsubtitle")
	m.MBRecordingID = first(tags, "musicbrainz_trackid", "MUSICBRAINZ_TRACKID", "MusicBrainz Release Track Id")
	m.MBReleaseID = first(tags, "musicbrainz_albumid", "MUSICBRAINZ_ALBUMID", "MusicBrainz Album Id")
	m.Lyrics = first(tags, "lyrics", "LYRICS")
	m.Compilation = parseBool(first(tags, "compilation", "COMPILATION"))
	m.ReleaseType = first(tags, "MUSICBRAINZ_ALBUMTYPE", "musicbrainz_albumtype")
	m.ReplayGain = ReplayGain{
		TrackGain: parseDBPtr(first(tags, "replaygain_track_gain", "REPLAYGAIN_TRACK_GAIN")),
		TrackPeak: parseFloatPtr(first(tags, "replaygain_track_peak", "REPLAYGAIN_TRACK_PEAK")),
		AlbumGain: parseDBPtr(first(tags, "replaygain_album_gain", "REPLAYGAIN_ALBUM_GAIN")),
		AlbumPeak: parseFloatPtr(first(tags, "replaygain_album_peak", "REPLAYGAIN_ALBUM_PEAK")),
	}
	return m, nil
}

func splitSemicolon(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
