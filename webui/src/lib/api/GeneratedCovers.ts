import { apiClient } from '@/lib/api/client'
import type { GeneratedCoverSettings, GeneratedCoverSettingsInput } from '@/types/generatedCovers'

export async function getGeneratedCoverSettings(): Promise<GeneratedCoverSettings> {
    const { data } = await apiClient.get<GeneratedCoverSettings>('/settings/generated-covers')
    return data
}

export async function updateGeneratedCoverSettings(
    input: GeneratedCoverSettingsInput
): Promise<GeneratedCoverSettings> {
    const { data } = await apiClient.put<GeneratedCoverSettings>('/settings/generated-covers', input)
    return data
}
