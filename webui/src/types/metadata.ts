export interface Folder {
    name: string
    path: string
    has_subfolders: boolean
}

export interface ListFoldersResponse {
    folders: Folder[]
}

export interface Track {
    path: string
    name: string
    title: string
    artists: string[]
    album_artists: string[]
    album: string
    genres: string[]
    year: number
    track_number: number
    disc_number: number
    disc_subtitle: string
    compilation: boolean
    mb_artist_ids: string[]
    mb_album_artist_ids: string[]
    mb_recording_id: string
    mb_release_id: string
    mb_release_group_id: string
    error?: string
}

export interface ListTracksResponse {
    tracks: Track[]
}

// Only fields with a defined value (not undefined) are sent over the wire;
// JSON.stringify omits undefined keys, which is exactly the "apply" semantics
// the server expects.
export interface PatchFields {
    title?: string
    album?: string
    artists?: string[]
    album_artists?: string[]
    genres?: string[]
    year?: number
    track_number?: number
    disc_number?: number
    disc_subtitle?: string
    compilation?: boolean
    artist_mbids?: Record<string, string>
    album_artist_mbids?: Record<string, string>
    mb_recording_id?: string
    mb_release_id?: string
    mb_release_group_id?: string
    // Free-form raw tag edits: key -> values, empty array deletes the key.
    raw_tags?: Record<string, string[]>
    // Hidden-frame descriptors to delete, as returned in RawTagsResult.unsupported.
    remove_unsupported?: string[]
}

// ----- Raw tag editor -----

// RawTags is a file's complete tag map as read from disk: every key with its
// value list, unfiltered.
export type RawTags = Record<string, string[]>

export interface RawTagsResult {
    path: string
    tags: RawTags
    // Hidden-frame descriptors: metadata the tag map cannot represent as text
    // (ID3v2 PRIV/GEOB/POPM, unknown binary frames). Sent back verbatim in
    // PatchFields.remove_unsupported to delete the frames.
    unsupported: string[]
    error?: string
}

export interface RawTagsResponse {
    results: RawTagsResult[]
}

// Raw tag keys owned by the structured editor (upper-cased) — read-only in
// the raw editor, which shows them with a "managed" indicator. Mirrors
// internal/metadataedit.IsManagedTag on the server.
export const MANAGED_TAG_KEYS: ReadonlySet<string> = new Set([
    'TITLE',
    'ALBUM',
    'ARTIST',
    'ALBUMARTIST',
    'ALBUM_ARTIST',
    'GENRE',
    'DATE',
    'YEAR',
    'ORIGINALDATE',
    'TRACKNUMBER',
    'TRACK',
    'DISCNUMBER',
    'DISC',
    'DISCSUBTITLE',
    'TSST',
    'COMPILATION',
    'TCMP',
    'MUSICBRAINZ_TRACKID',
    'MUSICBRAINZ_ALBUMID',
    'MUSICBRAINZ_RELEASEGROUPID',
    'MUSICBRAINZ_ARTISTID',
    'MUSICBRAINZ_ALBUMARTISTID'
])

export function isManagedTag(key: string): boolean {
    return MANAGED_TAG_KEYS.has(key.trim().toUpperCase())
}

// One credited artist: the credited-as name and the artist's MusicBrainz ID.
export interface ArtistCredit {
    name: string
    mbid: string
}

// TrackOverlay is one track's staged (unsaved) edits. A key PRESENT in the
// overlay means "this field is staged"; its absence means "keep the original".
// Artist lists are staged as full replacement lists with names and MBIDs
// aligned per credit.
export interface TrackOverlay {
    title?: string
    album?: string
    mb_recording_id?: string
    mb_release_id?: string
    mb_release_group_id?: string
    genres?: string[]
    year?: number
    track_number?: number
    disc_number?: number
    disc_subtitle?: string
    compilation?: boolean
    artists?: ArtistCredit[]
    album_artists?: ArtistCredit[]
    // Staged raw tag edits: key -> values; an empty array stages a delete.
    raw?: Record<string, string[]>
    // Hidden-frame descriptors staged for deletion (from RawTagsResult.unsupported).
    removeUnsupported?: string[]
}

// ----- Audio identification (fingerprint) -----

export interface MetadataCapabilities {
    identify: boolean
    // Why identification is off (missing fpcalc binary, missing AcoustID key).
    // Only sent when identify is false; shown to the user on the disabled
    // Identify button so a missing server dependency is discoverable.
    identify_unavailable_reason?: string
}

// One release (album) an identified recording appears on. track_number /
// disc_number locate the recording on the release (0 = unknown).
export interface IdentifyRelease {
    release_mbid: string
    release_group_mbid: string
    album: string
    year: number
    track_number: number
    disc_number: number
}

// One MusicBrainz recording matched by acoustic fingerprint.
export interface IdentifyCandidate {
    score: number
    recording_mbid: string
    title: string
    artists: ArtistCredit[]
    releases: IdentifyRelease[]
}

export interface IdentifyTrackResult {
    path: string
    candidates: IdentifyCandidate[]
    error?: string
}

// One confirmed identify pick from the review dialog: the accepted candidate
// and the release the user chose (null when the candidate has no releases).
//
// `genres` are the release group's genres, looked up by the dialog rather than
// returned by identification (AcoustID carries none). Empty when the pick has no
// release group, when the lookup failed, or when MusicBrainz holds no genres for
// it — in every one of those cases the apply stages no genres at all.
export interface IdentifyPick {
    path: string
    candidate: IdentifyCandidate
    release: IdentifyRelease | null
    genres: string[]
}

