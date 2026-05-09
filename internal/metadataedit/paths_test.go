package metadataedit_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/metadataedit"
)

func TestResolveInLibrary_Empty(t *testing.T) {
	root := t.TempDir()
	got, err := metadataedit.ResolveInLibrary(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(root) {
		t.Fatalf("empty rel should resolve to root, got %q", got)
	}
}

func TestResolveInLibrary_Nested(t *testing.T) {
	root := t.TempDir()
	got, err := metadataedit.ResolveInLibrary(root, "a/b")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "a", "b")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveInLibrary_RejectsParentEscape(t *testing.T) {
	root := t.TempDir()
	_, err := metadataedit.ResolveInLibrary(root, "../etc/passwd")
	if err == nil {
		t.Fatal("expected error on .. escape")
	}
	if !errors.Is(err, metadataedit.ErrOutsideLibrary) {
		t.Fatalf("expected ErrOutsideLibrary, got %v", err)
	}
	if !strings.Contains(err.Error(), "outside") {
		t.Fatalf("expected 'outside library' error, got %v", err)
	}
}

func TestResolveInLibrary_RejectsAbsolute(t *testing.T) {
	root := t.TempDir()
	_, err := metadataedit.ResolveInLibrary(root, "/etc/passwd")
	if err == nil {
		t.Fatal("expected error on absolute path")
	}
	if !errors.Is(err, metadataedit.ErrOutsideLibrary) {
		t.Fatalf("expected ErrOutsideLibrary, got %v", err)
	}
}

func TestResolveInLibrary_RejectsDotDotInMiddle(t *testing.T) {
	root := t.TempDir()
	_, err := metadataedit.ResolveInLibrary(root, "a/../../b")
	if err == nil {
		t.Fatal("expected error on mid-path escape")
	}
	if !errors.Is(err, metadataedit.ErrOutsideLibrary) {
		t.Fatalf("expected ErrOutsideLibrary, got %v", err)
	}
}
