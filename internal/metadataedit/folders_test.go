package metadataedit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/metadataedit"
)

func TestListFolders_EmptyDir(t *testing.T) {
	root := t.TempDir()
	folders, err := metadataedit.ListFolders(root)
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

	folders, err := metadataedit.ListFolders(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 3 {
		t.Fatalf("expected 3 folders, got %d: %+v", len(folders), folders)
	}
	wantOrder := []string{"alpha", "mike", "zeta"}
	for i, f := range folders {
		if f.Name != wantOrder[i] {
			t.Fatalf("pos %d: got %q want %q", i, f.Name, wantOrder[i])
		}
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

	folders, err := metadataedit.ListFolders(root)
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
