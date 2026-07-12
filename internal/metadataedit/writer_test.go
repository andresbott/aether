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
	got, err := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{}, metadataedit.CurrentTags{})
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
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{MultiValueArtist: ""}, metadataedit.CurrentTags{})
	want := map[string][]string{"ARTIST": {"A", "B"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_MultiValueMulti_WritesArray(t *testing.T) {
	patch := metadataedit.Patch{Artists: &[]string{"A", "B"}}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{MultiValueArtist: "multi"}, metadataedit.CurrentTags{})
	want := map[string][]string{"ARTIST": {"A", "B"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_Delim_JoinsWithSeparator(t *testing.T) {
	patch := metadataedit.Patch{Artists: &[]string{"A", "B"}}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{MultiValueArtist: "delim ; "}, metadataedit.CurrentTags{})
	want := map[string][]string{"ARTIST": {"A; B"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_None_KeepsFirstOnly(t *testing.T) {
	patch := metadataedit.Patch{Artists: &[]string{"A", "B", "C"}}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{MultiValueArtist: "none"}, metadataedit.CurrentTags{})
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
		DiscNumber:   intPtr(2),
		DiscSubtitle: strPtr("CD 2"),
		Compilation:  boolPtr(true),
	}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{
		MultiValueArtist:      "multi",
		MultiValueAlbumArtist: "none",
	}, metadataedit.CurrentTags{})
	want := map[string][]string{
		"TITLE":        {"T"},
		"ALBUM":        {"Al"},
		"ARTIST":       {"a1", "a2"},
		"ALBUMARTIST":  {"aa"},
		"DATE":         {"2001"},
		"DISCNUMBER":   {"2"},
		"DISCSUBTITLE": {"CD 2"},
		"COMPILATION":  {"1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_DiscFieldsOnly(t *testing.T) {
	patch := metadataedit.Patch{DiscNumber: intPtr(1), DiscSubtitle: strPtr("Original Score")}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{}, metadataedit.CurrentTags{})
	want := map[string][]string{"DISCNUMBER": {"1"}, "DISCSUBTITLE": {"Original Score"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_OmitsUnsetDiscFields(t *testing.T) {
	// A patch touching only the title must not emit any disc keys.
	patch := metadataedit.Patch{Title: strPtr("x")}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{}, metadataedit.CurrentTags{})
	if _, ok := got["DISCNUMBER"]; ok {
		t.Fatalf("unexpected DISCNUMBER in %v", got)
	}
	if _, ok := got["DISCSUBTITLE"]; ok {
		t.Fatalf("unexpected DISCSUBTITLE in %v", got)
	}
}

func TestBuildTagMap_CompilationFalseWritesZero(t *testing.T) {
	patch := metadataedit.Patch{Compilation: boolPtr(false)}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{}, metadataedit.CurrentTags{})
	want := map[string][]string{"COMPILATION": {"0"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_RejectsInvalidDelim(t *testing.T) {
	patch := metadataedit.Patch{Artists: &[]string{"A"}}
	_, err := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{MultiValueArtist: "bogus"}, metadataedit.CurrentTags{})
	if err == nil {
		t.Fatal("expected error on invalid multi-value mode")
	}
}

func TestBuildTagMap_ArtistMBID_AlignsByCurrentNames(t *testing.T) {
	m := map[string]string{"Daft Punk": "id-dp", "Pharrell": "id-ph"}
	patch := metadataedit.Patch{ArtistMBID: &m}
	cur := metadataedit.CurrentTags{
		Artists:     []string{"Daft Punk", "Pharrell"},
		ArtistMBIDs: []string{"", ""},
	}
	got, err := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{}, cur)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"MUSICBRAINZ_ARTISTID": {"id-dp", "id-ph"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_ArtistMBID_KeepsUntouchedNames(t *testing.T) {
	// Only "Pharrell" is in the map; "Daft Punk" keeps its current id.
	m := map[string]string{"Pharrell": "id-ph"}
	patch := metadataedit.Patch{ArtistMBID: &m}
	cur := metadataedit.CurrentTags{
		Artists:     []string{"Daft Punk", "Pharrell"},
		ArtistMBIDs: []string{"id-dp-old", ""},
	}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{}, cur)
	want := map[string][]string{"MUSICBRAINZ_ARTISTID": {"id-dp-old", "id-ph"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_ArtistMBID_ClearAllWritesEmpty(t *testing.T) {
	m := map[string]string{"Solo": ""}
	patch := metadataedit.Patch{ArtistMBID: &m}
	cur := metadataedit.CurrentTags{Artists: []string{"Solo"}, ArtistMBIDs: []string{"id-old"}}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{}, cur)
	want := map[string][]string{"MUSICBRAINZ_ARTISTID": {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_AlbumArtistMBID_BypassesNonePolicy(t *testing.T) {
	m := map[string]string{"A": "id-a", "B": "id-b"}
	patch := metadataedit.Patch{AlbumArtistMBID: &m}
	cur := metadataedit.CurrentTags{AlbumArtists: []string{"A", "B"}, AlbumArtistMBIDs: []string{"", ""}}
	// "none" would normally keep only the first value; MB IDs must ignore it.
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{MultiValueAlbumArtist: "none"}, cur)
	want := map[string][]string{"MUSICBRAINZ_ALBUMARTISTID": {"id-a", "id-b"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_ReleaseIDs_ScalarNoAlignment(t *testing.T) {
	patch := metadataedit.Patch{
		MBReleaseID:      strPtr("rel-id"),
		MBReleaseGroupID: strPtr("rg-id"),
	}
	got, err := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{}, metadataedit.CurrentTags{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"MUSICBRAINZ_ALBUMID":        {"rel-id"},
		"MUSICBRAINZ_RELEASEGROUPID": {"rg-id"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_ReleaseID_ClearWritesEmptyValue(t *testing.T) {
	patch := metadataedit.Patch{MBReleaseID: strPtr("")}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.LibraryCfg{}, metadataedit.CurrentTags{})
	want := map[string][]string{"MUSICBRAINZ_ALBUMID": {""}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
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
		DiscNumber:   intPtr(2),
		DiscSubtitle:     strPtr("Disc Two"),
		Compilation:      boolPtr(true),
		MBReleaseID:      strPtr("rel-uuid"),
		MBReleaseGroupID: strPtr("rg-uuid"),
	}
	cfg := metadataedit.LibraryCfg{MultiValueArtist: "multi", MultiValueAlbumArtist: "multi"}

	if err := metadataedit.WriteMetadata(dst, patch, cfg, metadataedit.CurrentTags{}); err != nil {
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
	if got["DISCNUMBER"][0] != "2" {
		t.Fatalf("disc number round-trip failed: %v", got["DISCNUMBER"])
	}
	if got["DISCSUBTITLE"][0] != "Disc Two" {
		t.Fatalf("disc subtitle round-trip failed: %v", got["DISCSUBTITLE"])
	}
	if got["MUSICBRAINZ_ALBUMID"][0] != "rel-uuid" {
		t.Fatalf("release id round-trip failed: %v", got["MUSICBRAINZ_ALBUMID"])
	}
	if got["MUSICBRAINZ_RELEASEGROUPID"][0] != "rg-uuid" {
		t.Fatalf("release-group id round-trip failed: %v", got["MUSICBRAINZ_RELEASEGROUPID"])
	}
}

func TestWriteMetadata_EmptyPatchIsNoOp(t *testing.T) {
	src := "testdata/empty.flac"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no fixture at %s: %v", src, err)
	}
	dst := filepath.Join(t.TempDir(), "copy.flac")
	copyFileForWriter(t, src, dst)

	if err := metadataedit.WriteMetadata(dst, metadataedit.Patch{}, metadataedit.LibraryCfg{}, metadataedit.CurrentTags{}); err != nil {
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
