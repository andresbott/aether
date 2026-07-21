import { apiClient } from '@/lib/api/client'
import type {
    ApplyPictureResult,
    CoverCandidate,
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

export async function updateTracks(body: UpdateTracksRequest) {
    const { data } = await apiClient.put<UpdateTracksResponse>('/metadata/tracks', body)
    return data.results
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
export async function identifyTracks(body: IdentifyRequest) {
    const { data } = await apiClient.post<IdentifyResponse>('/metadata/identify', body)
    return data.results
}

// getPictureUrl builds the URL of one picture type+slot cell for use as an
// <img> src. The optional bust value forces a reload after a change (the
// endpoint sets Cache-Control: no-cache but the URL is otherwise unchanged).
// For the embedded slot, paths narrow the probe to the selected tracks.
export function getPictureUrl(
    libraryId: number,
    path: string,
    type: string,
    slot: PictureSlot,
    bust?: number,
    paths?: string[]
): string {
    const base = apiClient.defaults.baseURL ?? ''
    const params = new URLSearchParams({ library_id: String(libraryId), path, type, slot })
    if (bust !== undefined) params.set('t', String(bust))
    for (const p of paths ?? []) params.append('paths', p)
    return `${base}/metadata/pictures/image?${params.toString()}`
}

export async function listReleaseCovers(mbid: string, releaseGroup?: string) {
    const { data } = await apiClient.get<CoverCandidate[]>('/metadata/pictures/candidates', {
        params: { mbid, release_group: releaseGroup }
    })
    return data
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

// deletePicture removes one picture type+slot cell. For 'embedded', paths are
// the selected tracks the removal applies to.
export async function deletePicture(
    libraryId: number,
    path: string,
    type: string,
    slot: PictureSlot,
    paths?: string[]
) {
    await apiClient.delete('/metadata/pictures', {
        params: { library_id: libraryId, path, type, slot, paths },
        paramsSerializer: { indexes: null } // repeat paths= for arrays
    })
}
