// The /api/v0/wishlist shapes: music the server does not have yet, picked from
// MusicBrainz and matched against the catalog by MBID after every scan.

export type WishlistKind = 'artist' | 'album' | 'song'

// The MusicBrainz snapshot an entry is added from — what a search result and an
// add request share. `album` is set for songs, `type` for artists ("Group") and
// albums ("Album · Live"). `mbid` is empty for an entry added by hand, which is
// never matched against the catalog.
export interface WishlistSnapshot {
    kind: WishlistKind
    mbid: string
    title: string
    artist: string
    album: string
    date: string
    type: string
    disambiguation: string
}

// One entry to add: a MusicBrainz pick, or one added by hand (empty `mbid`,
// with `note` as its description).
export interface WishlistAddInput extends WishlistSnapshot {
    note?: string
}

// Where an MBID is in the catalog. `match_id` is the artist, album or track id
// by kind (numeric, not a Subsonic id); `match_album_id` is a song's album.
export interface WishlistMatch {
    in_library: boolean
    match_id?: number
    match_album_id?: number
}

export interface WishlistItem extends WishlistSnapshot, WishlistMatch {
    id: number
    note: string
    found_at: string | null
    created_at: string
}

export interface WishlistSearchResult extends WishlistSnapshot, WishlistMatch {
    score: number
    // The entry already listing this result; absent when none does.
    wishlist_id?: number
}
