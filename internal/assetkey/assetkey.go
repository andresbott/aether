// Package assetkey derives the asset-store key of an entity from its natural
// identity — the same tuple the database uses as its unique index — rather than
// from its autoincrement primary key.
//
// Why: autoincrement ids are positional. They are handed out in the order the
// scanner happens to reconcile tracks, so dropping the DB and rescanning while
// keeping data/metadata/ does not merely orphan images, it MISATTRIBUTES them —
// album 5 is now a different album and inherits the old album 5's cover. A key
// derived from identity is recomputable from the entity at any moment, which
// makes a rebuild correct by construction instead of by reconciliation.
//
// This package is the SINGLE authority on key derivation. Deriving a key
// anywhere else re-creates the defect it exists to prevent: the codebase
// previously carried two copies of artistCoverKey, and a divergence between the
// asset-store key and the imagecache key was already a documented trap.
package assetkey

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/andresbott/aether/internal/model"
)

// keyVersion is part of every hashed input. Bumping it is how a deliberate
// change to identity semantics becomes a detectable key-scheme change rather
// than a silent re-attachment of every stored image.
const keyVersion = "v1"

// keyRe matches the character set assetstore.entityDir accepts.
var keyRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// hashKey builds the key for one kind from its canonical identity components.
// The kind prefix prevents two kinds whose identity strings happen to match
// from sharing a directory; the NUL separator prevents component boundaries
// from being ambiguous ("ab"+"c" must not equal "a"+"bc").
func hashKey(kind string, parts ...string) string {
	h := sha256.New()
	h.Write([]byte(kind))
	h.Write([]byte{0})
	h.Write([]byte(keyVersion))
	for _, p := range parts {
		h.Write([]byte{0})
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// isKeySafe reports whether s is safe for assetstore.entityDir: it matches
// ^[A-Za-z0-9._-]+$ and contains no "..".
func isKeySafe(s string) bool {
	return keyRe.MatchString(s) && !strings.Contains(s, "..")
}

// Album keys on the idx_album_identity tuple. Callers that hold a
// store.AlbumIdentity pass its three normalised components; callers holding a
// row use AlbumOf.
func Album(nameNorm, albumArtistNorm, mbReleaseID string) string {
	return hashKey("album", nameNorm, albumArtistNorm, mbReleaseID)
}

func AlbumOf(a *model.Album) string {
	return Album(a.NameNorm, a.AlbumArtistNorm, a.MBReleaseID)
}

// Artist returns the MusicBrainz ID verbatim when the artist is matched AND the
// MBID is key-safe: that slot is shared with the auto-fetcher
// (app/tasks/artistimage.go), it is already durable across a rebuild, and
// well-formed MBIDs are filesystem-safe. However, MBArtistID arrives straight
// from file tags with no validation (internal/tags/*.go), so a malformed or
// hostile MBID — one with a "/" or ".." in it — would bypass the exact safety
// property hashing exists to provide and silently break that artist's covers.
// When the MBID is not key-safe, or when the artist is unmatched, fall back to
// hashing name_norm.
func Artist(mbArtistID, nameNorm string) string {
	if mbArtistID != "" && isKeySafe(mbArtistID) {
		return mbArtistID
	}
	return hashKey("artist", nameNorm)
}

func ArtistOf(a *model.Artist) string {
	return Artist(a.MBArtistID, a.NameNorm)
}

// Genre hashes the RAW name. Genre names are stored and matched exactly
// (internal/store/genre.go), so normalising here would merge "Rock" and "rock"
// in the asset store while the database keeps them apart — two genres sharing
// one cover directory.
func Genre(name string) string {
	return hashKey("genre", name)
}

func GenreOf(g *model.Genre) string {
	return Genre(g.Name)
}

// Playlist keys on the row's UUID. A playlist is owned by the user rather than
// derived from tags, and a rebuild destroys the playlist itself — so there is
// nothing to re-attach and a surrogate is the correct key. The UUID exists only
// to stop a reused autoincrement id handing a stale cover to a new playlist.
//
// An empty UUID returns the empty string rather than a hash: rows created before
// this change carry UUID = "", and returning a hash would cause every legacy
// playlist to share one cover directory — exactly the collision bug this branch
// exists to prevent. assetstore.entityDir rejects an empty key, so a write
// attempt fails loudly rather than colliding.
func Playlist(uuid string) string {
	if uuid == "" {
		return ""
	}
	return hashKey("playlist", uuid)
}

func PlaylistOf(p *model.Playlist) string {
	return Playlist(p.UUID)
}

// Radio keys on the stream URL, which is the station's natural identity.
func Radio(streamURL string) string {
	return hashKey("radio", streamURL)
}
