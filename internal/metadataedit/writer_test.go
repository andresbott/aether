package metadataedit_test

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/metadataedit"
	"go.senan.xyz/taglib"
)

func TestBuildTagMap_AppliesOnlyProvidedFields(t *testing.T) {
	patch := metadataedit.Patch{
		Title: strPtr("Hello"),
	}
	got, err := metadataedit.BuildTagMap(patch, metadataedit.CurrentTags{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"TITLE": {"Hello"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_MultiValue_WritesArray(t *testing.T) {
	patch := metadataedit.Patch{Artists: &[]string{"A", "B"}}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.CurrentTags{})
	want := map[string][]string{"ARTIST": {"A", "B"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_GenresWrittenAsIs(t *testing.T) {
	patch := metadataedit.Patch{Genres: &[]string{"Rock", "Jazz"}}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.CurrentTags{})
	want := map[string][]string{"GENRE": {"Rock", "Jazz"}}
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
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.CurrentTags{})
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

func TestBuildTagMap_GenresEmptyListClears(t *testing.T) {
	patch := metadataedit.Patch{Genres: &[]string{}}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.CurrentTags{})
	want := map[string][]string{"GENRE": {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_TrackNumber(t *testing.T) {
	patch := metadataedit.Patch{TrackNumber: intPtr(7)}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.CurrentTags{})
	want := map[string][]string{"TRACKNUMBER": {"7"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_DiscFieldsOnly(t *testing.T) {
	patch := metadataedit.Patch{DiscNumber: intPtr(1), DiscSubtitle: strPtr("Original Score")}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.CurrentTags{})
	want := map[string][]string{"DISCNUMBER": {"1"}, "DISCSUBTITLE": {"Original Score"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_OmitsUnsetDiscFields(t *testing.T) {
	// A patch touching only the title must not emit any disc keys.
	patch := metadataedit.Patch{Title: strPtr("x")}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.CurrentTags{})
	if _, ok := got["DISCNUMBER"]; ok {
		t.Fatalf("unexpected DISCNUMBER in %v", got)
	}
	if _, ok := got["DISCSUBTITLE"]; ok {
		t.Fatalf("unexpected DISCSUBTITLE in %v", got)
	}
}

func TestBuildTagMap_CompilationFalseWritesZero(t *testing.T) {
	patch := metadataedit.Patch{Compilation: boolPtr(false)}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.CurrentTags{})
	want := map[string][]string{"COMPILATION": {"0"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_ArtistMBID_AlignsByCurrentNames(t *testing.T) {
	m := map[string]string{"Daft Punk": "id-dp", "Pharrell": "id-ph"}
	patch := metadataedit.Patch{ArtistMBID: &m}
	cur := metadataedit.CurrentTags{
		Artists:     []string{"Daft Punk", "Pharrell"},
		ArtistMBIDs: []string{"", ""},
	}
	got, err := metadataedit.BuildTagMap(patch, cur)
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
	got, _ := metadataedit.BuildTagMap(patch, cur)
	want := map[string][]string{"MUSICBRAINZ_ARTISTID": {"id-dp-old", "id-ph"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_ArtistMBID_ClearAllWritesEmpty(t *testing.T) {
	m := map[string]string{"Solo": ""}
	patch := metadataedit.Patch{ArtistMBID: &m}
	cur := metadataedit.CurrentTags{Artists: []string{"Solo"}, ArtistMBIDs: []string{"id-old"}}
	got, _ := metadataedit.BuildTagMap(patch, cur)
	want := map[string][]string{"MUSICBRAINZ_ARTISTID": {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_AlbumArtistMBID_WritesMultiValue(t *testing.T) {
	m := map[string]string{"A": "id-a", "B": "id-b"}
	patch := metadataedit.Patch{AlbumArtistMBID: &m}
	cur := metadataedit.CurrentTags{AlbumArtists: []string{"A", "B"}, AlbumArtistMBIDs: []string{"", ""}}
	got, _ := metadataedit.BuildTagMap(patch, cur)
	want := map[string][]string{"MUSICBRAINZ_ALBUMARTISTID": {"id-a", "id-b"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_ReleaseIDs_ScalarNoAlignment(t *testing.T) {
	patch := metadataedit.Patch{
		MBRecordingID:    strPtr("rec-id"),
		MBReleaseID:      strPtr("rel-id"),
		MBReleaseGroupID: strPtr("rg-id"),
	}
	got, err := metadataedit.BuildTagMap(patch, metadataedit.CurrentTags{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"MUSICBRAINZ_TRACKID":        {"rec-id"},
		"MUSICBRAINZ_ALBUMID":        {"rel-id"},
		"MUSICBRAINZ_RELEASEGROUPID": {"rg-id"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_RawWritesAndDeletes(t *testing.T) {
	raw := map[string][]string{
		"custom_field":          {"one", "two"},
		"REPLAYGAIN_TRACK_GAIN": {},
	}
	got, err := metadataedit.BuildTagMap(
		metadataedit.Patch{Raw: &raw}, metadataedit.CurrentTags{},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		// Keys are normalized to upper case; empty slice deletes the key.
		"CUSTOM_FIELD":          {"one", "two"},
		"REPLAYGAIN_TRACK_GAIN": {},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_RawRejectsManagedKey(t *testing.T) {
	for _, key := range []string{"TITLE", "artist", "Musicbrainz_TrackID"} {
		raw := map[string][]string{key: {"x"}}
		_, err := metadataedit.BuildTagMap(
			metadataedit.Patch{Raw: &raw}, metadataedit.CurrentTags{},
		)
		if err == nil {
			t.Errorf("expected error for managed key %q", key)
		}
	}
}

func TestBuildTagMap_RecordingID_ClearWritesEmptyValue(t *testing.T) {
	patch := metadataedit.Patch{MBRecordingID: strPtr("")}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.CurrentTags{})
	want := map[string][]string{"MUSICBRAINZ_TRACKID": {""}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBuildTagMap_ReleaseID_ClearWritesEmptyValue(t *testing.T) {
	patch := metadataedit.Patch{MBReleaseID: strPtr("")}
	got, _ := metadataedit.BuildTagMap(patch, metadataedit.CurrentTags{})
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
		Title:            strPtr("Round Trip"),
		Album:            strPtr("Test Album"),
		Artists:          &[]string{"First", "Second"},
		AlbumArtists:     &[]string{"Various"},
		Year:             intPtr(1999),
		DiscNumber:       intPtr(2),
		DiscSubtitle:     strPtr("Disc Two"),
		Compilation:      boolPtr(true),
		MBRecordingID:    strPtr("rec-uuid"),
		MBReleaseID:      strPtr("rel-uuid"),
		MBReleaseGroupID: strPtr("rg-uuid"),
	}
	if err := metadataedit.WriteMetadata(dst, patch, metadataedit.CurrentTags{}); err != nil {
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
	if got["MUSICBRAINZ_TRACKID"][0] != "rec-uuid" {
		t.Fatalf("recording id round-trip failed: %v", got["MUSICBRAINZ_TRACKID"])
	}
	if got["MUSICBRAINZ_ALBUMID"][0] != "rel-uuid" {
		t.Fatalf("release id round-trip failed: %v", got["MUSICBRAINZ_ALBUMID"])
	}
	if got["MUSICBRAINZ_RELEASEGROUPID"][0] != "rg-uuid" {
		t.Fatalf("release-group id round-trip failed: %v", got["MUSICBRAINZ_RELEASEGROUPID"])
	}
}

func TestWriteMetadata_RemoveUnsupportedFrames(t *testing.T) {
	// hidden.mp3 carries PRIV and GEOB frames, invisible to the tag map.
	src := "testdata/hidden.mp3"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no fixture at %s: %v", src, err)
	}
	dst := filepath.Join(t.TempDir(), "copy.mp3")
	copyFileForWriter(t, src, dst)

	before, err := taglib.ReadUnsupported(dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) < 2 {
		t.Fatalf("fixture should carry hidden frames, got %v", before)
	}

	// Remove one frame and write a title in the same patch.
	var priv []string
	for _, d := range before {
		if strings.HasPrefix(d, "PRIV") {
			priv = append(priv, d)
		}
	}
	if len(priv) == 0 {
		t.Fatalf("fixture should carry a PRIV frame, got %v", before)
	}
	patch := metadataedit.Patch{Title: strPtr("Cleaned"), RemoveUnsupported: &priv}
	if err := metadataedit.WriteMetadata(dst, patch, metadataedit.CurrentTags{}); err != nil {
		t.Fatalf("WriteMetadata: %v", err)
	}

	after, err := taglib.ReadUnsupported(dst)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range after {
		if strings.HasPrefix(d, "PRIV") {
			t.Fatalf("PRIV frame survived removal: %v", after)
		}
	}
	if len(after) != len(before)-len(priv) {
		t.Fatalf("only PRIV should be gone: before %v after %v", before, after)
	}
	got, err := taglib.ReadTags(dst)
	if err != nil {
		t.Fatal(err)
	}
	if got["TITLE"][0] != "Cleaned" {
		t.Fatalf("title write alongside removal failed: %v", got["TITLE"])
	}
}

func TestWriteMetadata_RemoveUnsupportedOnly(t *testing.T) {
	// A patch with only hidden-frame removals (no tag map keys) must still
	// apply and not be treated as empty.
	src := "testdata/hidden.mp3"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no fixture at %s: %v", src, err)
	}
	dst := filepath.Join(t.TempDir(), "copy.mp3")
	copyFileForWriter(t, src, dst)

	before, err := taglib.ReadUnsupported(dst)
	if err != nil {
		t.Fatal(err)
	}
	patch := metadataedit.Patch{RemoveUnsupported: &before}
	if patch.Empty() {
		t.Fatal("patch with removals must not report Empty")
	}
	if err := metadataedit.WriteMetadata(dst, patch, metadataedit.CurrentTags{}); err != nil {
		t.Fatalf("WriteMetadata: %v", err)
	}
	after, err := taglib.ReadUnsupported(dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 0 {
		t.Fatalf("expected no hidden frames left, got %v", after)
	}
}

func TestWriteMetadata_EmptyPatchIsNoOp(t *testing.T) {
	src := "testdata/empty.flac"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no fixture at %s: %v", src, err)
	}
	dst := filepath.Join(t.TempDir(), "copy.flac")
	copyFileForWriter(t, src, dst)

	if err := metadataedit.WriteMetadata(dst, metadataedit.Patch{}, metadataedit.CurrentTags{}); err != nil {
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
