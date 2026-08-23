// internal/scanner/albumidentity.go
package scanner

import (
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
	"github.com/andresbott/aether/internal/unidecode"
)

// Fallbacks applied when the corresponding tag is empty. They are part of the
// identity contract: two files with no album tag land in the same "Unknown
// Album" row, so changing a value here re-buckets existing libraries.
const (
	unknownArtistName = "Unknown Artist"
	variousArtistName = "Various Artists"
	unknownAlbumName  = "Unknown Album"
)

// TrackArtistNames returns the track's artist credit with the scanner's
// fallback applied. Multi-value frames arrive from tags.Reader already split,
// so they are taken as-is.
func TrackArtistNames(meta tags.Metadata) []string {
	names := nonEmpty(meta.Artist)
	if len(names) == 0 {
		return []string{unknownArtistName}
	}
	return names
}

// AlbumArtistNames returns the album-artist credit with the scanner's
// fallbacks applied: an untagged compilation is credited to "Various Artists",
// anything else to the track artists.
func AlbumArtistNames(meta tags.Metadata) []string {
	names := nonEmpty(meta.AlbumArtist)
	if len(names) > 0 {
		return names
	}
	if meta.Compilation {
		return []string{variousArtistName}
	}
	return TrackArtistNames(meta)
}

// AlbumIdentityOf is the single authority on which album row a set of tags
// belongs to. reconcileTrack resolves the album through it, and
// planAlbumContinuity predicts the album a batch is moving to with it — the two
// MUST agree, or a retag would rename one row and create another. See
// docs/agents/scanning.md ("Identity & normalization rules").
func AlbumIdentityOf(meta tags.Metadata) store.AlbumIdentity {
	name := meta.Album
	if name == "" {
		name = unknownAlbumName
	}
	return store.AlbumIdentity{
		Name:            name,
		NameNorm:        unidecode.Normalize(name),
		AlbumArtistNorm: unidecode.Normalize(AlbumArtistNames(meta)[0]),
		MBReleaseID:     meta.MBReleaseID,
	}
}
