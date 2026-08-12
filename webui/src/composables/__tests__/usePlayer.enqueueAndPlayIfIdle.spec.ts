import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import type { Song } from '@/types/subsonic'

const song = (id: string): Song => ({ id, title: `Song ${id}`, streamUrl: `/stream/${id}` })

/** jsdom has no media playback; stand in a minimal recorder (see usePlayer.preload.spec). */
class FakeAudio {
    static instances: FakeAudio[] = []
    src = ''
    preload = ''
    volume = 1
    currentTime = 0
    duration = 0
    paused = true
    private listeners: Record<string, Array<(e?: unknown) => void>> = {}

    constructor() {
        FakeAudio.instances.push(this)
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

describe('usePlayer.enqueueAndPlayIfIdle', () => {
    it('appends to the end of the queue without disturbing playback', () => {
        const player = usePlayer()
        player.playAlbum(['A', 'B', 'C'].map(song), 1)

        player.enqueueAndPlayIfIdle([song('X')])

        expect(player.queue.value.map((s) => s.id)).toEqual(['A', 'B', 'C', 'X'])
        expect(player.currentIndex.value).toBe(1)
        expect(player.currentTrack.value?.id).toBe('B')
    })

    it('keeps appending in call order', () => {
        const player = usePlayer()
        player.playAlbum([song('A')], 0)

        player.enqueueAndPlayIfIdle([song('X')])
        player.enqueueAndPlayIfIdle([song('Y')])

        expect(player.queue.value.map((s) => s.id)).toEqual(['A', 'X', 'Y'])
    })

    it('starts playing the appended track when the queue was empty', () => {
        const player = usePlayer()

        player.enqueueAndPlayIfIdle([song('X')])

        expect(player.queue.value.map((s) => s.id)).toEqual(['X'])
        expect(player.currentIndex.value).toBe(0)
        expect(player.currentTrack.value?.id).toBe('X')
        expect(player.isPlaying.value).toBe(true)
    })

    it('starts playing when the queue was cleared mid-session', () => {
        const player = usePlayer()
        player.playAlbum(['A', 'B'].map(song), 0)
        player.clearQueue()

        player.enqueueAndPlayIfIdle([song('X')])

        expect(player.currentTrack.value?.id).toBe('X')
        expect(player.isPlaying.value).toBe(true)
    })

    it('does not resume a paused queue that still has a loaded track', () => {
        const player = usePlayer()
        player.playAlbum(['A', 'B'].map(song), 0)
        player.pause()

        player.enqueueAndPlayIfIdle([song('X')])

        expect(player.currentTrack.value?.id).toBe('A')
        expect(player.isPlaying.value).toBe(false)
    })

    it('does nothing for an empty song list', () => {
        const player = usePlayer()
        player.playAlbum([song('A')], 0)

        player.enqueueAndPlayIfIdle([])

        expect(player.queue.value.map((s) => s.id)).toEqual(['A'])
    })

    it('splices the addition into the not-yet-played part of the shuffle order', () => {
        const player = usePlayer()
        player.playAlbum(['A', 'B', 'C'].map(song), 0)
        player.toggleShuffle()
        const before = [...player.shuffleOrder.value]

        player.enqueueAndPlayIfIdle([song('X')])

        const after = player.shuffleOrder.value
        expect([...after].sort()).toEqual(['A', 'B', 'C', 'X'])
        expect(after[0]).toBe(before[0]) // the playing track stays at the front
        expect(after.filter((id) => before.includes(id))).toEqual(before)
    })

    it('draws a shuffle order around the appended track on an idle queue', () => {
        const player = usePlayer()
        player.playAlbum(['A', 'B'].map(song), 0)
        player.toggleShuffle()
        player.clearQueue()

        player.enqueueAndPlayIfIdle([song('X'), song('Y')])

        expect(player.shuffleOrder.value[0]).toBe('X')
        expect([...player.shuffleOrder.value].sort()).toEqual(['X', 'Y'])
        expect(player.currentTrack.value?.id).toBe('X')
    })
})
