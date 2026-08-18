package assetkey_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/model"
)

var keySafetyRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func isKeySafe(key string) bool {
	return keySafetyRe.MatchString(key) && !strings.Contains(key, "..")
}

// TestAlbumStability verifies the same album identity yields the same key across calls.
func TestAlbumStability(t *testing.T) {
	key1 := assetkey.Album("rumours", "fleetwood mac", "")
	key2 := assetkey.Album("rumours", "fleetwood mac", "")
	if key1 != key2 {
		t.Errorf("Album keys differ: %q vs %q", key1, key2)
	}
	if len(key1) != 64 {
		t.Errorf("Album key length = %d, want 64 (hex SHA-256)", len(key1))
	}
	if key1 != strings.ToLower(key1) {
		t.Errorf("Album key not lowercase: %q", key1)
	}
}

// TestAlbumDistinctness verifies differing in any identity component yields different keys.
func TestAlbumDistinctness(t *testing.T) {
	base := assetkey.Album("rumours", "fleetwood mac", "")
	diffName := assetkey.Album("tusk", "fleetwood mac", "")
	diffArtist := assetkey.Album("rumours", "various artists", "")
	diffMBID := assetkey.Album("rumours", "fleetwood mac", "mbid-123")

	if base == diffName {
		t.Errorf("Album keys equal despite different name")
	}
	if base == diffArtist {
		t.Errorf("Album keys equal despite different artist")
	}
	if base == diffMBID {
		t.Errorf("Album keys equal despite different MBID")
	}
}

// TestAlbumOf verifies the helper extracts the correct fields.
func TestAlbumOf(t *testing.T) {
	a := &model.Album{
		NameNorm:        "rumours",
		AlbumArtistNorm: "fleetwood mac",
		MBReleaseID:     "mbid-123",
	}
	want := assetkey.Album("rumours", "fleetwood mac", "mbid-123")
	got := assetkey.AlbumOf(a)
	if got != want {
		t.Errorf("AlbumOf = %q, want %q", got, want)
	}
}

// TestArtistStabilityWithMBID verifies a matched artist returns the MBID verbatim.
func TestArtistStabilityWithMBID(t *testing.T) {
	key1 := assetkey.Artist("mbid-789", "apocalyptica")
	key2 := assetkey.Artist("mbid-789", "apocalyptica")
	if key1 != key2 {
		t.Errorf("Artist keys differ: %q vs %q", key1, key2)
	}
	if key1 != "mbid-789" {
		t.Errorf("Artist key = %q, want verbatim MBID %q", key1, "mbid-789")
	}
}

// TestArtistStabilityWithoutMBID verifies an unmatched artist is hashed.
func TestArtistStabilityWithoutMBID(t *testing.T) {
	key1 := assetkey.Artist("", "apocalyptica")
	key2 := assetkey.Artist("", "apocalyptica")
	if key1 != key2 {
		t.Errorf("Artist keys differ: %q vs %q", key1, key2)
	}
	if len(key1) != 64 {
		t.Errorf("Artist key length = %d, want 64 (hex SHA-256)", len(key1))
	}
	if key1 != strings.ToLower(key1) {
		t.Errorf("Artist key not lowercase: %q", key1)
	}
}

// TestArtistFallback verifies MBID vs no-MBID yields different keys.
func TestArtistFallback(t *testing.T) {
	withMBID := assetkey.Artist("mbid-1", "apocalyptica")
	withoutMBID := assetkey.Artist("", "apocalyptica")
	if withMBID == withoutMBID {
		t.Errorf("Artist keys equal despite MBID vs no MBID")
	}
	if withMBID != "mbid-1" {
		t.Errorf("Artist with MBID = %q, want %q", withMBID, "mbid-1")
	}
}

// TestArtistMBIDValidation verifies a hostile MBID falls back to hash for safety.
func TestArtistMBIDValidation(t *testing.T) {
	cases := []struct {
		name  string
		mbid  string
		valid bool
	}{
		{"normal MBID", "5b11f4ce-a62d-471e-81fc-a69a8278c7da", true},
		{"slash in MBID", "../../etc/passwd", false},
		{"contains slash", "foo/bar", false},
		{"dotdot in MBID", "foo..bar", false},
		{"special chars", "Rock & Roll", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := assetkey.Artist(tc.mbid, "test-artist")
			if !isKeySafe(key) {
				t.Errorf("Artist(%q, ...) = %q, not key-safe", tc.mbid, key)
			}
			if tc.valid {
				if key != tc.mbid {
					t.Errorf("Artist(%q, ...) = %q, want verbatim MBID", tc.mbid, key)
				}
			} else {
				if key == tc.mbid {
					t.Errorf("Artist(%q, ...) = %q, hostile MBID passed through", tc.mbid, key)
				}
				if len(key) != 64 {
					t.Errorf("Artist(%q, ...) = %q, want hashed (64-char hex)", tc.mbid, key)
				}
			}
		})
	}
}

