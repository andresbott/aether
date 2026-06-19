import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'

const mutate = vi.fn()
const isPending = ref(false)
const toastAdd = vi.fn()
const queue = ref<{ id: string }[]>([])
const clearQueueFn = vi.fn()

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ queue, clearQueue: clearQueueFn })
}))
vi.mock('@/composables/useSubsonicQueries', () => ({
    useCreatePlaylist: () => ({ mutate, isPending })
}))
vi.mock('primevue/usetoast', () => ({
    useToast: () => ({ add: toastAdd })
}))

import { useQueueActions } from '@/composables/useQueueActions'

beforeEach(() => {
    mutate.mockReset()
    toastAdd.mockReset()
    clearQueueFn.mockReset()
    queue.value = []
    isPending.value = false
})

describe('useQueueActions', () => {
    it('openSaveDialog resets the name and shows the dialog', () => {
        const a = useQueueActions()
        a.playlistName.value = 'stale'
        a.openSaveDialog()
        expect(a.showSaveDialog.value).toBe(true)
        expect(a.playlistName.value).toBe('')
    })

    it('handleSave does nothing for a blank name', () => {
        const a = useQueueActions()
        a.playlistName.value = '   '
        a.handleSave()
        expect(mutate).not.toHaveBeenCalled()
    })

    it('handleSave sends the trimmed name and the queue song ids', () => {
        queue.value = [{ id: 'tr-1' }, { id: 'tr-2' }]
        const a = useQueueActions()
        a.playlistName.value = '  Road Trip  '
        a.handleSave()
        expect(mutate).toHaveBeenCalledWith(
            { name: 'Road Trip', songIds: ['tr-1', 'tr-2'] },
            expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) })
        )
    })

    it('success handler closes the dialog and shows a success toast', () => {
        queue.value = [{ id: 'tr-1' }]
        const a = useQueueActions()
        a.playlistName.value = 'Mix'
        a.showSaveDialog.value = true
        a.handleSave()
        const opts = mutate.mock.calls[0][1]
        opts.onSuccess()
        expect(a.showSaveDialog.value).toBe(false)
        expect(a.playlistName.value).toBe('')
        expect(toastAdd).toHaveBeenCalledWith(expect.objectContaining({ severity: 'success' }))
    })

    it('error handler shows an error toast with the message', () => {
        queue.value = [{ id: 'tr-1' }]
        const a = useQueueActions()
        a.playlistName.value = 'Mix'
        a.handleSave()
        const opts = mutate.mock.calls[0][1]
        opts.onError(new Error('boom'))
        expect(toastAdd).toHaveBeenCalledWith(
            expect.objectContaining({ severity: 'error', detail: 'boom' })
        )
    })

    it('clearQueue delegates to the player', () => {
        const a = useQueueActions()
        a.clearQueue()
        expect(clearQueueFn).toHaveBeenCalledTimes(1)
    })
})
