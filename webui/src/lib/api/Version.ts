import { apiClient } from '@/lib/api/client'
import type { ServerVersion } from '@/types/version'

export async function getVersion(): Promise<ServerVersion> {
    const { data } = await apiClient.get<ServerVersion>('/version')
    return data
}
