import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { useToast } from 'primevue/usetoast'
import * as LibrariesApi from '@/lib/api/Libraries'
import type { Library, LibraryInput } from '@/types/libraries'

export const libraryQueryKeys = {
    all: ['libraries'] as const,
    detail: (id: number) => ['libraries', id] as const
}

export function useLibraries() {
    return useQuery<Library[]>({
        queryKey: libraryQueryKeys.all,
        queryFn: () => LibrariesApi.listLibraries(),
        staleTime: 30 * 1000
    })
}

export function useCreateLibrary() {
    const qc = useQueryClient()
    const toast = useToast()
    return useMutation({
        mutationFn: (input: LibraryInput) => LibrariesApi.createLibrary(input),
        onSuccess: (lib) => {
            qc.invalidateQueries({ queryKey: libraryQueryKeys.all })
            toast.add({
                severity: 'success',
                summary: 'Library created',
                detail: lib.name,
                life: 3000
            })
        },
        onError: (err: any) => {
            toast.add({
                severity: 'error',
                summary: 'Failed to create library',
                detail: err?.response?.data?.error ?? err.message,
                life: 5000
            })
        }
    })
}

export function useUpdateLibrary() {
    const qc = useQueryClient()
    const toast = useToast()
    return useMutation({
        mutationFn: ({ id, input }: { id: number; input: LibraryInput }) =>
            LibrariesApi.updateLibrary(id, input),
        onSuccess: (lib) => {
            qc.invalidateQueries({ queryKey: libraryQueryKeys.all })
            toast.add({
                severity: 'success',
                summary: lib.path_changed ? 'Library updated — tracks wiped' : 'Library updated',
                detail: lib.name,
                life: 3000
            })
        },
        onError: (err: any) => {
            toast.add({
                severity: 'error',
                summary: 'Failed to update library',
                detail: err?.response?.data?.error ?? err.message,
                life: 5000
            })
        }
    })
}

export function useDeleteLibrary() {
    const qc = useQueryClient()
    const toast = useToast()
    return useMutation({
        mutationFn: (id: number) => LibrariesApi.deleteLibrary(id),
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: libraryQueryKeys.all })
            // Also drop Subsonic library data, since albums/artists may have changed.
            qc.invalidateQueries({ queryKey: ['subsonic'] })
            toast.add({
                severity: 'info',
                summary: 'Library deleted',
                life: 3000
            })
        },
        onError: (err: any) => {
            toast.add({
                severity: 'error',
                summary: 'Failed to delete library',
                detail: err?.response?.data?.error ?? err.message,
                life: 5000
            })
        }
    })
}
