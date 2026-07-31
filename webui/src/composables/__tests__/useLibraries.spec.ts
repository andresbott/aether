import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import type { Library, LibraryInput } from '@/types/libraries'

const updateLibraryMock = vi.fn()
const createLibraryMock = vi.fn()

vi.mock('@/lib/api/Libraries', () => ({
    updateLibrary: (...args: unknown[]) => updateLibraryMock(...args),
    createLibrary: (...args: unknown[]) => createLibraryMock(...args),
    listLibraries: vi.fn(),
    deleteLibrary: vi.fn()
}))

vi.mock('primevue/usetoast', () => ({
    useToast: () => ({ add: vi.fn() })
}))

import { useUpdateLibrary, useCreateLibrary } from '@/composables/useLibraries'

function sampleLibrary(): Library {
    return {
        id: 1,
        name: 'Main',
        path: '/srv/music',
        exclude_patterns: [],
        follow_symlinks: true,
        show_artists: true,
        default_view: 'artists',
        icon: 'folder',
        cover_style: 'auto',
        last_scan_started_at: null,
        created_at: '',
        updated_at: '',
        track_count: 0
    }
}

const sampleInput: LibraryInput = {
    name: 'Main',
    path: '/srv/music',
    exclude_patterns: [],
    follow_symlinks: true,
    show_artists: true,
    default_view: 'artists',
    icon: 'folder',
    cover_style: 'auto'
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
})

describe('useUpdateLibrary', () => {
    it('invalidates the Subsonic cache so the library view reflects the new default view', async () => {
        updateLibraryMock.mockResolvedValue(sampleLibrary())
        const { mutation, invalidateSpy } = mountMutation(useUpdateLibrary)

        await mutation.mutateAsync({ id: 1, input: sampleInput })

        expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['subsonic'] })
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
