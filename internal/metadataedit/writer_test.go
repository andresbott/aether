package metadataedit_test

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/andresbott/aether/internal/metadataedit"
	"go.senan.xyz/taglib"
)

func TestBuildTagMap_AppliesOnlyProvidedFields(t *testing.T) {
	patch := metadataedit.Patch{
		Title: strPtr("Hello"),
	}
	got, err := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"TITLE": {"Hello"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_MultiValueDefault_WritesArray(t *testing.T) {
	patch := metadataedit.Patch{Artists: &[]string{"A", "B"}}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{MultiValueArtist: ""})
	want := map[string][]string{"ARTIST": {"A", "B"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_MultiValueMulti_WritesArray(t *testing.T) {
	patch := metadataedit.Patch{Artists: &[]string{"A", "B"}}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{MultiValueArtist: "multi"})
	want := map[string][]string{"ARTIST": {"A", "B"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_Delim_JoinsWithSeparator(t *testing.T) {
	patch := metadataedit.Patch{Artists: &[]string{"A", "B"}}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{MultiValueArtist: "delim ; "})
	want := map[string][]string{"ARTIST": {"A; B"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_None_KeepsFirstOnly(t *testing.T) {
	patch := metadataedit.Patch{Artists: &[]string{"A", "B", "C"}}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{MultiValueArtist: "none"})
	want := map[string][]string{"ARTIST": {"A"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_AllFields(t *testing.T) {
	patch := metadataedit.Patch{
		Title:        strPtr("T"),
		Album:        strPtr("Al"),
		Artists:      &[]string{"a1", "a2"},
		AlbumArtists: &[]string{"aa"},
		Year:         intPtr(2001),
		Compilation:  boolPtr(true),
	}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{
		MultiValueArtist:      "multi",
		MultiValueAlbumArtist: "none",
	})
	want := map[string][]string{
		"TITLE":       {"T"},
		"ALBUM":       {"Al"},
		"ARTIST":      {"a1", "a2"},
		"ALBUMARTIST": {"aa"},
		"DATE":        {"2001"},
		"COMPILATION": {"1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_CompilationFalseWritesZero(t *testing.T) {
	patch := metadataedit.Patch{Compilation: boolPtr(false)}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{})
	want := map[string][]string{"COMPILATION": {"0"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_RejectsInvalidDelim(t *testing.T) {
	patch := metadataedit.Patch{Artists: &[]string{"A"}}
	_, err := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{MultiValueArtist: "bogus"})
	if err == nil {
		t.Fatal("expected error on invalid multi-value mode")
	}
}

func TestWriteMetadata_RoundTripFLAC(t *testing.T) {
	src := "testdata/empty.flac"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no fixture at %s: %v", src, err)
	}
	dst := filepath.Join(t.TempDir(), "copy.flac")
	copyFileForWriter(t, src, dst)

	patch := metadataedit.Patch{
		Title:        strPtr("Round Trip"),
		Album:        strPtr("Test Album"),
		Artists:      &[]string{"First", "Second"},
		AlbumArtists: &[]string{"Various"},
		Year:         intPtr(1999),
		Compilation:  boolPtr(true),
	}
	cfg := metadataedit.LibraryCfg{MultiValueArtist: "multi", MultiValueAlbumArtist: "multi"}

	if err := metadataedit.WriteMetadata(dst, patch, cfg); err != nil {
		t.Fatalf("WriteMetadata: %v", err)
	}

	got, err := taglib.ReadTags(dst)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got["TITLE"][0] != "Round Trip" {
		t.Fatalf("title round-trip failed: %v", got["TITLE"])
	}
	if got["ALBUM"][0] != "Test Album" {
		t.Fatalf("album round-trip failed: %v", got["ALBUM"])
	}
	if len(got["ARTIST"]) != 2 || got["ARTIST"][0] != "First" || got["ARTIST"][1] != "Second" {
		t.Fatalf("artist multi-value round-trip failed: %v", got["ARTIST"])
	}
	if got["DATE"][0] != "1999" {
		t.Fatalf("year round-trip failed: %v", got["DATE"])
	}
}

func TestWriteMetadata_EmptyPatchIsNoOp(t *testing.T) {
	src := "testdata/empty.flac"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no fixture at %s: %v", src, err)
	}
	dst := filepath.Join(t.TempDir(), "copy.flac")
	copyFileForWriter(t, src, dst)

	if err := metadataedit.WriteMetadata(dst, metadataedit.Patch{}, metadataedit.LibraryCfg{}); err != nil {
		t.Fatalf("unexpected error on empty patch: %v", err)
	}
}

func copyFileForWriter(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
func boolPtr(b bool) *bool    { return &b }
