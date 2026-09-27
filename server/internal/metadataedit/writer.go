package metadataedit

import (
	"fmt"
	"strconv"
	"strings"

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
	ReleaseTypes    *[]string
	Year            *int
	TrackNumber     *int
	DiscNumber      *int
	DiscSubtitle    *string
	Compilation     *bool
	ArtistMBID      *map[string]string // artist name -> MBID ("" clears)
	AlbumArtistMBID *map[string]string // album-artist name -> MBID ("" clears)
	// Recording/album MusicBrainz IDs are single-valued scalars (one recording,
	// one album per file), so unlike the artist maps they need no positional
	// alignment. "" clears.
	MBRecordingID    *string
	MBReleaseID      *string
	MBReleaseGroupID *string
	// Raw carries free-form tag edits from the raw editor: key -> values,
	// where an empty slice deletes the key. Keys the structured editor owns
	// (IsManagedTag) are rejected by BuildTagMap.
	Raw *map[string][]string
	// RemoveUnsupported lists descriptors of hidden frames to delete —
	// metadata the tag map cannot represent as text (ID3v2 PRIV/GEOB/POPM,
	// unknown binary frames), as returned by taglib.ReadUnsupported.
	// Descriptors not present in a file are ignored.
	RemoveUnsupported *[]string
}

// Empty reports whether the patch would write nothing.
func (p Patch) Empty() bool {
	return p.Title == nil && p.Album == nil && p.Artists == nil &&
		p.AlbumArtists == nil && p.Genres == nil && p.ReleaseTypes == nil && p.Year == nil &&
		p.TrackNumber == nil && p.DiscNumber == nil &&
		p.DiscSubtitle == nil && p.Compilation == nil &&
		p.ArtistMBID == nil && p.AlbumArtistMBID == nil &&
		p.MBRecordingID == nil && p.MBReleaseID == nil && p.MBReleaseGroupID == nil &&
		p.Raw == nil && p.RemoveUnsupported == nil
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
	// Single-string fields share one shape: write [key] = [value] when set.
	for _, f := range []struct {
		val *string
		key string
	}{
		{p.Title, taglib.Title},
		{p.Album, taglib.Album},
		{p.DiscSubtitle, taglib.DiscSubtitle},
		{p.MBRecordingID, taglib.MusicBrainzTrackID},
		{p.MBReleaseID, taglib.MusicBrainzAlbumID},
		{p.MBReleaseGroupID, taglib.MusicBrainzReleaseGroupID},
	} {
		if f.val != nil {
			out[f.key] = []string{*f.val}
		}
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
	if p.ReleaseTypes != nil {
		// RELEASETYPE is taglib's name for the MusicBrainz album type and maps to
		// each format's native frame (TXXX:MusicBrainz Album Type on MP3, the
		// iTunes freeform atom on MP4) — what Picard and other taggers read. The
		// MUSICBRAINZ_ALBUMTYPE alias, which the readers fall back to, is deleted
		// in the same write: a leftover would compete with the edit and bring a
		// cleared list back on the next rescan.
		out[taglib.ReleaseType] = *p.ReleaseTypes
		out["MUSICBRAINZ_ALBUMTYPE"] = []string{}
	}
	if p.Year != nil {
		out[taglib.Date] = []string{strconv.Itoa(*p.Year)}
	}
	if p.TrackNumber != nil {
		out[taglib.TrackNumber] = []string{strconv.Itoa(*p.TrackNumber)}
	}
	if p.DiscNumber != nil {
		out[taglib.DiscNumber] = []string{strconv.Itoa(*p.DiscNumber)}
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
	if p.Raw != nil {
		for key, vals := range *p.Raw {
			if IsManagedTag(key) {
				return nil, fmt.Errorf("raw edit of managed tag %q; use the structured fields", key)
			}
			// Empty slice deletes the key (taglib removes keys written with
			// no values, matching the mergeMBIDs clear behavior above).
			out[strings.ToUpper(strings.TrimSpace(key))] = vals
		}
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
	// Build (and thereby validate) the tag map before mutating anything, so a
	// patch that fails validation never touches the file. This is validation
	// atomicity only; the two write passes below are NOT atomic (see below).
	tagMap, err := BuildTagMap(patch, cur)
	if err != nil {
		return err
	}
	// WriteTags and RemoveUnsupported are two independent taglib passes — the
	// fork exposes no combined call — so a mid-way error (or the process dying
	// between them) leaves the file partially edited. They touch disjoint
	// frames: WriteTags only rewrites the text/property frames it is given,
	// while RemoveUnsupported only deletes the hidden binary frames
	// (PRIV/GEOB/POPM/unknown) named by the descriptors, which the tag map
	// cannot represent. Because the sets are disjoint the order does not change
	// the result on the success path. We deliberately run the destructive
	// RemoveUnsupported LAST so that when a per-row failure is reported to the
	// user the file is left with the structured edit APPLIED rather than with
	// hidden frames stripped and the edit missing — the least-surprising state
	// to be in when acting on that error.
	if len(tagMap) > 0 {
		// No Clear flag: only overwrite the keys we provide; leave others intact.
		if err := taglib.WriteTags(path, tagMap, 0); err != nil {
			return err
		}
	}
	if patch.RemoveUnsupported != nil && len(*patch.RemoveUnsupported) > 0 {
		if err := taglib.RemoveUnsupported(path, *patch.RemoveUnsupported); err != nil {
			return fmt.Errorf("remove hidden frames: %w", err)
		}
	}
	return nil
}
