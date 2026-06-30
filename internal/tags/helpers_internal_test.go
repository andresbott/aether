package tags

import (
	"testing"
	"time"
)

func TestFirst(t *testing.T) {
	m := map[string][]string{
		"TITLE": {"Hello"},
		"EMPTY": {""},
		"MULTI": {"a", "b"},
	}
	if got := first(m, "title", "TITLE"); got != "Hello" {
		t.Errorf("first found: got %q", got)
	}
	if got := first(m, "missing"); got != "" {
		t.Errorf("first missing: got %q", got)
	}
	// First key present but empty -> falls through to the next key.
	if got := first(m, "EMPTY", "TITLE"); got != "Hello" {
		t.Errorf("first skips empty value: got %q", got)
	}
}

func TestValues(t *testing.T) {
	m := map[string][]string{"ARTIST": {"A", "B"}}
	if got := values(m, "artist", "ARTIST"); len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Errorf("values found: got %v", got)
	}
	if got := values(m, "missing"); got != nil {
		t.Errorf("values missing: got %v", got)
	}
}

func TestParseInt(t *testing.T) {
	cases := map[string]int{"5": 5, "3/12": 3, "": 0, "abc": 0, " 7 ": 7}
	for in, want := range cases {
		if got := parseInt(in); got != want {
			t.Errorf("parseInt(%q)=%d want %d", in, got, want)
		}
	}
}

func TestParseBool(t *testing.T) {
	if !parseBool("1") || !parseBool("true") {
		t.Error("expected true for 1/true")
	}
	if parseBool("") || parseBool("0") {
		t.Error("expected false for empty/0")
	}
}

func TestParseYear(t *testing.T) {
	cases := []struct {
		tags map[string][]string
		want int
	}{
		{map[string][]string{"DATE": {"2001-05-04"}}, 2001},
		{map[string][]string{"year": {"1999"}}, 1999},
		{map[string][]string{"ORIGINALDATE": {"1985"}}, 1985},
		{map[string][]string{}, 0},
		{map[string][]string{"DATE": {"19"}}, 0},     // too short
		{map[string][]string{"DATE": {"abcd"}}, 0},   // not numeric
		{map[string][]string{"DATE": {"0000"}}, 0},   // not > 0
	}
	for _, c := range cases {
		if got := parseYear(c.tags); got != c.want {
			t.Errorf("parseYear(%v)=%d want %d", c.tags, got, c.want)
		}
	}
}

func TestParseDBPtr(t *testing.T) {
	if got := parseDBPtr("-6.50 dB"); got == nil || *got != -6.5 {
		t.Errorf("parseDBPtr dB suffix: %v", got)
	}
	if got := parseDBPtr("3db"); got == nil || *got != 3 {
		t.Errorf("parseDBPtr db suffix: %v", got)
	}
	if got := parseDBPtr(""); got != nil {
		t.Errorf("parseDBPtr empty: want nil got %v", got)
	}
	if got := parseDBPtr("xyz"); got != nil {
		t.Errorf("parseDBPtr invalid: want nil got %v", got)
	}
}

func TestParseFloatPtr(t *testing.T) {
	if got := parseFloatPtr("0.98"); got == nil || *got != 0.98 {
		t.Errorf("parseFloatPtr: %v", got)
	}
	if got := parseFloatPtr(""); got != nil {
		t.Errorf("parseFloatPtr empty: want nil got %v", got)
	}
	if got := parseFloatPtr("bad"); got != nil {
		t.Errorf("parseFloatPtr invalid: want nil got %v", got)
	}
}

func TestParseFFProbeJSON(t *testing.T) {
	const sample = `{
		"streams": [{"codec_type": "audio"}, {"codec_type": "video"}],
		"format": {
			"duration": "180.5",
			"bit_rate": "320000",
			"tags": {
				"title": "Song Title",
				"artist": "A; B",
				"album_artist": "Album Artist",
				"album": "The Album",
				"genre": "Rock; Indie",
				"date": "2001-05-04",
				"track": "3/12",
				"disc": "1/2",
				"compilation": "1",
				"replaygain_track_gain": "-6.50 dB",
				"replaygain_track_peak": "0.988",
				"musicbrainz_trackid": "mb-1",
				"musicbrainz_albumid": "mb-2",
				"lyrics": "la la"
			}
		}
	}`
	m, err := parseFFProbeJSON([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "Song Title" {
		t.Errorf("Title = %q", m.Title)
	}
	if m.Album != "The Album" {
		t.Errorf("Album = %q", m.Album)
	}
	if len(m.Artist) != 2 || m.Artist[0] != "A" || m.Artist[1] != "B" {
		t.Errorf("Artist = %v", m.Artist)
	}
	if m.Year != 2001 {
		t.Errorf("Year = %d", m.Year)
	}
	if m.TrackNumber != 3 {
		t.Errorf("TrackNumber = %d", m.TrackNumber)
	}
	if m.DiscNumber != 1 {
		t.Errorf("DiscNumber = %d", m.DiscNumber)
	}
	if !m.HasCover {
		t.Error("expected HasCover true (video stream present)")
	}
	if m.Bitrate != 320 {
		t.Errorf("Bitrate = %d", m.Bitrate)
	}
	if m.Duration != 180*time.Second {
		t.Errorf("Duration = %v", m.Duration)
	}
	if !m.Compilation {
		t.Error("expected Compilation true")
	}
	if m.ReplayGain.TrackGain == nil || *m.ReplayGain.TrackGain != -6.5 {
		t.Errorf("TrackGain = %v", m.ReplayGain.TrackGain)
	}
	if m.ReplayGain.TrackPeak == nil || *m.ReplayGain.TrackPeak != 0.988 {
		t.Errorf("TrackPeak = %v", m.ReplayGain.TrackPeak)
	}

	// Invalid JSON returns an error.
	if _, err := parseFFProbeJSON([]byte("not json")); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestAllOrSingle(t *testing.T) {
	// Multi-value split on null byte.
	m := map[string][]string{"MBID": {"a\x00b\x00c"}}
	got := allOrSingle(m, "MBID")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("null-byte split: %v", got)
	}

	// Multi-value split on semicolon.
	m2 := map[string][]string{"MBID": {"x;y;z"}}
	got2 := allOrSingle(m2, "MBID")
	if len(got2) != 3 || got2[0] != "x" || got2[1] != "y" || got2[2] != "z" {
		t.Errorf("semicolon split: %v", got2)
	}

	// Trimming and skipping empties.
	m3 := map[string][]string{"K": {" a ; ; b "}}
	got3 := allOrSingle(m3, "K")
	if len(got3) != 2 || got3[0] != "a" || got3[1] != "b" {
		t.Errorf("trim/skip empties: %v", got3)
	}

	// Absent key returns nil.
	if got := allOrSingle(m, "MISSING"); got != nil {
		t.Errorf("absent key: want nil got %v", got)
	}

	// First present key wins.
	m4 := map[string][]string{"B": {"second"}}
	got4 := allOrSingle(m4, "A", "B")
	if len(got4) != 1 || got4[0] != "second" {
		t.Errorf("first present key wins: %v", got4)
	}
}

func TestSplitSemicolon(t *testing.T) {
	if got := splitSemicolon("A;B;C"); len(got) != 3 {
		t.Errorf("splitSemicolon basic: %v", got)
	}
	// Trims whitespace and drops empty segments.
	if got := splitSemicolon("A; B ; "); len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Errorf("splitSemicolon trims/skips: %v", got)
	}
	if got := splitSemicolon(""); got != nil {
		t.Errorf("splitSemicolon empty: want nil got %v", got)
	}
}
