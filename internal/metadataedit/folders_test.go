package metadataedit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/metadataedit"
)

// names flattens a listing for order/membership assertions.
func names(folders []metadataedit.Folder) []string {
	out := make([]string, 0, len(folders))
	for _, f := range folders {
		out = append(out, f.Name)
	}
	return out
}

func sameNames(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestListFolders_EmptyDir(t *testing.T) {
	root := t.TempDir()
	folders, err := metadataedit.ListFolders(root, metadataedit.ListFoldersOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 0 {
		t.Fatalf("expected empty, got %d", len(folders))
	}
}

func TestListFolders_ReturnsSortedDirsOnly(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"zeta", "alpha", "mike"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "song.mp3"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}

	folders, err := metadataedit.ListFolders(root, metadataedit.ListFoldersOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := names(folders), []string{"alpha", "mike", "zeta"}; !sameNames(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestListFolders_IncludeHidden(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"visible", ".hidden"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	folders, err := metadataedit.ListFolders(root, metadataedit.ListFoldersOptions{IncludeHidden: true})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := names(folders), []string{".hidden", "visible"}; !sameNames(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestListFolders_HasSubfoldersFollowsIncludeHidden(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	if err := os.MkdirAll(filepath.Join(parent, ".only-hidden-child"), 0o755); err != nil {
		t.Fatal(err)
	}

	hiddenOff, err := metadataedit.ListFolders(root, metadataedit.ListFoldersOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(hiddenOff) != 1 || hiddenOff[0].HasSubfolders {
		t.Fatalf("with hidden off, parent should look like a leaf: %+v", hiddenOff)
	}

	hiddenOn, err := metadataedit.ListFolders(root, metadataedit.ListFoldersOptions{IncludeHidden: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(hiddenOn) != 1 || !hiddenOn[0].HasSubfolders {
		t.Fatalf("with hidden on, parent should be expandable: %+v", hiddenOn)
	}
}

func TestListFolders_HasSubfoldersFlag(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	leaf := filepath.Join(root, "leaf")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(leaf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(parent, "child"), 0o755); err != nil {
		t.Fatal(err)
	}

	folders, err := metadataedit.ListFolders(root, metadataedit.ListFoldersOptions{})
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]metadataedit.Folder{}
	for _, f := range folders {
		byName[f.Name] = f
	}
	if !byName["parent"].HasSubfolders {
		t.Fatal("parent should report HasSubfolders=true")
	}
	if byName["leaf"].HasSubfolders {
		t.Fatal("leaf should report HasSubfolders=false")
	}
}

// symlinkRoot builds a tree with one real dir, a link to a dir outside the
// listing, a link to a file and a broken link.
func symlinkRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	target := t.TempDir()
	if err := os.Mkdir(filepath.Join(target, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "song.mp3")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(file, filepath.Join(root, "linked-file")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(target, "gone"), filepath.Join(root, "linked-broken")); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestListFolders_SymlinksExcludedByDefault(t *testing.T) {
	root := symlinkRoot(t)
	folders, err := metadataedit.ListFolders(root, metadataedit.ListFoldersOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := names(folders), []string{"real"}; !sameNames(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestListFolders_IncludeSymlinksOnlyDirTargets(t *testing.T) {
	root := symlinkRoot(t)
	folders, err := metadataedit.ListFolders(root, metadataedit.ListFoldersOptions{IncludeSymlinks: true})
	if err != nil {
		t.Fatal(err)
	}
	// linked-file (file target) and linked-broken (dangling) are not folders.
	if got, want := names(folders), []string{"linked", "real"}; !sameNames(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	byName := map[string]metadataedit.Folder{}
	for _, f := range folders {
		byName[f.Name] = f
	}
	if !byName["linked"].IsSymlink {
		t.Fatal("linked should be flagged as a symlink")
	}
	if !byName["linked"].HasSubfolders {
		t.Fatal("linked should be expandable: its target has a child dir")
	}
	if byName["real"].IsSymlink {
		t.Fatal("real should not be flagged as a symlink")
	}
}

func TestListFolders_HasSubfoldersThroughSymlinkedChild(t *testing.T) {
	root := t.TempDir()
	target := t.TempDir()
	parent := filepath.Join(root, "parent")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(parent, "link-child")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	off, err := metadataedit.ListFolders(root, metadataedit.ListFoldersOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(off) != 1 || off[0].HasSubfolders {
		t.Fatalf("without symlinks, parent should look like a leaf: %+v", off)
	}

	on, err := metadataedit.ListFolders(root, metadataedit.ListFoldersOptions{IncludeSymlinks: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(on) != 1 || !on[0].HasSubfolders {
		t.Fatalf("with symlinks, parent should be expandable: %+v", on)
	}
}

func TestListFolders_HiddenSymlinkNeedsBothOptions(t *testing.T) {
	root := t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(root, ".linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	symlinksOnly, err := metadataedit.ListFolders(root, metadataedit.ListFoldersOptions{IncludeSymlinks: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(symlinksOnly) != 0 {
		t.Fatalf("a hidden symlink needs IncludeHidden too: %+v", symlinksOnly)
	}

	both, err := metadataedit.ListFolders(root, metadataedit.ListFoldersOptions{
		IncludeHidden:   true,
		IncludeSymlinks: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := names(both), []string{".linked"}; !sameNames(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
