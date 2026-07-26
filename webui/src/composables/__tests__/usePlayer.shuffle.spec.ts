import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { nextTick } from 'vue'
import type { Song } from '@/types/subsonic'

const song = (id: string): Song => ({ id, title: `Song ${id}`, streamUrl: `/stream/${id}` })

/** jsdom has no media playback; stand in a minimal recorder (see usePlayer.preload.spec). */
class FakeAudio {
    static instances: FakeAudio[] = []
    _src = ''
    preload = ''
    volume = 1
    currentTime = 0
    duration = 0
    paused = true
    private listeners: Record<string, Array<(e?: unknown) => void>> = {}

    constructor() {
        FakeAudio.instances.push(this)
    }
    get src(): string {
        return this._src
    }
    set src(value: string) {
        this._src = value
    }
    addEventListener(type: string, cb: (e?: unknown) => void): void {
        ;(this.listeners[type] ??= []).push(cb)
    }
    removeEventListener(): void {}
    load(): void {}
    play(): Promise<void> {
        this.paused = false
        this.dispatch('play')
        return Promise.resolve()
    }
    pause(): void {
        this.paused = true
        this.dispatch('pause')
    }
    dispatch(type: string): void {
        ;(this.listeners[type] ?? []).forEach((cb) => cb({ target: this }))
    }
}

type UsePlayer = (typeof import('@/composables/usePlayer'))['usePlayer']
let usePlayer: UsePlayer

const album = ['A', 'B', 'C', 'D', 'E'].map(song)

beforeEach(async () => {
    FakeAudio.instances = []
    vi.stubGlobal('Audio', FakeAudio)
    localStorage.clear()
    vi.resetModules()
    ;({ usePlayer } = await import('@/composables/usePlayer'))
})

afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
})

