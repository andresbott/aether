package model

import "time"

// WishlistKind is what a wishlist entry names on MusicBrainz, and so which
// catalog column a scan matches its MBID against.
type WishlistKind string

const (
	// WishlistArtist is a MusicBrainz artist, matched on artists.mb_artist_id.
	WishlistArtist WishlistKind = "artist"
	// WishlistAlbum is a MusicBrainz release GROUP (the album, whatever the
	// edition), matched on albums.mb_release_group_id.
	WishlistAlbum WishlistKind = "album"
	// WishlistSong is a MusicBrainz recording, matched on tracks.mb_recording_id.
	WishlistSong WishlistKind = "song"
)

// WishlistKinds lists every kind, in display order.
var WishlistKinds = []WishlistKind{WishlistArtist, WishlistAlbum, WishlistSong}

// Valid reports whether k is one of WishlistKinds.
func (k WishlistKind) Valid() bool {
	for _, v := range WishlistKinds {
		if k == v {
			return true
		}
	}
	return false
}

// WishlistItem is music the server does not have yet, usually named by its
// MusicBrainz id. The list is server-wide (it lives on the admin-only /api/v0),
// not per user. Title, Artist, Album, Date, Type and Disambiguation are a
// snapshot of the MusicBrainz search result it was added from, kept so the list
// renders without a MusicBrainz round-trip.
//
// An entry added by hand has no MBID — just a title, its kind and the Note as
// its description — and is never matched by a scan: there is no id to match.
// (kind, mbid) is unique only among entries that have an MBID (the partial
// index idx_wishlist_item, created in Migrate).
//
// MatchID is the catalog row carrying the same MBID — an artist, album or track
// id by Kind — or 0 while none does. Scans keep it current
// (store.ReconcileWishlist); MatchAlbumID is the matched track's album, set for
// songs only, so the UI can link to where the song lives.
type WishlistItem struct {
	ID             uint         `gorm:"primaryKey"`
	Kind           WishlistKind `gorm:"not null"`
	MBID           string       `gorm:"column:mbid;not null;default:''"`
	Title          string       `gorm:"not null"`
	Artist         string
	Album          string
	Date           string
	Type           string
	Disambiguation string
	Note           string
	MatchID        uint `gorm:"not null;default:0"`
	MatchAlbumID   uint `gorm:"not null;default:0"`
	// MatchedAt is when the current match was first seen; nil while unmatched.
	MatchedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Manual reports whether the entry was added by hand, without an MBID.
func (it WishlistItem) Manual() bool { return it.MBID == "" }
