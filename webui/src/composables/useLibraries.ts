import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { useToast } from 'primevue/usetoast'
import * as LibrariesApi from '@/lib/api/Libraries'
import type { CatalogSettings, Library, LibraryInput, LibraryFilterOptions } from '@/types/libraries'
import { apiErrorMessage, apiFieldErrors } from '@/lib/apiError'

export const libraryQueryKeys = {
    all: ['libraries'] as const,
    detail: (id: number) => ['libraries', id] as const,
    filterOptions: ['libraries', 'filter-options'] as const,
    catalog: ['libraries', 'catalog'] as const
}

export function useLibraries() {
    return useQuery<Library[]>({
        queryKey: libraryQueryKeys.all,
        queryFn: () => LibrariesApi.listLibraries(),
        staleTime: 30 * 1000
    })
}

// The options change only when a scan changes the catalog, so a short stale
// time is plenty; the dialog refetches them each time it opens.
export function useLibraryFilterOptions() {
    return useQuery<LibraryFilterOptions>({
        queryKey: libraryQueryKeys.filterOptions,
        queryFn: () => LibrariesApi.getLibraryFilterOptions(),
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
            // Also drop Subsonic library data so the new library appears in
            // the music folders the library view is driven by.
            qc.invalidateQueries({ queryKey: ['subsonic'] })
            toast.add({
                severity: 'success',
                summary: 'Library created',
                detail: lib.name,
                life: 3000
            })
        },
        onError: (err: any) => {
            // A 422 names the bad field in errors[]; the dialog shows it inline,
            // so toasting would double-report the same message.
            if (apiFieldErrors(err).length > 0) return
            toast.add({
                severity: 'error',
                summary: 'Failed to create library',
                detail: apiErrorMessage(err),
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
            // Also drop Subsonic library data: the library view reads its
            // default view (and albums/artists) from the Subsonic music
            // folders, so edits must refresh that cache too.
            qc.invalidateQueries({ queryKey: ['subsonic'] })
            toast.add({
                severity: 'success',
                summary: 'Library updated',
                detail: lib.name,
                life: 3000
            })
        },
        onError: (err: any) => {
            // A 422 names the bad field in errors[]; the dialog shows it inline,
            // so toasting would double-report the same message.
            if (apiFieldErrors(err).length > 0) return
            toast.add({
                severity: 'error',
                summary: 'Failed to update library',
                detail: apiErrorMessage(err),
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
                detail: apiErrorMessage(err),
                life: 5000
            })
        }
    })
}

// The root library's settings, as the admin panel edits them. Everyone else
// reads them from getMusicFolders' catalog descriptor (useCatalogView).
export function useCatalogSettings() {
    return useQuery<CatalogSettings>({
        queryKey: libraryQueryKeys.catalog,
        queryFn: () => LibrariesApi.getCatalogSettings(),
        staleTime: 30 * 1000
    })
}

export function useUpdateCatalogSettings() {
    const qc = useQueryClient()
    const toast = useToast()
    return useMutation({
        mutationFn: (input: CatalogSettings) => LibrariesApi.updateCatalogSettings(input),
        onSuccess: (settings) => {
            qc.setQueryData(libraryQueryKeys.catalog, settings)
            // The sidebar reads the setting from the Subsonic music folders.
            qc.invalidateQueries({ queryKey: ['subsonic'] })
            toast.add({ severity: 'success', summary: 'Main library updated', life: 3000 })
        },
        onError: (err: any) => {
            toast.add({
                severity: 'error',
                summary: 'Failed to update the main library',
                detail: apiErrorMessage(err),
                life: 5000
            })
        }
    })
}
