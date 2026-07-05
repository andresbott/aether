import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import type { Song } from '@/types/subsonic'

const song = (id: string): Song => ({ id, title: `Song ${id}`, streamUrl: `/stream/${id}` })

/**
 * jsdom does not implement HTMLMediaElement playback, so we stand in a fake
 * audio element that records src/preload/load/play. Two instances are created
 * by the player (active + standby); tests locate them by their src.
 */
class FakeAudio {
    static instances: FakeAudio[] = []
    _src = ''
    preload = ''
    volume = 1
    currentTime = 0
    duration = 0
    paused = true
    playCalls = 0
    loadCalls = 0
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
    load(): void {
        this.loadCalls++
    }
    play(): Promise<void> {
        this.playCalls++
        this.paused = false
        this.dispatch('play')
        return Promise.resolve()
    }
    pause(): void {
        this.paused = true
        this.dispatch('pause')
    }
    dispatch(type: string, event: unknown = { target: this }): void {
        ;(this.listeners[type] ?? []).forEach((cb) => cb(event))
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
})

describe('usePlayer gapless preloading', () => {
    it('preloads the next track onto a standby element when an album starts', () => {
        const player = usePlayer()
        player.playAlbum([song('A'), song('B'), song('C')], 0)

        expect(player.currentTrack.value?.id).toBe('A')
        expect(player.preloadedTrack.value?.id).toBe('B')

        const buffered = FakeAudio.instances.find((a) => a.src === '/stream/B')
        expect(buffered).toBeDefined()
        expect(buffered?.preload).toBe('auto')
    })

    it('does not preload past the last track when repeat is none', () => {
        const player = usePlayer()
        player.playAlbum([song('A'), song('B')], 1) // B is last

        expect(player.preloadedTrack.value).toBeNull()
    })

    it('wraps the preload to the first track when repeat is all', () => {
        const player = usePlayer()
        player.toggleRepeat() // none -> all
        player.playAlbum([song('A'), song('B')], 1) // B is last

        expect(player.repeat.value).toBe('all')
        expect(player.preloadedTrack.value?.id).toBe('A')
    })

    it('advances to and plays the preloaded element when the current track ends', () => {
        const player = usePlayer()
        player.playAlbum([song('A'), song('B'), song('C')], 0)

        const elB = FakeAudio.instances.find((a) => a.src === '/stream/B')!
        const playsBefore = elB.playCalls
        const elA = FakeAudio.instances.find((a) => a.src === '/stream/A')!

        elA.dispatch('ended') // current track finishes

        expect(player.currentTrack.value?.id).toBe('B')
        expect(player.currentIndex.value).toBe(1)
        expect(elB.playCalls).toBe(playsBefore + 1) // the already-buffered element started
        expect(player.preloadedTrack.value?.id).toBe('C') // the following track is queued up
    })

    it('reports the new track duration after a gapless swap', () => {
        const player = usePlayer()
        player.playAlbum([song('A'), song('B')], 0)

        const elA = FakeAudio.instances.find((a) => a.src === '/stream/A')!
        const elB = FakeAudio.instances.find((a) => a.src === '/stream/B')!

        // The standby element (B) loads its metadata while buffering ahead, but
        // the durationchange is ignored because B is not active yet.
        elB.duration = 200
        elB.dispatch('durationchange')
        expect(player.duration.value).toBe(0)

        elA.dispatch('ended') // swap to the already-buffered element

        expect(player.duration.value).toBe(200)
    })

    it('re-points the preload when the queue is reordered', () => {
        const player = usePlayer()
        player.playAlbum([song('A'), song('B'), song('C')], 0)
        expect(player.preloadedTrack.value?.id).toBe('B')

        player.moveInQueue([2], 1) // [A, C, B] — C now follows the current track A

        expect(player.preloadedTrack.value?.id).toBe('C')
    })
})
