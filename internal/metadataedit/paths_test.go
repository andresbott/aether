package metadataedit_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/metadataedit"
)

func TestResolveInRoot_Empty(t *testing.T) {
	root := t.TempDir()
	got, err := metadataedit.ResolveInRoot(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(root) {
		t.Fatalf("empty rel should resolve to root, got %q", got)
	}
}

func TestResolveInRoot_Nested(t *testing.T) {
	root := t.TempDir()
	got, err := metadataedit.ResolveInRoot(root, "a/b")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "a", "b")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveInRoot_RejectsParentEscape(t *testing.T) {
	root := t.TempDir()
	_, err := metadataedit.ResolveInRoot(root, "../etc/passwd")
	if err == nil {
		t.Fatal("expected error on .. escape")
	}
	if !errors.Is(err, metadataedit.ErrOutsideRoot) {
		t.Fatalf("expected ErrOutsideRoot, got %v", err)
	}
	if !strings.Contains(err.Error(), "outside") {
		t.Fatalf("expected the outside-the-scan-folder error, got %v", err)
	}
}

func TestResolveInRoot_RejectsAbsolute(t *testing.T) {
	root := t.TempDir()
	_, err := metadataedit.ResolveInRoot(root, "/etc/passwd")
	if err == nil {
		t.Fatal("expected error on absolute path")
	}
	if !errors.Is(err, metadataedit.ErrOutsideRoot) {
		t.Fatalf("expected ErrOutsideRoot, got %v", err)
	}
}

func TestResolveInRoot_RejectsDotDotInMiddle(t *testing.T) {
	root := t.TempDir()
	_, err := metadataedit.ResolveInRoot(root, "a/../../b")
	if err == nil {
		t.Fatal("expected error on mid-path escape")
	}
	if !errors.Is(err, metadataedit.ErrOutsideRoot) {
		t.Fatalf("expected ErrOutsideRoot, got %v", err)
	}
}
