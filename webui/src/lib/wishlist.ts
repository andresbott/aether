import type { RouteLocationRaw } from 'vue-router'
import type { WishlistKind, WishlistMatch, WishlistSnapshot } from '@/types/wishlist'

export interface WishlistKindOption {
    value: WishlistKind
    label: string
    plural: string
    // A literal `ms-*` class, where the icon build finds it.
    icon: string
    // What MusicBrainz calls the entity, for its URLs.
    mbEntity: 'artist' | 'release-group' | 'recording'
}

export const WISHLIST_KINDS: readonly WishlistKindOption[] = [
    { value: 'album', label: 'Album', plural: 'Albums', icon: 'ms-album', mbEntity: 'release-group' },
    { value: 'artist', label: 'Artist', plural: 'Artists', icon: 'ms-person', mbEntity: 'artist' },
    { value: 'song', label: 'Song', plural: 'Songs', icon: 'ms-music-note', mbEntity: 'recording' }
]

export function wishlistKind(kind: WishlistKind): WishlistKindOption {
    return WISHLIST_KINDS.find((k) => k.value === kind) ?? WISHLIST_KINDS[0]
}

export function isWishlistKind(v: unknown): v is WishlistKind {
    return v === 'artist' || v === 'album' || v === 'song'
}

// The Subsonic id of the library entity a found entry matched — what the music
// views are addressed by. The API answers numeric catalog ids; a song has no
// page of its own, so it resolves to the album it is on. Null while not found.
export function wishlistLibraryId(m: WishlistMatch & { kind: WishlistKind }): string | null {
    if (!m.in_library || !m.match_id) return null
    switch (m.kind) {
        case 'artist':
            return `ar-${m.match_id}`
        case 'album':
            return `al-${m.match_id}`
        case 'song':
            return m.match_album_id ? `al-${m.match_album_id}` : null
    }
}

// Where a found entry lives in the library; null while not found.
export function wishlistLibraryRoute(m: WishlistMatch & { kind: WishlistKind }): RouteLocationRaw | null {
    const id = wishlistLibraryId(m)
    if (!id) return null
    return { name: m.kind === 'artist' ? 'artist' : 'album', params: { id } }
}

// The label of the action opening a found entry in the library.
export function wishlistLibraryLabel(kind: WishlistKind): string {
    return kind === 'artist' ? 'Open artist' : kind === 'album' ? 'Open album' : 'Open on its album'
}

// The entry's MusicBrainz page; null for an entry added by hand, which has none.
export function musicBrainzUrl(kind: WishlistKind, mbid: string): string | null {
    if (!mbid) return null
    return `https://musicbrainz.org/${wishlistKind(kind).mbEntity}/${encodeURIComponent(mbid)}`
}

// Whether an entry was added by hand: it has no MBID, so no scan can match it.
export function isManualEntry(s: Pick<WishlistSnapshot, 'mbid'>): boolean {
    return !s.mbid
}

// The year of a MusicBrainz date ("1991-09-24" → "1991"); empty when unknown.
export function yearOf(date: string): string {
    const m = /^(\d{4})/.exec(date ?? '')
    return m ? m[1] : ''
}

// The line naming who made an entry and where it is from: "Nirvana · Nevermind"
// for a song, the artist for an album, nothing for an artist.
export function wishlistSubtitle(s: WishlistSnapshot): string {
    return [s.artist, s.kind === 'song' ? s.album : ''].filter(Boolean).join(' · ')
}

// The lighter facts under it: kind or MusicBrainz types, year, disambiguation
// — or, for an entry added by hand, its kind and that it was.
// The server joins a release group's types with " · " ("Album · Live"); they
// are split back so each is a part of its own, separated like the rest.
export function wishlistDetails(s: WishlistSnapshot): string[] {
    if (isManualEntry(s)) return [wishlistKind(s.kind).label, 'Added by hand']
    const types = s.type ? s.type.split(' · ') : [wishlistKind(s.kind).label]
    return [...types, yearOf(s.date), s.disambiguation].filter((p) => p !== '')
}
