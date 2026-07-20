package metadataedit

import (
	"strconv"

	"go.senan.xyz/taglib"
)

// CurrentTags carries the file's existing artist/album-artist names and their
// aligned MusicBrainz IDs, needed to align per-artist MB-ID edits at write
// time. Zero value is fine when the patch touches no MB-ID field.
type CurrentTags struct {
	Artists          []string
	ArtistMBIDs      []string
	AlbumArtists     []string
	AlbumArtistMBIDs []string
}

// Patch describes a tag edit. A nil pointer means "leave this field alone";
// a non-nil pointer (including an empty slice or empty string) means "write
// this value". This is how the per-field "apply" toggle in the UI maps onto
// the wire.
type Patch struct {
	Title           *string
	Album           *string
	Artists         *[]string
	AlbumArtists    *[]string
	Genres          *[]string
	Year            *int
	DiscNumber      *int
	DiscSubtitle    *string
	Compilation     *bool
	ArtistMBID      *map[string]string // artist name -> MBID ("" clears)
	AlbumArtistMBID *map[string]string // album-artist name -> MBID ("" clears)
	// Album MusicBrainz IDs are single-valued scalars (one album per file), so
	// unlike the artist maps they need no positional alignment. "" clears.
	MBReleaseID      *string
	MBReleaseGroupID *string
}

// Empty reports whether the patch would write nothing.
func (p Patch) Empty() bool {
	return p.Title == nil && p.Album == nil && p.Artists == nil &&
		p.AlbumArtists == nil && p.Genres == nil && p.Year == nil && p.DiscNumber == nil &&
		p.DiscSubtitle == nil && p.Compilation == nil &&
		p.ArtistMBID == nil && p.AlbumArtistMBID == nil &&
		p.MBReleaseID == nil && p.MBReleaseGroupID == nil
}

// mergeMBIDs builds an MB-ID list aligned to names: a name present in
// overrides takes the override value; otherwise it keeps its current aligned
// ID (or "" if none). If every resulting value is empty the result is an empty
// slice, which clears the tag.
func mergeMBIDs(names, currentIDs []string, overrides map[string]string) []string {
	out := make([]string, len(names))
	anyNonEmpty := false
	for i, n := range names {
		if v, ok := overrides[n]; ok {
			out[i] = v
		} else if i < len(currentIDs) {
			out[i] = currentIDs[i]
		}
		if out[i] != "" {
			anyNonEmpty = true
		}
	}
	if !anyNonEmpty {
		return []string{}
	}
	return out
}

// BuildTagMap turns a Patch into the tag map expected by taglib.WriteTags.
// Only fields explicitly set in the Patch appear in the result.
// Multi-value fields (artists, album artists, genres) are always written as
// separate values (taglib emits multiple frames where the format allows).
func BuildTagMap(p Patch, cur CurrentTags) (map[string][]string, error) {
	out := map[string][]string{}
	if p.Title != nil {
		out[taglib.Title] = []string{*p.Title}
	}
	if p.Album != nil {
		out[taglib.Album] = []string{*p.Album}
	}
	if p.Artists != nil {
		out[taglib.Artist] = *p.Artists
	}
	if p.AlbumArtists != nil {
		out[taglib.AlbumArtist] = *p.AlbumArtists
	}
	if p.Genres != nil {
		out[taglib.Genre] = *p.Genres
	}
	if p.Year != nil {
		out[taglib.Date] = []string{strconv.Itoa(*p.Year)}
	}
	if p.DiscNumber != nil {
		out[taglib.DiscNumber] = []string{strconv.Itoa(*p.DiscNumber)}
	}
	if p.DiscSubtitle != nil {
		out[taglib.DiscSubtitle] = []string{*p.DiscSubtitle}
	}
	if p.Compilation != nil {
		if *p.Compilation {
			out[taglib.Compilation] = []string{"1"}
		} else {
			out[taglib.Compilation] = []string{"0"}
		}
	}
	if p.ArtistMBID != nil {
		out[taglib.MusicBrainzArtistID] = mergeMBIDs(cur.Artists, cur.ArtistMBIDs, *p.ArtistMBID)
	}
	if p.AlbumArtistMBID != nil {
		out[taglib.MusicBrainzAlbumArtistID] = mergeMBIDs(cur.AlbumArtists, cur.AlbumArtistMBIDs, *p.AlbumArtistMBID)
	}
	if p.MBReleaseID != nil {
		out[taglib.MusicBrainzAlbumID] = []string{*p.MBReleaseID}
	}
	if p.MBReleaseGroupID != nil {
		out[taglib.MusicBrainzReleaseGroupID] = []string{*p.MBReleaseGroupID}
	}
	return out, nil
}

// WriteMetadata applies the given patch to the file at path. Multi-value
// fields are always written as separate frames (where the format allows).
// Absent fields in the patch are left alone (taglib.WriteTags with no Clear
// option is additive per-key). An empty patch is a no-op.
func WriteMetadata(path string, patch Patch, cur CurrentTags) error {
	if patch.Empty() {
		return nil
	}
	tagMap, err := BuildTagMap(patch, cur)
	if err != nil {
		return err
	}
	if len(tagMap) == 0 {
		return nil
	}
	// No Clear flag: only overwrite the keys we provide; leave others intact.
	return taglib.WriteTags(path, tagMap, 0)
}
