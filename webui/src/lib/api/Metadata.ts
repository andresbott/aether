import { apiClient } from '@/lib/api/client'
import type {
    ApplyArtistImageResult,
    ApplyPictureResult,
    ArtistFolderInfo,
    CoverCandidate,
    DeleteArtistImageResult,
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
    SearchFoldersResponse,
    UpdateTracksRequest,
    UpdateTracksResponse
} from '@/types/metadata'

export async function listFolders(libraryId: number, path: string) {
    const { data } = await apiClient.get<ListFoldersResponse>('/metadata/folders', {
        params: { library_id: libraryId, path }
    })
    return data.folders
}

// searchFolders filters the library's folders by name: it returns every folder
// (at any depth) whose name contains query, so the picker can jump straight to a
// deep folder without the user expanding to it first. truncated is true when the
// match set hit the server's cap.
export async function searchFolders(
    libraryId: number,
    query: string
): Promise<SearchFoldersResponse> {
    const { data } = await apiClient.get<SearchFoldersResponse>('/metadata/folders', {
        params: { library_id: libraryId, q: query }
    })
    return { folders: data.folders ?? [], truncated: data.truncated ?? false }
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
export async function getRawTags(libraryId: number, paths: string[]) {
    const { data } = await apiClient.get<RawTagsResponse>('/metadata/tracks/raw', {
        params: { library_id: libraryId, paths },
        paramsSerializer: { indexes: null } // repeat paths= for arrays
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

// getPictureUrl builds the URL of one picture type+slot cell for use as an
// <img> src. The optional bust value forces a reload after a change (the
// endpoint sets Cache-Control: no-cache but the URL is otherwise unchanged).
// For the embedded slot, paths narrow the probe to the selected tracks.
//
// size requests an optimized, display-sized copy instead of the original — pass
// it for grid thumbnails. Omit it when the bytes themselves matter (copying a
// picture into another slot, see fetchPictureFile), since a derivative is a
// downscaled re-encode of the source.
export function getPictureUrl(
    libraryId: number,
    path: string,
    type: string,
    slot: PictureSlot,
    bust?: number,
    paths?: string[],
    size?: number
): string {
    const base = apiClient.defaults.baseURL ?? ''
    const params = new URLSearchParams({ library_id: String(libraryId), path, type, slot })
    if (bust !== undefined) params.set('t', String(bust))
    for (const p of paths ?? []) params.append('paths', p)
    if (size) params.set('size', String(size))
    return `${base}/metadata/pictures/image?${params.toString()}`
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

// getPictures reports every picture type present for the folder and which
// slots hold it. Embedded presence is counted over the given paths (or every
// folder track when omitted).
export async function getPictures(
    libraryId: number,
    path: string,
    paths?: string[]
): Promise<PictureInfo[]> {
    const { data } = await apiClient.get<PicturesResponse>('/metadata/pictures', {
        params: { library_id: libraryId, path, paths },
        paramsSerializer: { indexes: null } // repeat paths= for arrays
    })
    return data.pictures ?? []
}

// resolveArtistFolder reports whether the selected folder is an artist folder
// (its albums are tagged with an album artist matching the folder name) and the
// image it already holds. The editor shows the control only when eligible.
export async function resolveArtistFolder(
    libraryId: number,
    path: string
): Promise<ArtistFolderInfo> {
    const { data } = await apiClient.get<ArtistFolderInfo>('/metadata/artist-folder', {
        params: { library_id: libraryId, path }
    })
    return data
}

// getArtistImageUrl builds the <img> src for the selected folder's current image.
// bust forces a reload after a change (the endpoint sends Cache-Control:
// no-cache but the URL is otherwise unchanged).
export function getArtistImageUrl(libraryId: number, path: string, bust?: number): string {
    const base = apiClient.defaults.baseURL ?? ''
    const params = new URLSearchParams({ library_id: String(libraryId), path })
    if (bust !== undefined) params.set('t', String(bust))
    return `${base}/metadata/artist-image?${params.toString()}`
}

// applyArtistImage writes an artist portrait as artist.<ext> into the selected
// folder. The form carries library_id, path, and either an uploaded image file
// ("image") or an online pick ("mbid" + "url").
export async function applyArtistImage(form: FormData): Promise<ApplyArtistImageResult> {
    const { data } = await apiClient.post<ApplyArtistImageResult>('/metadata/artist-image', form, {
        headers: { 'Content-Type': 'multipart/form-data' }
    })
    return data
}

// deleteArtistImage removes the selected folder's current artist image.
export async function deleteArtistImage(
    libraryId: number,
    path: string
): Promise<DeleteArtistImageResult> {
    const { data } = await apiClient.delete<DeleteArtistImageResult>('/metadata/artist-image', {
        params: { library_id: libraryId, path }
    })
    return data
}

// deletePicture removes one picture type+slot cell. For 'embedded', paths are
// the selected tracks the removal applies to. Returns the whole response, not
// just ok: the server also reports whether its post-write re-index succeeded.
export async function deletePicture(
    libraryId: number,
    path: string,
    type: string,
    slot: PictureSlot,
    paths?: string[]
): Promise<DeletePictureResult> {
    const { data } = await apiClient.delete<DeletePictureResult>('/metadata/pictures', {
        params: { library_id: libraryId, path, type, slot, paths },
        paramsSerializer: { indexes: null } // repeat paths= for arrays
    })
    return data
}
