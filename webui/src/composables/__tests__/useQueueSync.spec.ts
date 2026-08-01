import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { nextTick } from 'vue'
import type { Song } from '@/types/subsonic'

const savePlayQueueMock = vi.fn(() => Promise.resolve())
const getPlayQueueMock = vi.fn(() => Promise.resolve(null as unknown))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getStreamUrl: (id: string) => `https://example.test/stream/${id}`,
        scrobble: vi.fn(() => Promise.resolve()),
        savePlayQueue: savePlayQueueMock,
        getPlayQueue: getPlayQueueMock
    }
}))

class FakeAudio {
    static instances: FakeAudio[] = []
    src = ''
    preload = ''
    volume = 1
    currentTime = 0
    duration = 0
    private listeners: Record<string, Array<() => void>> = {}
    constructor() {
        FakeAudio.instances.push(this)
    }
    addEventListener(type: string, cb: () => void): void {
        ;(this.listeners[type] ??= []).push(cb)
    }
    removeEventListener(): void {}
    load(): void {}
    play(): Promise<void> {
        this.dispatch('play')
        return Promise.resolve()
    }
    pause(): void {
        this.dispatch('pause')
    }
    dispatch(type: string): void {
        ;(this.listeners[type] ?? []).forEach((cb) => cb())
    }
}

const song = (id: string): Song => ({ id, title: `Song ${id}`, duration: 600 })

let usePlayer: (typeof import('@/composables/usePlayer'))['usePlayer']
let useQueueSync: (typeof import('@/composables/useQueueSync'))['useQueueSync']

beforeEach(async () => {
    vi.useFakeTimers()
    FakeAudio.instances = []
    vi.stubGlobal('Audio', FakeAudio)
    localStorage.clear()
    savePlayQueueMock.mockClear()
    getPlayQueueMock.mockClear()
    getPlayQueueMock.mockResolvedValue(null)
    vi.resetModules()
    ;({ usePlayer } = await import('@/composables/usePlayer'))
    ;({ useQueueSync } = await import('@/composables/useQueueSync'))
})

afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
})

const activeAudio = () => FakeAudio.instances[0]

describe('useQueueSync saving', () => {
    it('saves the queue after an edit', async () => {
        const player = usePlayer()
        const sync = useQueueSync()
        sync.start()

        player.addMultipleToQueue([song('tr-1'), song('tr-2')])
        await nextTick()
        await vi.runOnlyPendingTimersAsync()

        expect(savePlayQueueMock).toHaveBeenCalled()
        const [ids, index] = savePlayQueueMock.mock.calls.at(-1) as unknown as [string[], number, number]
        expect(ids).toEqual(['tr-1', 'tr-2'])
        expect(index).toBe(0)
    })

    // A burst of edits (dragging several rows, a multi-select removal) must not
    // fire one request per mutation.
    it('coalesces a burst of edits into a single save', async () => {
        const player = usePlayer()
        const sync = useQueueSync()
        sync.start()

        player.addToQueue(song('tr-1'))
        player.addToQueue(song('tr-2'))
        player.addToQueue(song('tr-3'))
        await nextTick()
        await vi.runOnlyPendingTimersAsync()

        expect(savePlayQueueMock).toHaveBeenCalledTimes(1)
        const [ids] = savePlayQueueMock.mock.calls[0] as unknown as [string[], number, number]
        expect(ids).toEqual(['tr-1', 'tr-2', 'tr-3'])
    })

    it('saves when the current track changes', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1'), song('tr-2')])
        const sync = useQueueSync()
        sync.start()
        await nextTick()
        await vi.runOnlyPendingTimersAsync()
        savePlayQueueMock.mockClear()

        player.playQueueItem(1)
        await nextTick()
        await vi.runOnlyPendingTimersAsync()

        const [, index] = savePlayQueueMock.mock.calls.at(-1) as unknown as [string[], number, number]
        expect(index).toBe(1)
    })

    // The position tick: a 10-minute track must be resumable to within 30s on
    // another device, which is what this interval buys.
    it('saves the playback position every 30 seconds while playing', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1')])
        const sync = useQueueSync()
        sync.start()
        await nextTick()
        await vi.runOnlyPendingTimersAsync()
        savePlayQueueMock.mockClear()

        const el = activeAudio()
        el.duration = 600
        el.currentTime = 125
        el.dispatch('timeupdate')
        el.dispatch('play')

        await vi.advanceTimersByTimeAsync(30_000)

        expect(savePlayQueueMock).toHaveBeenCalledTimes(1)
        const [, , position] = savePlayQueueMock.mock.calls[0] as unknown as [string[], number, number]
        expect(position).toBe(125_000)

        el.currentTime = 155
        el.dispatch('timeupdate')
        await vi.advanceTimersByTimeAsync(30_000)
        expect(savePlayQueueMock).toHaveBeenCalledTimes(2)
        const [, , later] = savePlayQueueMock.mock.calls[1] as unknown as [string[], number, number]
        expect(later).toBe(155_000)
    })

    // A paused player is not moving, so re-saving the same position every 30s
    // would be pure noise.
    it('does not tick while paused', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1')])
        const sync = useQueueSync()
        sync.start()
        await nextTick()
        await vi.runOnlyPendingTimersAsync()
        savePlayQueueMock.mockClear()

        activeAudio().dispatch('pause')
        await vi.advanceTimersByTimeAsync(90_000)

        expect(savePlayQueueMock).not.toHaveBeenCalled()
    })

    it('stops ticking after stop()', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1')])
        const sync = useQueueSync()
        sync.start()
        await nextTick()
        await vi.runOnlyPendingTimersAsync()
        activeAudio().dispatch('play')
        savePlayQueueMock.mockClear()

        sync.stop()
        await vi.advanceTimersByTimeAsync(120_000)

        expect(savePlayQueueMock).not.toHaveBeenCalled()
    })

    // Clearing the queue is a real state change and must reach the server, or the
    // next device would restore a queue the user deliberately emptied.
    it('saves an empty queue when the queue is cleared', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1')])
        const sync = useQueueSync()
        sync.start()
        await nextTick()
        await vi.runOnlyPendingTimersAsync()
        savePlayQueueMock.mockClear()

        player.clearQueue()
        await nextTick()
        await vi.runOnlyPendingTimersAsync()

        const [ids] = savePlayQueueMock.mock.calls.at(-1) as unknown as [string[], number, number]
        expect(ids).toEqual([])
    })
})