export interface IdentifyRequest {
    library_id: number
    paths: string[]
}

export interface IdentifyResponse {
    results: IdentifyTrackResult[]
}

// ----- Album identification (map a selection onto one release) -----

// How a file ended up on a track position of an album option. Mirrors
// internal/albumidentify's Source* constants.
export type AlbumAssignmentSource = 'fingerprint' | 'inferred' | 'none'

// One position on a release's tracklist.
export interface AlbumTrackSlot {
    disc_number: number
    track_number: number
    title: string
    recording_mbid: string
    duration_seconds: number
}

// One selected file's placement on one album option. `error` is set instead of
// a placement when the file could not be fingerprinted at all.
export interface AlbumAssignment {
    path: string
    source: AlbumAssignmentSource
    title: string
    recording_mbid: string
    artists: ArtistCredit[]
    disc_number: number
    track_number: number
    score: number
    error?: string
}

// One candidate release for the whole selection. `enriched` false means the
// MusicBrainz tracklist lookup did not happen or failed, so track_count,
// disc_count and tracks are unknown.
export interface AlbumOption {
    release_mbid: string
    release_group_mbid: string
    album: string
    year: number
    artists: ArtistCredit[]
    track_count: number
    disc_count: number
    enriched: boolean
    matched_count: number
    mean_score: number
    assignments: AlbumAssignment[]
    tracks: AlbumTrackSlot[]
}

export interface IdentifyAlbumRequest {
    library_id: number
    paths: string[]
}

export interface IdentifyAlbumResponse {
    options: AlbumOption[]
    // Files this identification did not cover, of both kinds: paths the server
    // refused before identification (outside the library) and files that reached
    // the resolver but could not be fingerprinted or looked up. `error` is a
    // short user-facing reason, never a raw server error.
    errors: { path: string; error: string }[]
}

// One song's confirmed placement from the album dialog: the album the user
// picked plus the position they accepted for this file (null = album-level
// fields only). `genres` are the chosen option's release-group genres, looked up
// by the dialog — see IdentifyPick.
export interface AlbumIdentifyPick {
    path: string
    option: AlbumOption
    assignment: AlbumAssignment | null
    genres: string[]
}

export interface UpdateTracksRequest {
    library_id: number
    paths: string[]
    fields: PatchFields
}

export interface UpdateResult {
    path: string
    ok: boolean
    error?: string
}

// The outcome of the server-side re-index that runs after a write. A failure is
// not a write failure: the tags are on disk, the library index just lags.
export interface RescanStatus {
    ok: boolean
    error?: string
}

export interface UpdateTracksResponse {
    results: UpdateResult[]
    rescan?: RescanStatus
}

// A cover candidate returned by the Cover Art Archive lookup.
export interface CoverCandidate {
    id: string
    thumbUrl: string
    imageUrl: string
    isFront: boolean
    // What the image depicts, e.g. ['Front'], ['Back'], ['Booklet'], ['Medium'].
    types: string[]
    // The uploader's free-text note, if any.
    comment: string
}

// Where an attached picture is stored. Both are on disk: the metadata editor
// edits nothing else. Manual album covers held in aether's own store are set
// from the album view (the /rest updateAlbum extension), not here.
// - 'embedded': in the tags of the selected tracks (primary)
// - 'folder': an art file in the album folder (e.g. cover.jpg, back.jpg)
export type PictureSlot = 'embedded' | 'folder'

// One occupied slot of a picture type, as reported by the pictures endpoint.
export interface PictureSlotInfo {
    slot: PictureSlot
    // e.g. "back.jpg" for folder slots, "4 of 10 files" for embedded slots.
    detail?: string
    // Folder slots only: the album spans several directories (a multi-disc
    // release) and they do not all hold the same image — one is missing it or
    // carries a different one. The editor shows the first folder's image and
    // warns; saving writes the picture into every folder.
    mixed?: boolean
}

// One picture type present somewhere for the folder, with its occupied slots.
export interface PictureInfo {
    type: string
    slots: PictureSlotInfo[]
}

export interface PicturesResponse {
    pictures: PictureInfo[]
}

export interface ApplyPictureResult {
    ok: boolean
    target: PictureSlot
    type: string
    rescan?: RescanStatus
}

export interface DeletePictureResult {
    ok: boolean
    rescan?: RescanStatus
}

// An image chosen in the picker but not yet persisted: it previews in the
// editor and is only written when the user clicks Save. Exactly one of
// file / imageUrl is set (a local upload vs. a Cover Art Archive URL).
export interface StagedPictureSource {
    file: File | null
    imageUrl: string | null
}

// An image this album already has in another type+slot cell, offered in the
// picker as a copy source (e.g. copy the embedded front cover into the
// internal store). Exactly one of file / imageUrl / fetchUrl resolves it:
// - file:     an upload already staged in this session
// - imageUrl: a Cover Art Archive URL already staged in this session
// - fetchUrl: an image the server holds; the browser fetches the bytes and
//             stages them as a file, so the server never re-reads its own API
export interface PictureCopySource {
    key: string
    // e.g. "Front cover — embedded in file"
    label: string
    // e.g. "cover.jpg", "2 of 5 files", "pending change"
    detail: string
    thumbUrl: string
    file: File | null
    imageUrl: string | null
    fetchUrl: string | null
}
