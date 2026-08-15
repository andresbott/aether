import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { nextTick } from 'vue'
import type { Song } from '@/types/subsonic'

const savePlayQueueMock = vi.fn(() => Promise.resolve())
const getPlayQueueMock = vi.fn(() => Promise.resolve(null as unknown))
const savePlayQueueBeaconMock = vi.fn(() => true)

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getStreamUrl: (id: string) => `https://example.test/stream/${id}`,
        scrobble: vi.fn(() => Promise.resolve()),
        savePlayQueue: savePlayQueueMock,
        savePlayQueueBeacon: savePlayQueueBeaconMock,
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
// The sync started by the test under way, torn down in afterEach (see below).
let activeSync: { start: () => void; stop: () => void } | null = null

beforeEach(async () => {
    vi.useFakeTimers()
    FakeAudio.instances = []
    vi.stubGlobal('Audio', FakeAudio)
    localStorage.clear()
    savePlayQueueMock.mockClear()
    savePlayQueueBeaconMock.mockClear()
    getPlayQueueMock.mockClear()
    getPlayQueueMock.mockResolvedValue(null)
    vi.resetModules()
    ;({ usePlayer } = await import('@/composables/usePlayer'))
    ;({ useQueueSync } = await import('@/composables/useQueueSync'))
})

afterEach(() => {
    // Each test gets a fresh module via resetModules(), but jsdom's window is shared
    // across the file — so a sync left running would leave its pagehide listener
    // attached and every later test would see extra beacons.
    activeSync?.stop()
    activeSync = null
    vi.useRealTimers()
    vi.unstubAllGlobals()
})

const activeAudio = () => FakeAudio.instances[0]

// Every sync a test creates goes through here so afterEach can tear it down.
const startSync = () => {
    activeSync = useQueueSync()
    return activeSync as ReturnType<typeof useQueueSync>
}

