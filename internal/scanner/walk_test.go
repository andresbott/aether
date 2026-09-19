// internal/scanner/walk_test.go
package scanner_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/tags"
)

// TestIsAudioFileTracksSupported guards the structural rule that the scanner
// keeps no private extension list: IsAudioFile must agree with tags.Supported
// for every extension, so the "what do we index" and "what do we edit" answers
// can never drift apart again.
func TestIsAudioFileTracksSupported(t *testing.T) {
	for _, ext := range []string{
		".mp3", ".flac", ".oga", ".m4a", ".mp4", ".wma", ".aac",
		".mpc", ".ape", ".wv", ".txt", ".jpg", "",
	} {
		name := "f" + ext
		if got, want := scanner.IsAudioFile(name), tags.Supported(name); got != want {
			t.Errorf("IsAudioFile(%q) = %v, tags.Supported = %v — a divergent scanner extension list has been reintroduced", name, got, want)
		}
	}
}

func createTestFiles(t *testing.T, dir string, files []string) {
	t.Helper()
	for _, f := range files {
		p := filepath.Join(dir, f)
		_ = os.MkdirAll(filepath.Dir(p), 0750)
		_ = os.WriteFile(p, []byte("fake"), 0644)
	}
}

func TestWalk(t *testing.T) {
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"artist1/album1/01.mp3",
		"artist1/album1/02.flac",
		"artist1/album1/cover.jpg",
		"artist1/album1/notes.txt",
		"artist2/album2/01.ogg",
	})

	folder := scanfolder.Folder{Name: "lib1", Path: dir, FollowSymlinks: true}
	results, err := scanner.Walk(folder, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 audio files, got %d", len(results))
	}
	for _, r := range results {
		if r.ScanFolder != "lib1" {
			t.Fatalf("expected ScanFolder=lib1, got %q", r.ScanFolder)
		}
	}
}

func TestWalkExcludePattern(t *testing.T) {
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"music/01.mp3",
		".hidden/02.mp3",
		"music/Thumbs.db",
	})

	excludes := []*regexp.Regexp{regexp.MustCompile(`^\..`)}
	folder := scanfolder.Folder{Name: "lib1", Path: dir, FollowSymlinks: true}
	results, err := scanner.Walk(folder, excludes)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 file after exclude, got %d", len(results))
	}
}

func TestWalkFollowsSymlinks(t *testing.T) {
	realDir := t.TempDir()
	libDir := t.TempDir()
	createTestFiles(t, realDir, []string{
		"linked-album/01.mp3",
		"linked-album/02.flac",
		"single.mp3",
	})
	createTestFiles(t, libDir, []string{"local/03.ogg"})
	// Directory symlink: its audio files must be collected.
	if err := os.Symlink(filepath.Join(realDir, "linked-album"), filepath.Join(libDir, "album-link")); err != nil {
		t.Fatal(err)
	}
	// File symlink: reported as an audio file itself.
	if err := os.Symlink(filepath.Join(realDir, "single.mp3"), filepath.Join(libDir, "single-link.mp3")); err != nil {
		t.Fatal(err)
	}
	// Broken symlink: skipped without error.
	if err := os.Symlink(filepath.Join(realDir, "missing"), filepath.Join(libDir, "broken.mp3")); err != nil {
		t.Fatal(err)
	}
	// Cycle: a symlink back to the library root must not recurse forever.
	if err := os.Symlink(libDir, filepath.Join(libDir, "self-link")); err != nil {
		t.Fatal(err)
	}

	folder := scanfolder.Folder{Name: "lib1", Path: libDir, FollowSymlinks: true}
	results, err := scanner.Walk(folder, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 audio files (2 linked-album + 1 file link + 1 local), got %d: %+v", len(results), results)
	}
}

func TestWalkNoFollowSymlinks(t *testing.T) {
	realDir := t.TempDir()
	libDir := t.TempDir()
	createTestFiles(t, realDir, []string{"linked-album/01.mp3"})
	createTestFiles(t, libDir, []string{"local/02.ogg"})
	if err := os.Symlink(filepath.Join(realDir, "linked-album"), filepath.Join(libDir, "album-link")); err != nil {
		t.Fatal(err)
	}

	folder := scanfolder.Folder{Name: "lib1", Path: libDir, FollowSymlinks: false}
	results, err := scanner.Walk(folder, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 audio file (symlink not followed), got %d", len(results))
	}
}

func TestWalkMultipleLibraries(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	createTestFiles(t, dir1, []string{"01.mp3"})
	createTestFiles(t, dir2, []string{"02.flac"})

	folder1 := scanfolder.Folder{Name: "lib1", Path: dir1, FollowSymlinks: true}
	folder2 := scanfolder.Folder{Name: "lib2", Path: dir2, FollowSymlinks: true}
	// Walk takes one scan folder; the multi-folder case calls it once per folder
	// and appends the results, exactly what Scan's preflight loop does.
	results1, err := scanner.Walk(folder1, nil)
	if err != nil {
		t.Fatal(err)
	}
	results2, err := scanner.Walk(folder2, nil)
	if err != nil {
		t.Fatal(err)
	}
	results := append(results1, results2...)
	if len(results) != 2 {
		t.Fatalf("expected 2 files, got %d", len(results))
	}
	byFolder := map[string]string{}
	for _, r := range results {
		byFolder[r.ScanFolder] = filepath.Base(r.FilePath)
	}
	if byFolder["lib1"] != "01.mp3" {
		t.Fatalf("lib1 should map to 01.mp3, got %q", byFolder["lib1"])
	}
	if byFolder["lib2"] != "02.flac" {
		t.Fatalf("lib2 should map to 02.flac, got %q", byFolder["lib2"])
	}
}

// The walk must record the size of the audio file itself, including through a
// symlink: planTrackContinuity proves a file moved by matching file_size, and a
// symlink's own size (the length of its target path) would be a false
// fingerprint. It is also what Subsonic reports to clients as `size`.
func TestWalkRecordsFileSizeThroughSymlinks(t *testing.T) {
	libDir := t.TempDir()
	realDir := t.TempDir()
	body := []byte("0123456789")
	if err := os.WriteFile(filepath.Join(libDir, "plain.mp3"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(realDir, "single.mp3")
	if err := os.WriteFile(target, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(libDir, "single-link.mp3")); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}

	folder := scanfolder.Folder{Name: "lib1", Path: libDir, FollowSymlinks: true}
	results, err := scanner.Walk(folder, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 audio files, got %d: %+v", len(results), results)
	}
	for _, r := range results {
		if r.FileSize != int64(len(body)) {
			t.Errorf("%s: FileSize = %d, want %d", r.FilePath, r.FileSize, len(body))
		}
	}
}
