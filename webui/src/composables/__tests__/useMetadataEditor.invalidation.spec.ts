import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'

const updateTracksMock = vi.hoisted(() => vi.fn())
const applyPictureMock = vi.hoisted(() => vi.fn())
const deletePictureMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api/Metadata', () => ({
    updateTracks: (...args: unknown[]) => updateTracksMock(...args),
    applyPicture: (...args: unknown[]) => applyPictureMock(...args),
    deletePicture: (...args: unknown[]) => deletePictureMock(...args)
}))

const toastAddSpy = vi.hoisted(() => vi.fn())
vi.mock('primevue/usetoast', () => ({
    useToast: () => ({ add: toastAddSpy })
}))

import {
    useUpdateTracks,
    useApplyPicture,
    useDeletePicture
} from '@/composables/useMetadataEditor'

/** Mounts a mutation composable in a real vue-query context; returns it plus the invalidate spy. */
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
    mount(Comp, { global: { plugins: [[VueQueryPlugin, { queryClient }]] } })
    return { mutation, invalidateSpy }
}

/** The keys every metadata write must drop: the editor's own views plus the whole music UI. */
function invalidatedKeys(spy: any): unknown[][] {
    return spy.mock.calls.map((c: any) => c[0].queryKey)
}

beforeEach(() => {
    updateTracksMock.mockReset()
    applyPictureMock.mockReset()
    deletePictureMock.mockReset()
    toastAddSpy.mockReset()
})

/** The warn toasts a failed post-write re-index must produce. */
function rescanWarnings(): any[] {
    return toastAddSpy.mock.calls
        .map((c) => c[0])
        .filter((t: any) => t.summary === 'Saved, but the library index was not updated')
}

describe('metadata write invalidation', () => {
    it('useUpdateTracks drops the editor views and the whole subsonic tree', async () => {
        updateTracksMock.mockResolvedValue({ results: [{ path: 'a.mp3', ok: true }] })
        const { mutation, invalidateSpy } = mountMutation(useUpdateTracks)
        await mutation.mutateAsync({ library_id: 1, paths: ['a.mp3'], fields: { title: 'T' } })
        const keys = invalidatedKeys(invalidateSpy)
        expect(keys).toContainEqual(['metadata', 'tracks'])
        expect(keys).toContainEqual(['metadata', 'raw'])
        expect(keys).toContainEqual(['subsonic'])
    })

    it('useApplyPicture drops the same keys', async () => {
        applyPictureMock.mockResolvedValue({ ok: true, slot: 'folder', type: 'Front Cover' })
        const { mutation, invalidateSpy } = mountMutation(useApplyPicture)
        await mutation.mutateAsync(new FormData())
        const keys = invalidatedKeys(invalidateSpy)
        expect(keys).toContainEqual(['metadata', 'tracks'])
        expect(keys).toContainEqual(['metadata', 'raw'])
        expect(keys).toContainEqual(['subsonic'])
    })

    it('useDeletePicture drops the same keys', async () => {
        deletePictureMock.mockResolvedValue({ ok: true })
        const { mutation, invalidateSpy } = mountMutation(useDeletePicture)
        await mutation.mutateAsync({
            libraryId: 1,
            paths: ['Artist/Album/01.mp3'],
            type: 'Front Cover',
            slot: 'folder'
        })
        const keys = invalidatedKeys(invalidateSpy)
        expect(keys).toContainEqual(['metadata', 'tracks'])
        expect(keys).toContainEqual(['metadata', 'raw'])
        expect(keys).toContainEqual(['subsonic'])
    })
})

// The picture endpoints report the same post-write re-index status the tag
// endpoint does; a failure means the image is written but the album still
// serves the old one, so it must never pass as a plain success.
describe('picture write rescan reporting', () => {
    it('useApplyPicture warns on a failed re-index', async () => {
        applyPictureMock.mockResolvedValue({
            ok: true,
            slot: 'folder',
            type: 'Front Cover',
            rescan: { ok: false, error: 'db is locked' }
        })
        const { mutation } = mountMutation(useApplyPicture)
        await mutation.mutateAsync(new FormData())
        expect(rescanWarnings()).toEqual([
            expect.objectContaining({ severity: 'warn', detail: 'db is locked', life: 8000 })
        ])
    })

    it('useApplyPicture stays silent when the re-index succeeded', async () => {
        applyPictureMock.mockResolvedValue({
            ok: true,
            slot: 'folder',
            type: 'Front Cover',
            rescan: { ok: true }
        })
        const { mutation } = mountMutation(useApplyPicture)
        await mutation.mutateAsync(new FormData())
        expect(rescanWarnings()).toEqual([])
    })

    it('useDeletePicture warns on a failed re-index, falling back for a missing message', async () => {
        deletePictureMock.mockResolvedValue({ ok: true, rescan: { ok: false } })
        const { mutation } = mountMutation(useDeletePicture)
        await mutation.mutateAsync({
            libraryId: 1,
            paths: ['Artist/Album/01.mp3'],
            type: 'Front Cover',
            slot: 'folder'
        })
        expect(rescanWarnings()).toEqual([
            expect.objectContaining({ severity: 'warn', detail: 'unknown error', life: 8000 })
        ])
    })

    // The edit session raises one aggregate warning per save, so the per-op
    // toast must be suppressible or a multi-cell save stacks duplicates.
    it('quietRescanWarning suppresses the per-call warning', async () => {
        applyPictureMock.mockResolvedValue({
            ok: true,
            slot: 'folder',
            type: 'Front Cover',
            rescan: { ok: false, error: 'db is locked' }
        })
        const { mutation } = mountMutation(() => useApplyPicture({ quietRescanWarning: true }))
        await mutation.mutateAsync(new FormData())
        expect(rescanWarnings()).toEqual([])
    })
})
