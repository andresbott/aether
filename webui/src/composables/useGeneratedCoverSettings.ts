import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { useToast } from 'primevue/usetoast'
import * as Api from '@/lib/api/GeneratedCovers'
import type { GeneratedCoverSettings, GeneratedCoverSettingsInput } from '@/types/generatedCovers'
import { apiErrorMessage } from '@/lib/apiError'

export const generatedCoverKeys = { all: ['generated-covers'] as const }

export function useGeneratedCoverSettings() {
    return useQuery<GeneratedCoverSettings>({
        queryKey: generatedCoverKeys.all,
        queryFn: () => Api.getGeneratedCoverSettings(),
        staleTime: 60 * 1000
    })
}

export function useUpdateGeneratedCoverSettings() {
    const qc = useQueryClient()
    const toast = useToast()
    return useMutation({
        mutationFn: (input: GeneratedCoverSettingsInput) => Api.updateGeneratedCoverSettings(input),
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: generatedCoverKeys.all })
            // Placeholder covers depend on the Default set, so drop cached covers.
            qc.invalidateQueries({ queryKey: ['subsonic'] })
            toast.add({ severity: 'success', summary: 'Generated covers updated', life: 3000 })
        },
        onError: (err: any) =>
            toast.add({
                severity: 'error',
                summary: 'Failed to update generated covers',
                detail: apiErrorMessage(err),
                life: 5000
            })
    })
}
