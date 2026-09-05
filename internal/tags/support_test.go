package tags

import "testing"

// The single source of truth for which formats Aether indexes and edits.
// Membership is policy; a reader's CanRead is capability. Policy must be a
// subset of capability (something we support must be readable), but capability
// may be wider (a reader can parse formats we choose not to support).
var wantSupported = []string{
	".mp3", ".flac", ".ogg", ".oga", ".opus",
	".m4a", ".m4b", ".mp4", ".wav", ".aiff",
	".wma", ".aac",
}

func TestSupported(t *testing.T) {
	for _, ext := range wantSupported {
		if !Supported("song" + ext) {
			t.Errorf("Supported(%q) = false, want true", ext)
		}
	}
	for _, ext := range []string{".mpc", ".tak", ".spx", ".w64", ".rf64", ".txt", ".jpg", ""} {
		if Supported("file" + ext) {
			t.Errorf("Supported(%q) = true, want false", ext)
		}
	}
}

func TestSupportedIsCaseInsensitive(t *testing.T) {
	if !Supported("SONG.FLAC") {
		t.Error("Supported(\"SONG.FLAC\") = false, want true")
	}
}

// TestSupportedIsReadable is the drift backstop: every supported format must be
// readable by taglib or ffprobe, or the scanner would admit a file no reader
// can parse. Fails if the supported set grows past reader capability.
func TestSupportedIsReadable(t *testing.T) {
	fb := NewFallbackReader(TaglibReader{}, FFProbeReader{})
	for _, ext := range wantSupported {
		if !fb.CanRead("song" + ext) {
			t.Errorf("%s is a supported format but no reader can read it", ext)
		}
	}
}
