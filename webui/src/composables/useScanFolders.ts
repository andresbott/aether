import { useQuery } from '@tanstack/vue-query'
import * as ScanFoldersApi from '@/lib/api/ScanFolders'
import type { ScanFolder } from '@/types/scanFolders'

export const scanFolderQueryKeys = {
    all: ['scan-folders'] as const
}

// The list only changes when the server restarts with an edited config, so it is
// cheap to keep; `available` is probed per request, which is why it is not cached
// for long either.
export function useScanFolders() {
    return useQuery<ScanFolder[]>({
        queryKey: scanFolderQueryKeys.all,
        queryFn: () => ScanFoldersApi.listScanFolders(),
        staleTime: 30 * 1000
    })
}
