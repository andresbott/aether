package metadataedit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/metadataedit"
)

// paths flattens a listing for path membership/order assertions.
func paths(folders []metadataedit.Folder) []string {
	out := make([]string, 0, len(folders))
	for _, f := range folders {
		out = append(out, f.Path)
	}
	return out
}

// searchTree builds a small library tree with nested folders whose names are
// what the search matches against.
func searchTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{
		filepath.Join("Alexia dixon", "fire up"),
		filepath.Join("Alexia dixon", "ballad"),
		filepath.Join("Other", "thing"),
		filepath.Join(".hidden", "up secret"),
	} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestSearchFolders_MatchesDeepFolderByName(t *testing.T) {
	root := searchTree(t)
	got, truncated, err := metadataedit.SearchFolders(root, "up", metadataedit.ListFoldersOptions{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Fatal("did not expect truncation")
	}
	if want := []string{"Alexia dixon/fire up"}; !sameNames(paths(got), want) {
		t.Fatalf("got %v want %v", paths(got), want)
	}
	if got[0].Name != "fire up" {
		t.Fatalf("name: got %q want %q", got[0].Name, "fire up")
	}
	if got[0].HasSubfolders {
		t.Fatalf("fire up is a leaf, HasSubfolders should be false")
	}
}

func TestSearchFolders_CaseInsensitive(t *testing.T) {
	root := searchTree(t)
	got, _, err := metadataedit.SearchFolders(root, "UP", metadataedit.ListFoldersOptions{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Alexia dixon/fire up"}; !sameNames(paths(got), want) {
		t.Fatalf("got %v want %v", paths(got), want)
	}
}

func TestSearchFolders_MatchesAncestorName(t *testing.T) {
	root := searchTree(t)
	got, _, err := metadataedit.SearchFolders(root, "dixon", metadataedit.ListFoldersOptions{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Alexia dixon"}; !sameNames(paths(got), want) {
		t.Fatalf("got %v want %v", paths(got), want)
	}
	if !got[0].HasSubfolders {
		t.Fatalf("Alexia dixon has children, HasSubfolders should be true")
	}
}

func TestSearchFolders_SkipsHiddenByDefault(t *testing.T) {
	root := searchTree(t)
	// ".hidden/up secret" also contains "up" but must not appear with hidden off.
	got, _, err := metadataedit.SearchFolders(root, "up", metadataedit.ListFoldersOptions{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths(got) {
		if p == ".hidden/up secret" {
			t.Fatalf("hidden folder leaked into results: %v", paths(got))
		}
	}
}

func TestSearchFolders_EmptyQueryReturnsNothing(t *testing.T) {
	root := searchTree(t)
	got, _, err := metadataedit.SearchFolders(root, "  ", metadataedit.ListFoldersOptions{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("blank query should match nothing, got %v", paths(got))
	}
}

func TestSearchFolders_Truncates(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"up 1", "up 2", "up 3"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got, truncated, err := metadataedit.SearchFolders(root, "up", metadataedit.ListFoldersOptions{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !truncated {
		t.Fatal("expected truncated=true when matches exceed the limit")
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results at limit 2, got %d", len(got))
	}
}