describe('usePlayer shuffle order', () => {
    it('builds a random order over the queue without touching the queue itself', () => {
        const player = usePlayer()
        player.playAlbum(album, 0)

        player.toggleShuffle()

        expect(player.queue.value.map((s) => s.id)).toEqual(['A', 'B', 'C', 'D', 'E'])
        expect(player.currentIndex.value).toBe(0)
        expect(player.shuffleOrder.value[0]).toBe('A') // starts where playback is
        expect([...player.shuffleOrder.value].sort()).toEqual(['A', 'B', 'C', 'D', 'E'])
    })

    it('drops the order when shuffle is switched off', () => {
        const player = usePlayer()
        player.playAlbum(album, 0)
        player.toggleShuffle()
        player.toggleShuffle()

        expect(player.shuffleOrder.value).toEqual([])
    })

    it('walks the random order forward instead of the queue order', () => {
        const player = usePlayer()
        player.playAlbum(album, 0)
        player.toggleShuffle()
        const order = [...player.shuffleOrder.value]

        const heard: string[] = []
        for (let i = 1; i < order.length; i++) {
            player.playNext()
            heard.push(player.currentTrack.value?.id as string)
        }

        expect(heard).toEqual(order.slice(1))
    })

    it('walks the same order backward with previous', () => {
        const player = usePlayer()
        player.playAlbum(album, 0)
        player.toggleShuffle()
        const order = [...player.shuffleOrder.value]

        player.playNext()
        player.playNext()
        expect(player.currentTrack.value?.id).toBe(order[2])

        player.playPrevious()
        expect(player.currentTrack.value?.id).toBe(order[1])
        player.playPrevious()
        expect(player.currentTrack.value?.id).toBe(order[0])
    })

    it('returns to the same track after next then previous', () => {
        const player = usePlayer()
        player.playAlbum(album, 2)
        player.toggleShuffle()
        const start = player.currentTrack.value?.id

        player.playNext()
        expect(player.currentTrack.value?.id).not.toBe(start)
        player.playPrevious()

        expect(player.currentTrack.value?.id).toBe(start)
    })

    it('keeps the order stable across repeated next/previous walks', () => {
        const player = usePlayer()
        player.playAlbum(album, 0)
        player.toggleShuffle()

        player.playNext()
        const second = player.currentTrack.value?.id
        player.playNext()
        const third = player.currentTrack.value?.id

        player.playPrevious()
        player.playPrevious()
        player.playNext()
        expect(player.currentTrack.value?.id).toBe(second)
        player.playNext()
        expect(player.currentTrack.value?.id).toBe(third)
    })

    it('stops at the end of the order when repeat is none', () => {
        const player = usePlayer()
        player.playAlbum(album, 0)
        player.toggleShuffle()
        const order = [...player.shuffleOrder.value]

        for (let i = 1; i < order.length; i++) player.playNext()
        expect(player.currentTrack.value?.id).toBe(order[order.length - 1])
        expect(player.hasNext.value).toBe(false)

        player.playNext()
        expect(player.currentTrack.value?.id).toBe(order[order.length - 1])
        expect(player.isPlaying.value).toBe(false)
    })

    it('can still walk back from the end of the order', () => {
        const player = usePlayer()
        player.playAlbum(album, 0)
        player.toggleShuffle()
        const order = [...player.shuffleOrder.value]

        for (let i = 1; i < order.length; i++) player.playNext()
        player.playNext() // no-op at the end

        expect(player.hasPrevious.value).toBe(true)
        player.playPrevious()
        expect(player.currentTrack.value?.id).toBe(order[order.length - 2])
    })

    it('holds at the head of the order when previous is pressed there', () => {
        const player = usePlayer()
        player.playAlbum(album, 0)
        player.toggleShuffle()
        const head = player.shuffleOrder.value[0]

        expect(player.hasPrevious.value).toBe(false)
        player.playPrevious()

        expect(player.currentTrack.value?.id).toBe(head)
    })

    it('wraps around both ends of the order when repeat is all', () => {
        const player = usePlayer()
        player.playAlbum(album, 0)
        player.toggleShuffle()
        player.toggleRepeat() // none -> all
        const order = [...player.shuffleOrder.value]

        player.playPrevious() // from the head, backwards
        expect(player.currentTrack.value?.id).toBe(order[order.length - 1])

        player.playNext() // from the tail, forwards
        expect(player.currentTrack.value?.id).toBe(order[0])
    })

    it('preloads the next entry of the random order', () => {
        const player = usePlayer()
        player.playAlbum(album, 0)
        player.toggleShuffle()

        expect(player.preloadedTrack.value?.id).toBe(player.shuffleOrder.value[1])

        player.playNext()

        expect(player.preloadedTrack.value?.id).toBe(player.shuffleOrder.value[2])
    })

    it('jumps to the random next track when the current one ends on its own', () => {
        const player = usePlayer()
        player.playAlbum(album, 0)
        player.toggleShuffle()
        const expected = player.shuffleOrder.value[1]

        const elA = FakeAudio.instances.find((a) => a.src === '/stream/A')!
        elA.dispatch('ended')

        expect(player.currentTrack.value?.id).toBe(expected)
    })

    it('advances linearly when shuffle is off', () => {
        const player = usePlayer()
        player.playAlbum(album, 1)

        player.playNext()
        expect(player.currentTrack.value?.id).toBe('C')
        player.playPrevious()
        expect(player.currentTrack.value?.id).toBe('B')
    })

    it('continues the existing order after picking a track by hand', () => {
        const player = usePlayer()
        player.playAlbum(album, 0)
        player.toggleShuffle()
        const order = [...player.shuffleOrder.value]

        const target = order[2] as string
        player.playQueueItem(player.queue.value.findIndex((s) => s.id === target))

        expect(player.shuffleOrder.value).toEqual(order) // not redrawn
        player.playNext()
        expect(player.currentTrack.value?.id).toBe(order[3])
    })

    it('redraws the order around the picked track when a new album starts', () => {
        const player = usePlayer()
        player.playAlbum(album, 0)
        player.toggleShuffle()

        const next = ['X', 'Y', 'Z'].map(song)
        player.playAlbum(next, 1)

        expect(player.shuffleOrder.value[0]).toBe('Y')
        expect([...player.shuffleOrder.value].sort()).toEqual(['X', 'Y', 'Z'])
        expect(player.currentTrack.value?.id).toBe('Y')
    })

    it('splices queue additions into the not-yet-played part of the order', () => {
        const player = usePlayer()
        player.playAlbum(album, 0)
        player.toggleShuffle()
        const before = [...player.shuffleOrder.value]

        player.addMultipleToQueue([song('X'), song('Y')])

        const after = player.shuffleOrder.value
        expect([...after].sort()).toEqual(['A', 'B', 'C', 'D', 'E', 'X', 'Y'])
        expect(after[0]).toBe(before[0]) // the playing track stays at the front
        // Surviving entries keep their relative order.
        expect(after.filter((id) => before.includes(id))).toEqual(before)
    })

    it('removes tracks from the order when they leave the queue', () => {
        const player = usePlayer()
        player.playAlbum(album, 0)
        player.toggleShuffle()

        player.removeManyFromQueue([1, 3]) // drop B and D

        expect(player.shuffleOrder.value).not.toContain('B')
        expect(player.shuffleOrder.value).not.toContain('D')
        expect([...player.shuffleOrder.value].sort()).toEqual(['A', 'C', 'E'])
    })

    it('clears the order with the queue', () => {
        const player = usePlayer()
        player.playAlbum(album, 0)
        player.toggleShuffle()

        player.clearQueue()

        expect(player.shuffleOrder.value).toEqual([])
        expect(player.hasNext.value).toBe(false)
        expect(player.hasPrevious.value).toBe(false)
    })

    it('rebuilds a persisted order when the queue changed between sessions', async () => {
        const player = usePlayer()
        player.playAlbum(album, 0)
        player.toggleShuffle()
        await nextTick() // the persistence watcher flushes on the next tick
        const persisted = JSON.parse(
            localStorage.getItem('musicPlayer:shuffleOrder') as string
        ) as string[]
        expect([...persisted].sort()).toEqual(['A', 'B', 'C', 'D', 'E'])

        // Reload with the same stored state: the order survives intact.
        vi.resetModules()
        const { usePlayer: reloaded } = await import('@/composables/usePlayer')
        const restored = reloaded()

        expect(restored.shuffle.value).toBe(true)
        expect(restored.shuffleOrder.value).toEqual(persisted)
    })
})