// TestArtistOf verifies the helper extracts the correct fields.
func TestArtistOf(t *testing.T) {
	a := &model.Artist{
		MBArtistID: "mbid-789",
		NameNorm:   "apocalyptica",
	}
	want := assetkey.Artist("mbid-789", "apocalyptica")
	got := assetkey.ArtistOf(a)
	if got != want {
		t.Errorf("ArtistOf = %q, want %q", got, want)
	}
}

// TestGenreStability verifies the same genre name yields the same key across calls.
func TestGenreStability(t *testing.T) {
	key1 := assetkey.Genre("Rock")
	key2 := assetkey.Genre("Rock")
	if key1 != key2 {
		t.Errorf("Genre keys differ: %q vs %q", key1, key2)
	}
	if len(key1) != 64 {
		t.Errorf("Genre key length = %d, want 64 (hex SHA-256)", len(key1))
	}
	if key1 != strings.ToLower(key1) {
		t.Errorf("Genre key not lowercase: %q", key1)
	}
}

// TestGenreCaseSensitivity verifies Genre("Rock") and Genre("rock") differ.
func TestGenreCaseSensitivity(t *testing.T) {
	upper := assetkey.Genre("Rock")
	lower := assetkey.Genre("rock")
	if upper == lower {
		t.Errorf("Genre keys equal despite case difference")
	}
}

// TestGenreKeySafety verifies hostile genre names are hashed safely.
func TestGenreKeySafety(t *testing.T) {
	hostile := "Rock & Roll / Blues .. \n"
	key := assetkey.Genre(hostile)
	if !isKeySafe(key) {
		t.Errorf("Genre(%q) = %q, not key-safe", hostile, key)
	}
}

// TestGenreOf verifies the helper extracts the correct field.
func TestGenreOf(t *testing.T) {
	g := &model.Genre{Name: "Rock"}
	want := assetkey.Genre("Rock")
	got := assetkey.GenreOf(g)
	if got != want {
		t.Errorf("GenreOf = %q, want %q", got, want)
	}
}

// TestPlaylistStability verifies the same UUID yields the same key across calls.
func TestPlaylistStability(t *testing.T) {
	key1 := assetkey.Playlist("uuid-123")
	key2 := assetkey.Playlist("uuid-123")
	if key1 != key2 {
		t.Errorf("Playlist keys differ: %q vs %q", key1, key2)
	}
	if len(key1) != 64 {
		t.Errorf("Playlist key length = %d, want 64 (hex SHA-256)", len(key1))
	}
	if key1 != strings.ToLower(key1) {
		t.Errorf("Playlist key not lowercase: %q", key1)
	}
}

// TestPlaylistDistinctness verifies different UUIDs yield different keys.
func TestPlaylistDistinctness(t *testing.T) {
	key1 := assetkey.Playlist("uuid-123")
	key2 := assetkey.Playlist("uuid-456")
	if key1 == key2 {
		t.Errorf("Playlist keys equal despite different UUIDs")
	}
}

// TestRadioStability verifies the same stream URL yields the same key across calls.
func TestRadioStability(t *testing.T) {
	url := "https://example.com/stream"
	key1 := assetkey.Radio(url)
	key2 := assetkey.Radio(url)
	if key1 != key2 {
		t.Errorf("Radio keys differ: %q vs %q", key1, key2)
	}
	if len(key1) != 64 {
		t.Errorf("Radio key length = %d, want 64 (hex SHA-256)", len(key1))
	}
	if key1 != strings.ToLower(key1) {
		t.Errorf("Radio key not lowercase: %q", key1)
	}
}

// TestRadioDistinctness verifies different URLs yield different keys.
func TestRadioDistinctness(t *testing.T) {
	key1 := assetkey.Radio("https://example.com/stream1")
	key2 := assetkey.Radio("https://example.com/stream2")
	if key1 == key2 {
		t.Errorf("Radio keys equal despite different URLs")
	}
}

// TestCrossKindCollision verifies an album and a genre with textually identical
// identity strings yield different keys.
func TestCrossKindCollision(t *testing.T) {
	albumKey := assetkey.Album("rock", "", "")
	genreKey := assetkey.Genre("rock")
	if albumKey == genreKey {
		t.Errorf("Album and Genre keys collide: %q", albumKey)
	}
}
