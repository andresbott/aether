import { useQuery } from '@tanstack/vue-query'
import * as VersionApi from '@/lib/api/Version'
import type { ServerVersion } from '@/types/version'

export const versionQueryKeys = {
    all: ['version'] as const
}

// The build info never changes while the server runs, so cache it for the
// lifetime of the session.
export function useVersion() {
    return useQuery<ServerVersion>({
        queryKey: versionQueryKeys.all,
        queryFn: () => VersionApi.getVersion(),
        staleTime: Infinity,
        retry: false
    })
}
