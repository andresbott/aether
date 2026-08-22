import { apiClient } from '@/lib/api/client'
import type {
    ApplyPictureResult,
    CoverCandidate,
    DeletePictureResult,
    IdentifyAlbumRequest,
    IdentifyAlbumResponse,
    IdentifyRequest,
    IdentifyResponse,
    ListFoldersResponse,
    ListTracksResponse,
    MetadataCapabilities,
    PictureInfo,
    PictureSlot,
    PicturesResponse,
    RawTagsResponse,
    UpdateTracksRequest,
    UpdateTracksResponse
} from '@/types/metadata'

export async function listFolders(libraryId: number, path: string) {
    const { data } = await apiClient.get<ListFoldersResponse>('/metadata/folders', {
        params: { library_id: libraryId, path }
    })
    return data.folders
}

export async function listTracks(libraryId: number, path: string) {
    const { data } = await apiClient.get<ListTracksResponse>('/metadata/tracks', {
        params: { library_id: libraryId, path }
    })
    return data.tracks
}

// updateTracks returns the whole response, not just the per-path results: the
// server also reports whether its post-write re-index succeeded.
export async function updateTracks(body: UpdateTracksRequest): Promise<UpdateTracksResponse> {
    const { data } = await apiClient.put<UpdateTracksResponse>('/metadata/tracks', body)
    return data
}

// getRawTags reads the complete tag map of the given files, including keys
// the structured editor does not manage.
//
// The selection travels in the POST body rather than the URL: a large
// multi-disc selection as a repeated ?paths= query param overflowed a
// production reverse proxy's header buffer (HTTP 431) — the same fix as
// getPictures below.
export async function getRawTags(libraryId: number, paths: string[]) {
    const { data } = await apiClient.post<RawTagsResponse>('/metadata/tracks/raw-tags', {
        library_id: libraryId,
        paths
    })
    return data.results
}

export async function getMetadataCapabilities() {
    const { data } = await apiClient.get<MetadataCapabilities>('/metadata/capabilities')
    return data
}

// identifyTracks resolves the given files to MusicBrainz recording candidates
// by acoustic fingerprint. Slow: each path costs a fingerprint run plus a
// rate-limited AcoustID call on the server.
// signal aborts the request: the server fingerprints the paths in a loop driven
// by the request context, so cancelling stops it part-way instead of running the
// remaining files for a response nobody will read.
export async function identifyTracks(body: IdentifyRequest, signal?: AbortSignal) {
    const { data } = await apiClient.post<IdentifyResponse>('/metadata/identify', body, { signal })
    return data.results
}

// identifyAlbum maps a multi-file selection onto a single release. Slower than
// identifyTracks: the server fingerprints every file and then fetches
// MusicBrainz tracklists for the best candidate releases.
// signal aborts the request: the server threads the request context through
// fpcalc and the AcoustID/MusicBrainz lookups, so cancelling really does stop
// the work rather than just abandoning the response.
export async function identifyAlbum(body: IdentifyAlbumRequest, signal?: AbortSignal) {
    const { data } = await apiClient.post<IdentifyAlbumResponse>(
        '/metadata/identify-album',
        body,
        { signal }
    )
    return data
}

export async function listReleaseCovers(mbid: string, releaseGroup?: string) {
    const { data } = await apiClient.get<CoverCandidate[]>('/metadata/pictures/candidates', {
        params: { mbid, release_group: releaseGroup }
    })
    return data
}

// fetchPictureFile downloads an image the server already serves (a picture
// cell's image URL) and wraps it in a File, so an image the album already has
// can be staged as an upload for another type+slot cell.
export async function fetchPictureFile(url: string): Promise<File> {
    const res = await fetch(url, { credentials: 'include' })
    if (!res.ok) throw new Error(`could not load the image (status ${res.status})`)
    const blob = await res.blob()
    const ext = blob.type === 'image/png' ? 'png' : 'jpg'
    return new File([blob], `copied.${ext}`, { type: blob.type || 'image/jpeg' })
}

export async function applyPicture(form: FormData) {
    const { data } = await apiClient.post<ApplyPictureResult>('/metadata/pictures', form, {
        headers: { 'Content-Type': 'multipart/form-data' }
    })
    return data
}

// getPictures reports every picture type present for the selection and which
// slots hold it, each populated slot carrying its ready-to-render image URL
// (server-resolved — see PictureImageRef). Embedded presence is counted over
// paths; folder art spans the distinct directories paths resolves to.
//
// The selection travels in the POST body rather than the URL: a large
// multi-disc selection as a repeated ?paths= query param overflowed a
// production reverse proxy's header buffer (HTTP 431). The image endpoint it
// returns stays a GET, keyed on one resolved file instead of the selection.
export async function getPictures(libraryId: number, paths: string[]): Promise<PictureInfo[]> {
    const { data } = await apiClient.post<PicturesResponse>('/metadata/pictures/inventory', {
        library_id: libraryId,
        paths
    })
    return data.pictures ?? []
}

// deletePicture removes one picture type+slot cell across paths: for
// 'embedded' it applies directly to those tracks; for 'folder' it reaches
// every directory the selection spans. Returns the whole response, not just
// ok: the server also reports whether its post-write re-index succeeded.
//
// POST, not DELETE-with-body: the selection (paths, mandatory and non-empty)
// travels in the body rather than the URL, the same header-safety fix as
// getPictures/getRawTags above — a POST action rather than a DELETE-with-body
// avoids attaching a payload to a DELETE verb.
export async function deletePicture(
    libraryId: number,
    paths: string[],
    type: string,
    slot: PictureSlot
): Promise<DeletePictureResult> {
    const { data } = await apiClient.post<DeletePictureResult>('/metadata/pictures/removals', {
        library_id: libraryId,
        paths,
        type,
        slot
    })
    return data
}
