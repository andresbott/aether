import { apiClient } from '@/lib/api/client'
import type {
    Library,
    LibraryInput,
    ListLibrariesResponse,
    BrowseResponse
} from '@/types/libraries'

export async function listLibraries(): Promise<Library[]> {
    const { data } = await apiClient.get<ListLibrariesResponse>('/libraries')
    return data.libraries ?? []
}

export async function getLibrary(id: number): Promise<Library> {
    const { data } = await apiClient.get<Library>(`/libraries/${id}`)
    return data
}

export async function createLibrary(input: LibraryInput): Promise<Library> {
    const { data } = await apiClient.post<Library>('/libraries', input)
    return data
}

export async function updateLibrary(id: number, input: LibraryInput): Promise<Library> {
    const { data } = await apiClient.put<Library>(`/libraries/${id}`, input)
    return data
}

export async function deleteLibrary(id: number): Promise<void> {
    await apiClient.delete(`/libraries/${id}`)
}

export async function browseFolders(
    path?: string,
    showHidden = false
): Promise<BrowseResponse> {
    const { data } = await apiClient.get<BrowseResponse>('/libraries/browse', {
        params: {
            ...(path ? { path } : {}),
            ...(showHidden ? { show_hidden: true } : {})
        }
    })
    return data
}
