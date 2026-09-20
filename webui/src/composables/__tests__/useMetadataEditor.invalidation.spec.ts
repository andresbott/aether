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

const pollReindexMock = vi.hoisted(() => vi.fn())
vi.mock('@/composables/useReindexPolling', () => ({
    pollReindex: (...args: unknown[]) => pollReindexMock(...args)
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
    pollReindexMock.mockReset()
    // Every mutation polls unconditionally; default to an instantly-settled,
    // successful poll so tests that don't care about it stay unaffected.
    pollReindexMock.mockResolvedValue({ failed: 0 })
})

/** The warn toasts a failed polled re-index must produce. */
function reindexWarnings(): any[] {
    return toastAddSpy.mock.calls
        .map((c) => c[0])
        .filter((t: any) => t.summary === 'Saved, but the library index was not updated')
}

describe('metadata write invalidation', () => {
    it('useUpdateTracks drops the editor views and the whole subsonic tree', async () => {
        updateTracksMock.mockResolvedValue({ results: [{ path: 'a.mp3', ok: true }] })
        const { mutation, invalidateSpy } = mountMutation(useUpdateTracks)
        await mutation.mutateAsync({ scan_folder: 'Main', paths: ['a.mp3'], fields: { title: 'T' } })
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
            scanFolder: 'Main',
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

// Each single-write mutation now enqueues a re-index job (reindex.execution_id
// on the response) instead of re-indexing synchronously. The mutation must stay
// pending — and must not invalidate caches or toast success — until pollReindex
// settles. A poll that reports a failure is not a write failure (the change
// already landed on disk); it surfaces as a warn toast instead.
describe('metadata write reindex polling', () => {
    it('useApplyPicture does not resolve or invalidate until the poll settles', async () => {
        applyPictureMock.mockResolvedValue({
            ok: true,
            slot: 'folder',
            type: 'Front Cover',
            reindex: { execution_id: 'x' }
        })
        let resolvePoll!: (v: { failed: number }) => void
        pollReindexMock.mockReturnValue(
            new Promise((resolve) => {
                resolvePoll = resolve
            })
        )
        const { mutation, invalidateSpy } = mountMutation(useApplyPicture)
        let resolved = false
        const done = mutation.mutateAsync(new FormData()).then(() => {
            resolved = true
        })

        await vi.waitFor(() => expect(pollReindexMock).toHaveBeenCalledWith(['x'], { signal: expect.any(AbortSignal) }))
        expect(resolved).toBe(false)
        expect(invalidateSpy).not.toHaveBeenCalled()

        resolvePoll({ failed: 0 })
        await done
        expect(resolved).toBe(true)
        expect(invalidateSpy).toHaveBeenCalled()
    })

    it('useApplyPicture polls an empty id list when the write reports no reindex job', async () => {
        applyPictureMock.mockResolvedValue({ ok: true, slot: 'folder', type: 'Front Cover' })
        const { mutation } = mountMutation(useApplyPicture)
        await mutation.mutateAsync(new FormData())
        expect(pollReindexMock).toHaveBeenCalledWith([], { signal: expect.any(AbortSignal) })
    })

    it('useApplyPicture warns when the polled re-index failed', async () => {
        applyPictureMock.mockResolvedValue({
            ok: true,
            slot: 'folder',
            type: 'Front Cover',
            reindex: { execution_id: 'x' }
        })
        pollReindexMock.mockResolvedValue({ failed: 1 })
        const { mutation } = mountMutation(useApplyPicture)
        await mutation.mutateAsync(new FormData())
        expect(reindexWarnings()).toEqual([expect.objectContaining({ severity: 'warn', life: 8000 })])
    })

    it('useApplyPicture stays silent when the re-index succeeded', async () => {
        applyPictureMock.mockResolvedValue({
            ok: true,
            slot: 'folder',
            type: 'Front Cover',
            reindex: { execution_id: 'x' }
        })
        pollReindexMock.mockResolvedValue({ failed: 0 })
        const { mutation } = mountMutation(useApplyPicture)
        await mutation.mutateAsync(new FormData())
        expect(reindexWarnings()).toEqual([])
    })

    it('useDeletePicture warns when the polled re-index failed', async () => {
        deletePictureMock.mockResolvedValue({ ok: true, reindex: { execution_id: 'y' } })
        pollReindexMock.mockResolvedValue({ failed: 1 })
        const { mutation } = mountMutation(useDeletePicture)
        await mutation.mutateAsync({
            scanFolder: 'Main',
            paths: ['Artist/Album/01.mp3'],
            type: 'Front Cover',
            slot: 'folder'
        })
        expect(pollReindexMock).toHaveBeenCalledWith(['y'], { signal: expect.any(AbortSignal) })
        expect(reindexWarnings()).toEqual([expect.objectContaining({ severity: 'warn', life: 8000 })])
    })

    it('useUpdateTracks warns when the polled re-index failed, alongside the per-row summary', async () => {
        updateTracksMock.mockResolvedValue({
            results: [{ path: 'a.mp3', ok: true }],
            reindex: { execution_id: 'z' }
        })
        pollReindexMock.mockResolvedValue({ failed: 1 })
        const { mutation } = mountMutation(useUpdateTracks)
        await mutation.mutateAsync({ scan_folder: 'Main', paths: ['a.mp3'], fields: { title: 'T' } })
        expect(pollReindexMock).toHaveBeenCalledWith(['z'], { signal: expect.any(AbortSignal) })
        expect(reindexWarnings()).toEqual([expect.objectContaining({ severity: 'warn', life: 8000 })])
        expect(toastAddSpy).toHaveBeenCalledWith(
            expect.objectContaining({ severity: 'success', summary: '1 of 1 saved' })
        )
    })

    // The edit session raises one aggregate warning per save and owns the cache
    // invalidation, so a quiet op must stay fully silent — no reindex warning and
    // no invalidation — or a multi-cell save stacks duplicates and refetches
    // mid-write.
    it('quiet suppresses the per-call warning and invalidation', async () => {
        applyPictureMock.mockResolvedValue({
            ok: true,
            slot: 'folder',
            type: 'Front Cover',
            reindex: { execution_id: 'x' }
        })
        pollReindexMock.mockResolvedValue({ failed: 1 })
        const { mutation, invalidateSpy } = mountMutation(() => useApplyPicture({ quiet: true }))
        await mutation.mutateAsync(new FormData())
        expect(reindexWarnings()).toEqual([])
        expect(invalidateSpy).not.toHaveBeenCalled()
    })

    // A quiet caller (the batch/session save) collects every write's execution
    // id itself and polls them all in ONE call at the end. If the mutation also
    // polled internally while quiet, a sequential per-cell batch would
    // serialize one poll per cell into its loop — exactly what quiet exists to
    // avoid. So a quiet mutationFn must skip pollReindex entirely and just hand
    // back the write result, reindex ref intact, for the caller to collect.
    it('useApplyPicture quiet skips the internal poll, still returning the reindex ref', async () => {
        applyPictureMock.mockResolvedValue({
            ok: true,
            slot: 'folder',
            type: 'Front Cover',
            reindex: { execution_id: 'x' }
        })
        const { mutation } = mountMutation(() => useApplyPicture({ quiet: true }))
        const out = await mutation.mutateAsync(new FormData())
        expect(pollReindexMock).not.toHaveBeenCalled()
        expect(out.reindex).toEqual({ execution_id: 'x' })
    })

    it('useApplyPicture non-quiet still polls internally', async () => {
        applyPictureMock.mockResolvedValue({
            ok: true,
            slot: 'folder',
            type: 'Front Cover',
            reindex: { execution_id: 'x' }
        })
        const { mutation } = mountMutation(useApplyPicture)
        await mutation.mutateAsync(new FormData())
        expect(pollReindexMock).toHaveBeenCalledWith(['x'], { signal: expect.any(AbortSignal) })
    })

    it('useDeletePicture quiet skips the internal poll, still returning the reindex ref', async () => {
        deletePictureMock.mockResolvedValue({ ok: true, reindex: { execution_id: 'y' } })
        const { mutation } = mountMutation(() => useDeletePicture({ quiet: true }))
        const out = await mutation.mutateAsync({
            scanFolder: 'Main',
            paths: ['Artist/Album/01.mp3'],
            type: 'Front Cover',
            slot: 'folder'
        })
        expect(pollReindexMock).not.toHaveBeenCalled()
        expect(out.reindex).toEqual({ execution_id: 'y' })
    })
})
