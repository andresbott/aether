package scanfolder_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/scanfolder"
)

func TestNewSetNormalizesAndOrdersByName(t *testing.T) {
	dir := t.TempDir()
	set, err := scanfolder.NewSet([]scanfolder.Folder{
		{Name: " Zeta ", Path: filepath.Join(dir, "z") + string(filepath.Separator)},
		{Name: "Alpha", Path: filepath.Join(dir, "a"), ExcludePatterns: []string{`^\.`}, FollowSymlinks: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	all := set.All()
	if len(all) != 2 || set.Len() != 2 {
		t.Fatalf("expected 2 folders, got %d", len(all))
	}
	// Name order is the order folders are scanned in, which decides who owns a
	// file reachable from two of them.
	if all[0].Name != "Alpha" || all[1].Name != "Zeta" {
		t.Fatalf("order = %q, %q; want Alpha, Zeta (name order, names trimmed)", all[0].Name, all[1].Name)
	}
	if all[1].Path != filepath.Join(dir, "z") {
		t.Fatalf("path not cleaned: %q", all[1].Path)
	}
	if !all[0].FollowSymlinks || all[1].FollowSymlinks {
		t.Fatalf("FollowSymlinks not carried: %+v", all)
	}
}

func TestNewSetMakesRelativePathsAbsolute(t *testing.T) {
	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Music", Path: "some/relative/dir"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := set.All()[0].Path; !filepath.IsAbs(got) {
		t.Fatalf("path %q is not absolute", got)
	}
}

func TestNewSetRejects(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name    string
		folders []scanfolder.Folder
		wantErr string
	}{
		{"missing name", []scanfolder.Folder{{Path: dir}}, "name is required"},
		{"blank name", []scanfolder.Folder{{Name: "   ", Path: dir}}, "name is required"},
		{"name too long", []scanfolder.Folder{{Name: strings.Repeat("x", 201), Path: dir}}, "name too long"},
		{"missing path", []scanfolder.Folder{{Name: "Music"}}, "path is required"},
		{"bad exclude regex", []scanfolder.Folder{{Name: "Music", Path: dir, ExcludePatterns: []string{"("}}}, "invalid exclude pattern"},
		{"duplicate name", []scanfolder.Folder{
			{Name: "Music", Path: filepath.Join(dir, "a")},
			{Name: "Music", Path: filepath.Join(dir, "b")},
		}, "declared twice"},
		{"names equal after trimming", []scanfolder.Folder{
			{Name: "Music", Path: filepath.Join(dir, "a")},
			{Name: " Music ", Path: filepath.Join(dir, "b")},
		}, "declared twice"},
		{"equal roots", []scanfolder.Folder{
			{Name: "A", Path: filepath.Join(dir, "m")},
			{Name: "B", Path: filepath.Join(dir, "m") + string(filepath.Separator)},
		}, "overlap"},
		{"nested root, outer first", []scanfolder.Folder{
			{Name: "A", Path: filepath.Join(dir, "m")},
			{Name: "B", Path: filepath.Join(dir, "m", "kids")},
		}, "overlap"},
		{"nested root, inner first", []scanfolder.Folder{
			{Name: "A", Path: filepath.Join(dir, "m", "kids")},
			{Name: "B", Path: filepath.Join(dir, "m")},
		}, "overlap"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := scanfolder.NewSet(tc.folders)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// A root that shares a name prefix with another is a sibling, not a child.
func TestNewSetAcceptsSiblingPrefixRoots(t *testing.T) {
	dir := t.TempDir()
	if _, err := scanfolder.NewSet([]scanfolder.Folder{
		{Name: "A", Path: filepath.Join(dir, "music")},
		{Name: "B", Path: filepath.Join(dir, "music2")},
	}); err != nil {
		t.Fatalf("sibling roots must be accepted: %v", err)
	}
}

// Whether a root exists is runtime state (a share not mounted yet), not a
// configuration error: NewSet must accept it.
func TestNewSetAcceptsAMissingDirectory(t *testing.T) {
	if _, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Music", Path: filepath.Join(t.TempDir(), "not-mounted")}}); err != nil {
		t.Fatalf("a missing directory must not be a validation error: %v", err)
	}
}

func TestSetLookups(t *testing.T) {
	dir := t.TempDir()
	music := filepath.Join(dir, "music")
	books := filepath.Join(dir, "music2")
	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Music", Path: music}, {Name: "Books", Path: books}})
	if err != nil {
		t.Fatal(err)
	}

	if f, ok := set.ByName("Books"); !ok || f.Path != books {
		t.Fatalf("ByName(Books) = %+v, %v", f, ok)
	}
	if _, ok := set.ByName("books"); ok {
		t.Fatal("ByName must be exact: names are identifiers, not search terms")
	}

	for _, tc := range []struct {
		path string
		want string // "" = no folder contains it
	}{
		{filepath.Join(music, "Artist", "01.mp3"), "Music"},
		{music, "Music"},
		{filepath.Join(books, "x.m4b"), "Books"}, // /music2/... must not resolve to /music
		{filepath.Join(music, "..", "music2", "x.m4b"), "Books"},
		{filepath.Join(dir, "elsewhere", "x.mp3"), ""},
	} {
		f, ok := set.Containing(tc.path)
		if (tc.want == "") == ok || (ok && f.Name != tc.want) {
			t.Errorf("Containing(%q) = %q, %v; want %q", tc.path, f.Name, ok, tc.want)
		}
	}

	roots := set.Roots()
	if len(roots) != 2 || roots[0] != books || roots[1] != music {
		t.Fatalf("Roots() = %v, want [%s %s] (name order)", roots, books, music)
	}
}

func TestAllReturnsACopy(t *testing.T) {
	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Music", Path: t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	set.All()[0].Name = "mutated"
	if set.All()[0].Name != "Music" {
		t.Fatal("All must return a copy: the set is immutable")
	}
}

func TestNilSetIsEmpty(t *testing.T) {
	var set *scanfolder.Set
	if set.Len() != 0 || len(set.All()) != 0 || len(set.Roots()) != 0 {
		t.Fatal("a nil set must behave as empty")
	}
	if _, ok := set.ByName("Music"); ok {
		t.Fatal("ByName on a nil set must miss")
	}
	if _, ok := set.Containing("/music/x.mp3"); ok {
		t.Fatal("Containing on a nil set must miss")
	}
}

func TestFolderAvailable(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a-file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (scanfolder.Folder{Path: dir}).Available(); err != nil {
		t.Fatalf("an existing directory must be available: %v", err)
	}
	if err := (scanfolder.Folder{Path: filepath.Join(dir, "missing")}).Available(); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("missing dir: err = %v, want 'unavailable'", err)
	}
	if err := (scanfolder.Folder{Path: file}).Available(); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("file root: err = %v, want 'not a directory'", err)
	}
}

func TestFolderExcludes(t *testing.T) {
	res, err := (scanfolder.Folder{Name: "Music", ExcludePatterns: []string{`^\.`, `covers`}}).Excludes()
	if err != nil || len(res) != 2 || !res[0].MatchString(".hidden") || res[0].MatchString("visible") {
		t.Fatalf("Excludes() = %v, %v", res, err)
	}
	if _, err := (scanfolder.Folder{Name: "Music", ExcludePatterns: []string{"("}}).Excludes(); err == nil {
		t.Fatal("an uncompilable pattern must be an error")
	}
	if res, err := (scanfolder.Folder{}).Excludes(); err != nil || len(res) != 0 {
		t.Fatalf("no patterns: %v, %v", res, err)
	}
}
