import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { useToast } from 'primevue/usetoast'
import type { Ref, ComputedRef } from 'vue'
import * as TokensApi from '@/lib/api/Tokens'
import type { ApiToken, CreateTokenInput } from '@/types/tokens'
import { apiErrorMessage } from '@/lib/apiError'

export const tokenQueryKeys = {
    all: ['tokens'] as const
}

/** All the caller's tokens: PATs (kind 'client') and SPA sessions (kind 'session'). */
export function useTokens(enabled?: Ref<boolean> | ComputedRef<boolean>) {
    return useQuery<ApiToken[]>({
        queryKey: tokenQueryKeys.all,
        queryFn: () => TokensApi.listTokens(),
        staleTime: 30 * 1000,
        enabled: enabled ?? true
    })
}

export function useCreateToken() {
    const qc = useQueryClient()
    const toast = useToast()
    return useMutation({
        mutationFn: (input: CreateTokenInput) => TokensApi.createToken(input),
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: tokenQueryKeys.all })
        },
        onError: (err: unknown) => {
            toast.add({
                severity: 'error',
                summary: 'Failed to create token',
                detail: apiErrorMessage(err),
                life: 5000
            })
        }
    })
}

export function useRevokeToken() {
    const qc = useQueryClient()
    const toast = useToast()
    return useMutation({
        mutationFn: (tokenId: string) => TokensApi.revokeToken(tokenId),
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: tokenQueryKeys.all })
            toast.add({ severity: 'info', summary: 'Token revoked', life: 3000 })
        },
        onError: (err: unknown) => {
            toast.add({
                severity: 'error',
                summary: 'Failed to revoke token',
                detail: apiErrorMessage(err),
                life: 5000
            })
        }
    })
}
