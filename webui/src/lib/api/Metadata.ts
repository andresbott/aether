import { apiClient } from '@/lib/api/client'
import type {
    ApplyCoverResult,
    CoverCandidate,
    CoverInfoResponse,
    CoverSourceEntry,
    CoverTarget,
    ListFoldersResponse,
    ListTracksResponse,
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

// getSourceCoverUrl builds the URL of a folder cover image for use as an <img>
// src. When source is given it serves that specific source (db/folder/embedded),
// otherwise the winning cover. The optional bust value forces a reload after a
// change (the endpoint sets Cache-Control: no-cache but the URL is otherwise
// unchanged).
export function getSourceCoverUrl(
    libraryId: number,
    path: string,
    source?: CoverTarget,
    bust?: number
): string {
    const base = apiClient.defaults.baseURL ?? ''
    const params = new URLSearchParams({ library_id: String(libraryId), path })
    if (source) params.set('source', source)
    if (bust !== undefined) params.set('t', String(bust))
    return `${base}/metadata/cover?${params.toString()}`
}

export async function listReleaseCovers(mbid: string, releaseGroup?: string) {
    const { data } = await apiClient.get<CoverCandidate[]>('/metadata/cover/candidates', {
        params: { mbid, release_group: releaseGroup }
    })
    return data
}

export async function applyCover(form: FormData) {
    const { data } = await apiClient.post<ApplyCoverResult>('/metadata/cover', form, {
        headers: { 'Content-Type': 'multipart/form-data' }
    })
    return data
}

export async function getCoverInfo(libraryId: number, path: string): Promise<CoverSourceEntry[]> {
    const { data } = await apiClient.get<CoverInfoResponse>('/metadata/cover/info', {
        params: { library_id: libraryId, path }
    })
    return data.sources ?? []
}

// deleteCover removes a folder cover from one source. For 'embedded', paths are
// the selected tracks the removal applies to.
export async function deleteCover(
    libraryId: number,
    path: string,
    source: CoverTarget,
    paths?: string[]
) {
    await apiClient.delete('/metadata/cover', {
        params: { library_id: libraryId, path, source, paths },
        paramsSerializer: { indexes: null } // repeat paths= for arrays
    })
}
