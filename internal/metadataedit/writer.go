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
		p.AlbumArtists == nil && p.Genres == nil && p.Year == nil &&
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

// LibraryCfg carries the per-library multi-value settings that affect tag
// serialization.
type LibraryCfg struct {
	MultiValueArtist      string
	MultiValueAlbumArtist string
	MultiValueGenre       string
}

// BuildTagMap turns a Patch into the tag map expected by taglib.WriteTags.
// Only fields explicitly set in the Patch appear in the result.
func BuildTagMap(p Patch, cfg LibraryCfg, cur CurrentTags) (map[string][]string, error) {
	out := map[string][]string{}
	if p.Title != nil {
		out[taglib.Title] = []string{*p.Title}
	}
	if p.Album != nil {
		out[taglib.Album] = []string{*p.Album}
	}
	if p.Artists != nil {
		vals, err := serializeMulti(*p.Artists, cfg.MultiValueArtist)
		if err != nil {
			return nil, fmt.Errorf("artists: %w", err)
		}
		out[taglib.Artist] = vals
	}
	if p.AlbumArtists != nil {
		vals, err := serializeMulti(*p.AlbumArtists, cfg.MultiValueAlbumArtist)
		if err != nil {
			return nil, fmt.Errorf("album_artists: %w", err)
		}
		out[taglib.AlbumArtist] = vals
	}
	if p.Genres != nil {
		vals, err := serializeMulti(*p.Genres, cfg.MultiValueGenre)
		if err != nil {
			return nil, fmt.Errorf("genres: %w", err)
		}
		out[taglib.Genre] = vals
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
	if p.MBRecordingID != nil {
		out[taglib.MusicBrainzTrackID] = []string{*p.MBRecordingID}
	}
	if p.MBReleaseID != nil {
		out[taglib.MusicBrainzAlbumID] = []string{*p.MBReleaseID}
	}
	if p.MBReleaseGroupID != nil {
		out[taglib.MusicBrainzReleaseGroupID] = []string{*p.MBReleaseGroupID}
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

// serializeMulti applies the library's multi-value policy to a list.
//
// Modes mirror the library config:
//   - "" or "multi": return the list as-is (taglib emits multiple frames
//     where the format allows, otherwise a single joined string).
//   - "delim <sep>": join with <sep> into one value.
//   - "none": keep only the first element.
func serializeMulti(vals []string, mode string) ([]string, error) {
	switch {
	case mode == "" || mode == "multi":
		return vals, nil
	case mode == "none":
		if len(vals) == 0 {
			return []string{}, nil
		}
		return []string{vals[0]}, nil
	case strings.HasPrefix(mode, "delim "):
		sep := strings.TrimPrefix(mode, "delim ")
		return []string{strings.Join(vals, sep)}, nil
	default:
		return nil, fmt.Errorf("unknown multi-value mode %q", mode)
	}
}

// WriteMetadata applies the given patch to the file at path, using cfg to
// decide how multi-value fields are serialized. Absent fields in the patch
// are left alone (taglib.WriteTags with no Clear option is additive per-key).
// An empty patch is a no-op.
func WriteMetadata(path string, patch Patch, cfg LibraryCfg, cur CurrentTags) error {
	if patch.Empty() {
		return nil
	}
	// Build (and thereby validate) the tag map before mutating anything, so
	// an invalid patch never half-applies.
	tagMap, err := BuildTagMap(patch, cfg, cur)
	if err != nil {
		return err
	}
	if patch.RemoveUnsupported != nil && len(*patch.RemoveUnsupported) > 0 {
		if err := taglib.RemoveUnsupported(path, *patch.RemoveUnsupported); err != nil {
			return fmt.Errorf("remove hidden frames: %w", err)
		}
	}
	if len(tagMap) == 0 {
		return nil
	}
	// No Clear flag: only overwrite the keys we provide; leave others intact.
	return taglib.WriteTags(path, tagMap, 0)
}
