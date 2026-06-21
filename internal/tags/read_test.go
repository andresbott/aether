package tags_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/tags"
	"go.senan.xyz/taglib"
)

const flacFixture = "testdata/empty.flac"

// writeTaggedFLAC copies the empty fixture to a temp file and writes a rich set
// of tags so the readers exercise every metadata field.
func writeTaggedFLAC(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat(flacFixture); err != nil {
		t.Skipf("no fixture at %s: %v", flacFixture, err)
	}
	data, err := os.ReadFile(flacFixture)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "tagged.flac")
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatal(err)
	}
	tagMap := map[string][]string{
		"TITLE":                 {"Song Title"},
		"ARTIST":                {"Artist One", "Artist Two"},
		"ALBUMARTIST":           {"Album Artist"},
		"ALBUM":                 {"The Album"},
		"GENRE":                 {"Rock", "Indie"},
		"DATE":                  {"2001-05-04"},
		"TRACKNUMBER":           {"3/12"},
		"DISCNUMBER":            {"1/2"},
		"DISCSUBTITLE":          {"Bonus"},
		"MUSICBRAINZ_TRACKID":   {"mb-track-1"},
		"MUSICBRAINZ_ALBUMID":   {"mb-album-1"},
		"LYRICS":                {"la la"},
		"COMPILATION":           {"1"},
		"MUSICBRAINZ_ALBUMTYPE": {"album"},
		"REPLAYGAIN_TRACK_GAIN": {"-6.50 dB"},
		"REPLAYGAIN_TRACK_PEAK": {"0.988"},
		"REPLAYGAIN_ALBUM_GAIN": {"-7.10 dB"},
		"REPLAYGAIN_ALBUM_PEAK": {"0.991"},
	}
	if err := taglib.WriteTags(dst, tagMap, 0); err != nil {
		t.Fatalf("WriteTags: %v", err)
	}
	return dst
}

func TestTaglibReader_CanRead(t *testing.T) {
	r := tags.TaglibReader{}
	if !r.CanRead("song.flac") || !r.CanRead("a.MP3") {
		t.Error("expected CanRead true for audio extensions")
	}
	if r.CanRead("note.txt") {
		t.Error("expected CanRead false for .txt")
	}
}

func TestFFProbeReader_CanRead(t *testing.T) {
	r := tags.FFProbeReader{}
	if !r.CanRead("song.flac") || !r.CanRead("clip.webm") {
		t.Error("expected CanRead true for audio extensions")
	}
	if r.CanRead("note.txt") {
		t.Error("expected CanRead false for .txt")
	}
}

func TestTaglibReader_Read(t *testing.T) {
	dst := writeTaggedFLAC(t)
	m, err := tags.TaglibReader{}.Read(dst)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if m.Title != "Song Title" {
		t.Errorf("Title = %q", m.Title)
	}
	if m.Album != "The Album" {
		t.Errorf("Album = %q", m.Album)
	}
	if len(m.Artist) != 2 {
		t.Errorf("Artist = %v, want 2 values", m.Artist)
	}
	if len(m.AlbumArtist) == 0 || m.AlbumArtist[0] != "Album Artist" {
		t.Errorf("AlbumArtist = %v", m.AlbumArtist)
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
	if !m.Compilation {
		t.Error("Compilation = false, want true")
	}
}

func TestTaglibReader_ReadMissingFile(t *testing.T) {
	_, err := tags.TaglibReader{}.Read(filepath.Join(t.TempDir(), "nope.flac"))
	if err == nil {
		t.Fatal("expected error reading missing file")
	}
}

func TestFFProbeReader_Read(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	dst := writeTaggedFLAC(t)
	m, err := tags.FFProbeReader{}.Read(dst)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if m.Title != "Song Title" {
		t.Errorf("Title = %q", m.Title)
	}
	if m.Album != "The Album" {
		t.Errorf("Album = %q", m.Album)
	}
}

func TestFFProbeReader_ReadMissingFile(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	_, err := tags.FFProbeReader{}.Read(filepath.Join(t.TempDir(), "nope.flac"))
	if err == nil {
		t.Fatal("expected error on missing file")
	}
}