describe('useQueueSync restoring', () => {
    const savedQueue = {
        entry: [song('tr-10'), song('tr-11'), song('tr-12')],
        currentIndex: 1,
        position: 90_000,
        changedBy: 'other-device'
    }

    it('adopts the saved queue and seeks the current track to its position', async () => {
        getPlayQueueMock.mockResolvedValue(savedQueue)
        const player = usePlayer()
        const sync = useQueueSync()

        await sync.restore()

        expect(player.queue.value.map((s) => s.id)).toEqual(['tr-10', 'tr-11', 'tr-12'])
        expect(player.currentIndex.value).toBe(1)
        expect(player.currentTrack.value?.id).toBe('tr-11')
        expect(player.currentTime.value).toBe(90)
    })

    // Restoring must not start playback: browsers block autoplay, and resuming
    // audio unprompted on page load is hostile.
    it('restores paused', async () => {
        getPlayQueueMock.mockResolvedValue(savedQueue)
        const player = usePlayer()
        await useQueueSync().restore()

        expect(player.isPlaying.value).toBe(false)
    })

    // The saved offset belongs to the current track alone. Moving to any other
    // track in the restored queue must start at the beginning.
    it('applies the position to the current track only', async () => {
        getPlayQueueMock.mockResolvedValue(savedQueue)
        const player = usePlayer()
        await useQueueSync().restore()
        expect(player.currentTime.value).toBe(90)

        player.playQueueItem(2)
        expect(player.currentTime.value).toBe(0)

        player.playQueueItem(0)
        expect(player.currentTime.value).toBe(0)
    })

    // Restoring is not an edit; echoing it straight back would rewrite changedBy
    // and clobber the very state we just adopted.
    it('does not save the queue it just restored', async () => {
        getPlayQueueMock.mockResolvedValue(savedQueue)
        const sync = useQueueSync()
        sync.start()

        await sync.restore()
        await nextTick()
        await vi.runOnlyPendingTimersAsync()

        expect(savePlayQueueMock).not.toHaveBeenCalled()
    })

    it('leaves the local queue untouched when the server has no saved queue', async () => {
        getPlayQueueMock.mockResolvedValue(null)
        const player = usePlayer()
        player.playAlbum([song('tr-local')])

        await useQueueSync().restore()

        expect(player.queue.value.map((s) => s.id)).toEqual(['tr-local'])
    })

    // A queue saved without a current track (index -1) is still a queue; it just
    // starts from the top rather than resuming.
    it('starts from the first track when the saved queue has no current index', async () => {
        getPlayQueueMock.mockResolvedValue({
            entry: [song('tr-20'), song('tr-21')],
            currentIndex: -1,
            position: 0
        })
        const player = usePlayer()
        await useQueueSync().restore()

        expect(player.currentIndex.value).toBe(0)
        expect(player.currentTime.value).toBe(0)
    })

    // A server index beyond the entries it sent would leave the player pointing at
    // nothing playable.
    it('clamps an out-of-range current index', async () => {
        getPlayQueueMock.mockResolvedValue({
            entry: [song('tr-30'), song('tr-31')],
            currentIndex: 7,
            position: 5_000
        })
        const player = usePlayer()
        await useQueueSync().restore()

        expect(player.currentIndex.value).toBe(1)
    })
})
