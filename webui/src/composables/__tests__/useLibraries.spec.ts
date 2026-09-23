import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import type { Library, LibraryInput, LibraryFilterOptions } from '@/types/libraries'

const updateLibraryMock = vi.fn()
const createLibraryMock = vi.fn()
const getLibraryFilterOptionsMock = vi.fn()

vi.mock('@/lib/api/Libraries', () => ({
    updateLibrary: (...args: unknown[]) => updateLibraryMock(...args),
    createLibrary: (...args: unknown[]) => createLibraryMock(...args),
    listLibraries: vi.fn(),
    deleteLibrary: vi.fn(),
    getLibraryFilterOptions: (...args: unknown[]) => getLibraryFilterOptionsMock(...args)
}))

const toastAdd = vi.hoisted(() => vi.fn())
vi.mock('primevue/usetoast', () => ({
    useToast: () => ({ add: toastAdd })
}))

import { useUpdateLibrary, useCreateLibrary, useLibraryFilterOptions } from '@/composables/useLibraries'

function sampleLibrary(): Library {
    return {
        id: 1,
        name: 'Main',
        views: ['artists', 'releases'],
        default_view: 'artists',
        hide_from_artist_index: false,
        split_views: false,
        icon: 'folder',
        filters: [],
        created_at: '',
        updated_at: '',
        track_count: 0
    }
}

const sampleInput: LibraryInput = {
    name: 'Main',
    views: ['artists', 'releases'],
    default_view: 'artists',
    hide_from_artist_index: false,
    split_views: false,
    icon: 'folder',
    filters: []
}

/** Mounts a mutation composable inside a real vue-query context and returns the mutation + the invalidate spy. */
function mountMutation<T>(composable: () => T) {
    const queryClient = new QueryClient()
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

    let mutation!: T
    const Comp = defineComponent({
        setup() {
            mutation = composable()
            return () => h('div')
        }
    })
    mount(Comp, {
        global: { plugins: [[VueQueryPlugin, { queryClient }]] }
    })
    return { mutation, invalidateSpy }
}

beforeEach(() => {
    updateLibraryMock.mockReset()
    createLibraryMock.mockReset()
    getLibraryFilterOptionsMock.mockReset()
    toastAdd.mockReset()
})

describe('useUpdateLibrary', () => {
    it('invalidates the Subsonic cache so the library view reflects the new default view', async () => {
        updateLibraryMock.mockResolvedValue(sampleLibrary())
        const { mutation, invalidateSpy } = mountMutation(useUpdateLibrary)

        await mutation.mutateAsync({ id: 1, input: sampleInput })

        expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['subsonic'] })
    })

    it('toasts "Library updated" on success', async () => {
        updateLibraryMock.mockResolvedValue(sampleLibrary())
        const { mutation } = mountMutation(useUpdateLibrary)

        await mutation.mutateAsync({ id: 1, input: sampleInput })

        expect(toastAdd).toHaveBeenCalledWith(
            expect.objectContaining({ severity: 'success', summary: 'Library updated' })
        )
    })
})

describe('useCreateLibrary', () => {
    it('invalidates the Subsonic cache so the new library shows up in the music folders', async () => {
        createLibraryMock.mockResolvedValue(sampleLibrary())
        const { mutation, invalidateSpy } = mountMutation(useCreateLibrary)

        await mutation.mutateAsync(sampleInput)

        expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['subsonic'] })
    })
})

// A 422 names the offending field in errors[]; the dialog renders that inline,
// so the composable must not also toast it (that would double-report the same
// message). Other failures (network, 500) still toast.
describe('validation errors are left for the form, not toasted', () => {
    const fieldProblem = {
        response: {
            status: 422,
            data: { detail: 'name is required', errors: [{ pointer: '/name', detail: 'name is required' }] }
        }
    }

    it('useCreateLibrary does not toast a field-validation error', async () => {
        createLibraryMock.mockRejectedValue(fieldProblem)
        const { mutation } = mountMutation(useCreateLibrary)
        await mutation.mutateAsync(sampleInput).catch(() => {})
        expect(toastAdd).not.toHaveBeenCalled()
    })

    it('useUpdateLibrary does not toast a field-validation error', async () => {
        updateLibraryMock.mockRejectedValue(fieldProblem)
        const { mutation } = mountMutation(useUpdateLibrary)
        await mutation.mutateAsync({ id: 1, input: sampleInput }).catch(() => {})
        expect(toastAdd).not.toHaveBeenCalled()
    })

    it('still toasts a non-field failure such as a 500', async () => {
        createLibraryMock.mockRejectedValue({ response: { status: 500, data: { detail: 'boom' } } })
        const { mutation } = mountMutation(useCreateLibrary)
        await mutation.mutateAsync(sampleInput).catch(() => {})
        expect(toastAdd).toHaveBeenCalledWith(expect.objectContaining({ severity: 'error' }))
    })
})

describe('useLibraryFilterOptions', () => {
    it('calls getLibraryFilterOptions once and exposes the data', async () => {
        const sample: LibraryFilterOptions = {
            scan_folders: ['Music'],
            formats: ['flac', 'mp3'],
            genres: ['Rock'],
            release_types: ['Album']
        }
        getLibraryFilterOptionsMock.mockResolvedValue(sample)

        let query!: ReturnType<typeof useLibraryFilterOptions>
        const Comp = defineComponent({
            setup() {
                query = useLibraryFilterOptions()
                return () => h('div')
            }
        })
        mount(Comp, {
            global: { plugins: [[VueQueryPlugin, { queryClient: new QueryClient() }]] }
        })
        await flushPromises()

        expect(getLibraryFilterOptionsMock).toHaveBeenCalledTimes(1)
        expect(query.data.value).toEqual(sample)
    })
})
