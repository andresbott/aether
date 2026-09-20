import { apiClient } from '@/lib/api/client'
import type { ListScanFoldersResponse, ScanFolder } from '@/types/scanFolders'

export async function listScanFolders(): Promise<ScanFolder[]> {
    const { data } = await apiClient.get<ListScanFoldersResponse>('/scan-folders')
    return data.scan_folders ?? []
}