describe('useQueueSync saving', () => {
    it('saves the queue after an edit', async () => {
        const player = usePlayer()
        const sync = startSync()
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
        const sync = startSync()
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
        const sync = startSync()
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
        const sync = startSync()
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

    // The pause itself saves (above), but a paused player is not moving — so the
    // ticker must not keep re-sending that same offset every 30s afterwards.
    it('does not keep ticking while paused', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1')])
        const sync = startSync()
        sync.start()
        await nextTick()
        await vi.runOnlyPendingTimersAsync()
        savePlayQueueMock.mockClear()

        activeAudio().dispatch('pause')
        await vi.runOnlyPendingTimersAsync()
        const afterPause = savePlayQueueMock.mock.calls.length
        expect(afterPause).toBe(1) // the pause save

        await vi.advanceTimersByTimeAsync(90_000)
        expect(savePlayQueueMock.mock.calls.length).toBe(afterPause)
    })

    it('stops ticking after stop()', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1')])
        const sync = startSync()
        sync.start()
        await nextTick()
        await vi.runOnlyPendingTimersAsync()
        activeAudio().dispatch('play')
        savePlayQueueMock.mockClear()

        sync.stop()
        await vi.advanceTimersByTimeAsync(120_000)

        expect(savePlayQueueMock).not.toHaveBeenCalled()
    })

    // Pausing is the moment a listener stops, and no tick runs while paused — so
    // without this the saved offset stays at the last tick, up to 30s behind where
    // the user actually stopped.
    it('saves the position immediately on pause', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1')])
        const sync = startSync()
        sync.start()
        await nextTick()
        await vi.runOnlyPendingTimersAsync()

        const el = activeAudio()
        el.duration = 600
        el.dispatch('play')
        el.currentTime = 105
        el.dispatch('timeupdate')
        savePlayQueueMock.mockClear()

        el.dispatch('pause')
        await vi.runOnlyPendingTimersAsync()

        expect(savePlayQueueMock).toHaveBeenCalledTimes(1)
        const [, index, position] = savePlayQueueMock.mock.calls[0] as unknown as [
            string[],
            number,
            number
        ]
        expect(index).toBe(0)
        expect(position).toBe(105_000)
    })

    // The pause save must not be debounced away by the edit path, and it must not
    // be skipped just because the queue shape is unchanged — only the position moved.
    it('saves on pause even though the queue shape did not change', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1'), song('tr-2')])
        const sync = startSync()
        sync.start()
        await nextTick()
        await vi.runOnlyPendingTimersAsync()

        const el = activeAudio()
        el.duration = 600
        el.dispatch('play')
        el.currentTime = 42
        el.dispatch('timeupdate')
        savePlayQueueMock.mockClear()

        el.dispatch('pause')
        await vi.runOnlyPendingTimersAsync()

        expect(savePlayQueueMock).toHaveBeenCalledTimes(1)
        const [, , position] = savePlayQueueMock.mock.calls[0] as unknown as [
            string[],
            number,
            number
        ]
        expect(position).toBe(42_000)
    })

    // Closing the tab cancels in-flight fetches, so the final write has to go out
    // as a beacon or it never lands.
    it('beacons the position when the page is hidden', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1')])
        const sync = startSync()
        sync.start()
        await nextTick()
        await vi.runOnlyPendingTimersAsync()

        const el = activeAudio()
        el.duration = 600
        el.dispatch('play')
        el.currentTime = 200
        el.dispatch('timeupdate')

        window.dispatchEvent(new Event('pagehide'))

        expect(savePlayQueueBeaconMock).toHaveBeenCalledTimes(1)
        const [ids, index, position] = savePlayQueueBeaconMock.mock.calls[0] as unknown as [
            string[],
            number,
            number
        ]
        expect(ids).toEqual(['tr-1'])
        expect(index).toBe(0)
        expect(position).toBe(200_000)
    })

    // THE REGRESSION: a tab that only ever restored somebody else's queue must not
    // beacon on unload. Its position is whatever restore() handed it, so writing it
    // back overwrites a newer save from another browser with a stale offset — and a
    // beacon lands ~10ms after that save, so it always wins the race.
    it('does not beacon when this tab never played anything', async () => {
        getPlayQueueMock.mockResolvedValue({
            entry: [song('tr-1'), song('tr-2')],
            currentIndex: 0,
            position: 8_669
        })
        usePlayer()
        const sync = startSync()
        await sync.restore()
        sync.start()
        await nextTick()
        await vi.runOnlyPendingTimersAsync()

        window.dispatchEvent(new Event('pagehide'))

        expect(savePlayQueueBeaconMock).not.toHaveBeenCalled()
    })

    // A tab that DID play is the authority on its own position, so it must still
    // beacon — that is the whole point of the unload save.
    it('beacons after this tab actually played', async () => {
        getPlayQueueMock.mockResolvedValue({
            entry: [song('tr-1')],
            currentIndex: 0,
            position: 8_669
        })
        usePlayer()
        const sync = startSync()
        await sync.restore()
        sync.start()
        await nextTick()
        await vi.runOnlyPendingTimersAsync()

        const el = activeAudio()
        el.duration = 600
        el.dispatch('play')
        el.currentTime = 250
        el.dispatch('timeupdate')

        window.dispatchEvent(new Event('pagehide'))

        expect(savePlayQueueBeaconMock).toHaveBeenCalledTimes(1)
        const [, , position] = savePlayQueueBeaconMock.mock.calls[0] as unknown as [
            string[],
            number,
            number
        ]
        expect(position).toBe(250_000)
    })

    // An empty queue has nothing to report; beaconing on every tab close would
    // clobber a queue saved from another device with an empty one.
    it('does not beacon when the queue is empty', async () => {
        usePlayer()
        const sync = startSync()
        sync.start()
        await nextTick()
        await vi.runOnlyPendingTimersAsync()

        window.dispatchEvent(new Event('pagehide'))

        expect(savePlayQueueBeaconMock).not.toHaveBeenCalled()
    })

    // After stop() the layout is gone; a beacon fired then would write state the
    // player no longer owns.
    it('stops beaconing after stop()', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1')])
        const sync = startSync()
        sync.start()
        await nextTick()
        await vi.runOnlyPendingTimersAsync()

        sync.stop()
        window.dispatchEvent(new Event('pagehide'))

        expect(savePlayQueueBeaconMock).not.toHaveBeenCalled()
    })

    // start() after a stop() must work: the layout unmounts and remounts on any
    // navigation into /settings and back, and a sync that stayed dead would silently
    // stop persisting the queue for the rest of the session.
    it('resumes syncing after stop() then start()', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1')])
        const sync = startSync()
        sync.start()
        await nextTick()
        await vi.runOnlyPendingTimersAsync()

        sync.stop()
        sync.start()
        savePlayQueueMock.mockClear()
        savePlayQueueBeaconMock.mockClear()

        // The ticker is live again...
        activeAudio().dispatch('play')
        await vi.advanceTimersByTimeAsync(30_000)
        expect(savePlayQueueMock).toHaveBeenCalled()

        // ...and so is the unload beacon, exactly once.
        window.dispatchEvent(new Event('pagehide'))
        expect(savePlayQueueBeaconMock).toHaveBeenCalledTimes(1)
    })

    // Clearing the queue is a real state change and must reach the server, or the
    // next device would restore a queue the user deliberately emptied.
    it('saves an empty queue when the queue is cleared', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1')])
        const sync = startSync()
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
        const sync = startSync()

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
        await startSync().restore()

        expect(player.isPlaying.value).toBe(false)
    })

    // The saved offset belongs to the current track alone. Moving to any other
    // track in the restored queue must start at the beginning.
    it('applies the position to the current track only', async () => {
        getPlayQueueMock.mockResolvedValue(savedQueue)
        const player = usePlayer()
        await startSync().restore()
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
        const sync = startSync()
        sync.start()

        await sync.restore()
        await nextTick()
        await vi.runOnlyPendingTimersAsync()

        expect(savePlayQueueMock).not.toHaveBeenCalled()
    })

    // THE REGRESSION: PlayerLayout remounts on every trip through /settings and
    // its onMounted calls restore() again. Re-adopting the server snapshot into a
    // session that is already playing re-points the active element's src, which
    // stops playback dead. restore() is a page-load concern: once per load.
    it('restores only once per page load', async () => {
        getPlayQueueMock.mockResolvedValue(savedQueue)
        const player = usePlayer()
        const sync = startSync()
        await sync.restore()
        sync.start()

        // The user plays a different track than the restored one...
        player.playQueueItem(2)
        await nextTick()

        // ...then round-trips through /settings: the layout remounts and runs
        // the restore/start sequence again.
        sync.stop()
        await sync.restore()
        sync.start()

        expect(getPlayQueueMock).toHaveBeenCalledTimes(1)
        expect(player.currentIndex.value).toBe(2)
        expect(player.isPlaying.value).toBe(true)
    })

    it('leaves the local queue untouched when the server has no saved queue', async () => {
        getPlayQueueMock.mockResolvedValue(null)
        const player = usePlayer()
        player.playAlbum([song('tr-local')])

        await startSync().restore()

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
        await startSync().restore()

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
        await startSync().restore()

        expect(player.currentIndex.value).toBe(1)
    })
})
