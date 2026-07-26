import { useMutation } from '@tanstack/vue-query'
import { useToast } from 'primevue/usetoast'
import { setArtistMBID } from '@/lib/api/Artists'
import { apiErrorMessage } from '@/lib/apiError'

export function useSetArtistMBID() {
    const toast = useToast()
    return useMutation({
        mutationFn: (params: { numericId: number; mbid: string }) =>
            setArtistMBID(params.numericId, params.mbid),
        onSuccess: (res) => {
            if (res.fetchError) {
                toast.add({
                    severity: 'warn',
                    summary: 'Match saved',
                    detail: `Image fetch failed: ${res.fetchError}`,
                    life: 5000
                })
            } else {
                toast.add({ severity: 'success', summary: 'Match saved', life: 3000 })
            }
        },
        onError: (err: any) => {
            toast.add({
                severity: 'error',
                summary: 'Failed to save match',
                detail: apiErrorMessage(err),
                life: 5000
            })
        }
    })
}
