package artistimage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/artistimage"
)

func createTestFiles(t *testing.T, root string, rel []string) {
	t.Helper()
	for _, r := range rel {
		p := filepath.Join(root, r)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBestFilename(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  string
	}{
		{"artist wins over everything", []string{"folder.jpg", "artistthumb.png", "artist.jpg"}, "artist.jpg"},
		{"artistthumb beats folder", []string{"folder.jpg", "artistthumb.png"}, "artistthumb.png"},
		{"folder alone", []string{"folder.png"}, "folder.png"},
		{"case insensitive", []string{"Artist.JPG"}, "Artist.JPG"},
		{"album art names are not artist images", []string{"cover.jpg", "front.png", "album.jpg"}, ""},
		{"non-front art rejected", []string{"back.jpg", "booklet.png"}, ""},
		{"non-image extensions rejected", []string{"artist.txt", "artist.nfo"}, ""},
		{"audio files rejected", []string{"01-track.mp3"}, ""},
		{"substring matches rejected", []string{"the artist live.jpg"}, ""},
		{"empty", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := artistimage.BestFilename(tt.files); got != tt.want {
				t.Errorf("BestFilename(%v) = %q, want %q", tt.files, got, tt.want)
			}
		})
	}
}

func TestIsUsablePath(t *testing.T) {
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Pink Floyd/artist.jpg", "Pink Floyd/cover.jpg", "Some Artist/artist.jpg/keep"})
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"existing artist image", filepath.Join(dir, "Pink Floyd/artist.jpg"), true},
		{"existing but not an artist-image name", filepath.Join(dir, "Pink Floyd/cover.jpg"), false},
		{"missing file", filepath.Join(dir, "Pink Floyd/artistthumb.png"), false},
		{"empty path", "", false},
		{"directory", filepath.Join(dir, "Pink Floyd"), false},
		{"directory named like an image", filepath.Join(dir, "Some Artist/artist.jpg"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := artistimage.IsUsablePath(tt.path); got != tt.want {
				t.Errorf("IsUsablePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// Detect accepts an image only in a directory that is both above the album
// directory and named after the artist, so a library not laid out as
// <collection>/<artist>/<album> never gets a wrong image. startDir is the
// directory the track file sits in.
func TestDetect(t *testing.T) {
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Pink Floyd/artist.jpg",
		"Pink Floyd/The Wall/01.flac",
		"Pink Floyd/The Wall/cover.jpg",
		"Pink Floyd/The Wall/CD1/02.flac",
		"Nirvana/folder.png",
		"Nirvana/Nevermind/01.flac",
		"The Doors/The Doors/01.flac",
		"The Doors/The Doors/folder.jpg",
		"Flat Artist/01.flac",
		"Flat Artist/artist.jpg",
		"Compilations/Some Sampler/01.flac",
		"Compilations/artist.jpg",
	})
	tests := []struct {
		name      string
		trackPath string
		artist    string
		want      string
	}{
		{"artist image in the artist folder", "Pink Floyd/The Wall/01.flac", "Pink Floyd", "Pink Floyd/artist.jpg"},
		{"found from a disc subdirectory", "Pink Floyd/The Wall/CD1/02.flac", "Pink Floyd", "Pink Floyd/artist.jpg"},
		{"folder.png in the artist folder", "Nirvana/Nevermind/01.flac", "Nirvana", "Nirvana/folder.png"},
		{"album cover is not an artist image", "Pink Floyd/The Wall/01.flac", "The Wall", ""},
		{"folder.jpg inside the album directory is left alone", "The Doors/The Doors/01.flac", "The Doors", ""},
		{"no album level means no artist folder", "Flat Artist/01.flac", "Flat Artist", ""},
		{"directory name does not match the artist", "Compilations/Some Sampler/01.flac", "Various Artists", ""},
		{"artist name matched after normalisation", "Pink Floyd/The Wall/01.flac", "pink floyd", "Pink Floyd/artist.jpg"},
		{"empty artist name", "Pink Floyd/The Wall/01.flac", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := ""
			if tt.want != "" {
				want = filepath.Join(dir, tt.want)
			}
			startDir := filepath.Dir(filepath.Join(dir, tt.trackPath))
			if got := artistimage.Detect(dir, startDir, tt.artist); got != want {
				t.Errorf("Detect(%q, %q) = %q, want %q", tt.trackPath, tt.artist, got, want)
			}
		})
	}
}

// Detect returns the image from the NEAREST same-named ancestor that has one; a
// deeper artist-named folder without an image falls through to a shallower one.
func TestDetectNearestSameNamedAncestorWins(t *testing.T) {
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Pink Floyd/artist.jpg",            // shallow ancestor has an image
		"Pink Floyd/Pink Floyd/artist.jpg", // deeper ancestor also has one
		"Pink Floyd/Pink Floyd/The Wall/01.flac",
		"Genesis/artist.jpg", // only the shallow ancestor has an image
		"Genesis/Genesis/The Lamb/01.flac",
	})
	tests := []struct {
		name      string
		trackPath string
		artist    string
		want      string
	}{
		{"nearest same-named ancestor wins", "Pink Floyd/Pink Floyd/The Wall/01.flac", "Pink Floyd", "Pink Floyd/Pink Floyd/artist.jpg"},
		{"falls through to a shallower ancestor when the nearer has none", "Genesis/Genesis/The Lamb/01.flac", "Genesis", "Genesis/artist.jpg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := filepath.Join(dir, tt.want)
			startDir := filepath.Dir(filepath.Join(dir, tt.trackPath))
			if got := artistimage.Detect(dir, startDir, tt.artist); got != want {
				t.Errorf("Detect(%q, %q) = %q, want %q", tt.trackPath, tt.artist, got, want)
			}
		})
	}
}

// FindDir returns the artist folder even when it holds no image yet, so a caller
// can create one there.
func TestFindDir(t *testing.T) {
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Pink Floyd/The Wall/01.flac", // artist folder exists but has NO image
		"Nirvana/Nevermind/01.flac",
		"Nirvana/folder.png",
	})
	tests := []struct {
		name     string
		startDir string
		artist   string
		wantDir  string
		wantOK   bool
	}{
		{"folder without image", "Pink Floyd/The Wall", "Pink Floyd", "Pink Floyd", true},
		{"folder with image", "Nirvana/Nevermind", "Nirvana", "Nirvana", true},
		{"no artist folder", "Pink Floyd/The Wall", "Nobody", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDir, gotOK := artistimage.FindDir(dir, filepath.Join(dir, tt.startDir), tt.artist)
			wantDir := ""
			if tt.wantDir != "" {
				wantDir = filepath.Join(dir, tt.wantDir)
			}
			if gotDir != wantDir || gotOK != tt.wantOK {
				t.Errorf("FindDir(%q,%q) = %q,%v want %q,%v", tt.startDir, tt.artist, gotDir, gotOK, wantDir, tt.wantOK)
			}
		})
	}
}

// Detection must never resolve an image by walking above the library root.
func TestDetectStaysInsideLibrary(t *testing.T) {
	root := t.TempDir()
	createTestFiles(t, root, []string{
		"artist.jpg",
		"lib/Pink Floyd/The Wall/01.flac",
	})
	libRoot := filepath.Join(root, "lib")
	startDir := filepath.Dir(filepath.Join(libRoot, "Pink Floyd/The Wall/01.flac"))
	if got := artistimage.Detect(libRoot, startDir, "lib"); got != "" {
		t.Errorf("Detect crossed the library root, got %q", got)
	}
}
