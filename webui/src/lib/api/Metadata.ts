import { apiClient } from '@/lib/api/client'
import type {
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
