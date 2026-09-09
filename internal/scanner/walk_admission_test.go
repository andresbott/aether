// internal/scanner/walk_admission_test.go
package scanner_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanner"
)

// The editor's rescan admits one path at a time; the full Walk discovers files
// by crawling. They must agree on membership, or a rescan indexes a row the
// next scheduled scan cannot see and deletes — churning the track id and
// cascading away its stars, playlist memberships and play history. These tests
// are the guardrail that keeps the two in agreement.

// A file reachable only by descending a symlinked directory is invisible to a
// no-follow Walk (filepath.WalkDir does not follow symlinks), so single-path
// admission must refuse it too.
func TestWalkWouldEmitRejectsFileUnderUnfollowedSymlinkedDir(t *testing.T) {
	libDir := t.TempDir()
	realDir := t.TempDir()
	createTestFiles(t, realDir, []string{"linked-album/01.mp3"})
	createTestFiles(t, libDir, []string{"local/02.ogg"})
	if err := os.Symlink(filepath.Join(realDir, "linked-album"), filepath.Join(libDir, "album-link")); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}

	const follow = false
	lib := model.Library{ID: 1, Path: libDir}
	walked, err := scanner.Walk([]model.Library{lib}, nil, follow)
	if err != nil {
		t.Fatal(err)
	}
	emitted := make(map[string]bool, len(walked))
	for _, r := range walked {
		emitted[r.FilePath] = true
	}

	underSymlink := filepath.Join(libDir, "album-link", "01.mp3")
	if emitted[underSymlink] {
		t.Fatalf("test premise broken: no-follow Walk emitted %q", underSymlink)
	}
	if _, ok := scanner.WalkWouldEmit(lib.Path, lib.ID, underSymlink, nil, follow); ok {
		t.Errorf("WalkWouldEmit admitted %q, but a no-follow Walk does not emit it — a rescan would index a row the next scan deletes", underSymlink)
	}
}

// Under follow-symlinks, the crawling Walk records a file reached through a
// symlinked directory by its resolved (canonical) path. Admission must produce
// that same spelling, or the same physical file is indexed under two paths —
// duplicating the row or churning its id through planTrackContinuity's
// path-identity matching.
func TestWalkWouldEmitCanonicalizesFollowedSymlinkPath(t *testing.T) {
	libDir := t.TempDir()
	realDir := t.TempDir()
	createTestFiles(t, realDir, []string{"linked-album/01.mp3"})
	if err := os.Symlink(filepath.Join(realDir, "linked-album"), filepath.Join(libDir, "album-link")); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}

	const follow = true
	lib := model.Library{ID: 1, Path: libDir}
	walked, err := scanner.Walk([]model.Library{lib}, nil, follow)
	if err != nil {
		t.Fatal(err)
	}
	emitted := make(map[string]bool, len(walked))
	for _, r := range walked {
		emitted[r.FilePath] = true
	}

	spelled := filepath.Join(libDir, "album-link", "01.mp3")
	wr, ok := scanner.WalkWouldEmit(lib.Path, lib.ID, spelled, nil, follow)
	if !ok {
		t.Fatalf("WalkWouldEmit rejected %q, but a follow Walk indexes it", spelled)
	}
	if !emitted[wr.FilePath] {
		got := make([]string, 0, len(emitted))
		for p := range emitted {
			got = append(got, p)
		}
		t.Errorf("WalkWouldEmit stored FilePath %q, but Walk emitted %v — the same file under two spellings", wr.FilePath, got)
	}
}

// TestWalkWouldEmitAgreesWithWalk is the guardrail the hand-maintained mirror
// always lacked: over one fixture tree exercising excludes, a pruned ancestor, a
// symlinked directory, a symlink into a directory excluded by its real name, and
// a symlinked file, single-path admission must accept exactly the set the
// crawling Walk emits — under identical FilePath spellings — in both symlink
// modes. Any future drift between the two ends fails here.
func TestWalkWouldEmitAgreesWithWalk(t *testing.T) {
	libDir := t.TempDir()
	realDir := t.TempDir()
	createTestFiles(t, libDir, []string{
		"a/01.mp3",
		"a/cover.jpg",
		"a/notes.txt",
		"b/03.flac",
		"b/.hidden/02.mp3",
	})
	createTestFiles(t, realDir, []string{
		"linked-album/04.mp3",
		"private/05.mp3",
		"single.mp3",
	})
	links := map[string]string{
		"album-link":     filepath.Join(realDir, "linked-album"),
		"private-link":   filepath.Join(realDir, "private"),
		"track-link.mp3": filepath.Join(realDir, "single.mp3"),
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(libDir, name)); err != nil {
			t.Skipf("symlinks unsupported here: %v", err)
		}
	}

	// Every file spelled the lexical, symlink-preserving way the editor hands to
	// the rescan. Non-audio and excluded entries are included on purpose: they
	// must be refused in both modes.
	probes := []string{
		filepath.Join(libDir, "a", "01.mp3"),
		filepath.Join(libDir, "a", "cover.jpg"),
		filepath.Join(libDir, "a", "notes.txt"),
		filepath.Join(libDir, "b", "03.flac"),
		filepath.Join(libDir, "b", ".hidden", "02.mp3"),
		filepath.Join(libDir, "album-link", "04.mp3"),
		filepath.Join(libDir, "private-link", "05.mp3"),
		filepath.Join(libDir, "track-link.mp3"),
	}
	// Exclude the hidden dir by name, and a directory by its real (resolved)
	// name — so the fixture covers an exclude that only bites once symlinks are
	// resolved, the spelling the crawl tests against.
	excludes := []*regexp.Regexp{
		regexp.MustCompile(`\.hidden`),
		regexp.MustCompile(`^private$`),
	}

	for _, follow := range []bool{false, true} {
		t.Run(fmt.Sprintf("follow=%v", follow), func(t *testing.T) {
			lib := model.Library{ID: 1, Path: libDir}
			walked, err := scanner.Walk([]model.Library{lib}, excludes, follow)
			if err != nil {
				t.Fatal(err)
			}
			walkSet := map[string]bool{}
			for _, r := range walked {
				walkSet[r.FilePath] = true
			}

			admitSet := map[string]bool{}
			for _, p := range probes {
				if wr, ok := scanner.WalkWouldEmit(lib.Path, lib.ID, p, excludes, follow); ok {
					admitSet[wr.FilePath] = true
				}
			}

			if !reflect.DeepEqual(walkSet, admitSet) {
				t.Errorf("admission and Walk disagree\n  Walk emits: %v\n  admits:     %v", sortedKeys(walkSet), sortedKeys(admitSet))
			}
		})
	}
}

func sortedKeys(m map[string]bool) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
