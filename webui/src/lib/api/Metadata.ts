import { apiClient } from '@/lib/api/client'
import type {
    ApplyArtistImageResult,
    ApplyPictureResult,
    ArtistFolderInfo,
    CoverCandidate,
    DeleteArtistImageResult,
    DeletePictureResult,
    ImageMeta,
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

// getPictureCandidateInfo probes a Cover Art Archive candidate: the server
// downloads the real image and reports its size, dimensions and format, so the
// picker can show what saving will write (the grid only shows a thumbnail).
export async function getPictureCandidateInfo(url: string): Promise<ImageMeta> {
    const { data } = await apiClient.get<ImageMeta>('/metadata/pictures/candidate-info', {
        params: { url }
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

// getArtistImageCandidateInfo probes a candidate artist portrait — an MBID plus
// a provider-offered URL: the server re-lists (SSRF guard), downloads the real
// image and reports its size, dimensions and format, so the picker can show what
// saving will write (the grid only shows a downscaled preview).
export async function getArtistImageCandidateInfo(mbid: string, url: string): Promise<ImageMeta> {
    const { data } = await apiClient.get<ImageMeta>('/metadata/artist-image/candidate-info', {
        params: { mbid, url }
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
