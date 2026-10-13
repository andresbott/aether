import { computed } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import * as VersionApi from '@/lib/api/Version'
import { subsonicClient } from '@/lib/api/subsonic'
import { sessionKind } from '@/lib/host'
import type { ServerVersion } from '@/types/version'

export const versionQueryKeys = {
    all: ['version'] as const
}

// A key session has no /api/v0: the version comes off a /rest ping's envelope
// instead, with no commit or build time (About hides those when empty).
async function restVersion(): Promise<ServerVersion> {
    const version = await subsonicClient.serverVersion()
    return { version: version ?? '', commit: '', build_time: '' }
}

// The build info never changes while the server runs, so cache it for the
// lifetime of the session — per session kind, since the two sources differ. The
// key, not the live ref, picks the source, so the answer caches under its key.
export function useVersion() {
    return useQuery<ServerVersion>({
        queryKey: computed(() => [...versionQueryKeys.all, sessionKind.value] as const),
        queryFn: ({ queryKey }) => (queryKey[1] === 'apiKey' ? restVersion() : VersionApi.getVersion()),
        staleTime: Infinity,
        retry: false
    })
}
