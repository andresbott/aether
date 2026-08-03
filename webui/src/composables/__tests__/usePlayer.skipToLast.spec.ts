import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import type { Song } from '@/types/subsonic'

const song = (id: string): Song => ({ id, title: `Song ${id}`, streamUrl: `/stream/${id}` })

// Same stand-in as usePlayer.preload.spec.ts: jsdom implements no media playback.
// `paused` is the state under test here, so pause()/play() maintain it faithfully.
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
    // A real element ABORTS playback when src is assigned or load() is called.
    // Modelling that is the whole point: it is what accidentally masks this bug
    // everywhere except at the end of the queue.
    set src(value: string) {
        this._src = value
        this.paused = true
        this.currentTime = 0
    }
    addEventListener(type: string, cb: (e?: unknown) => void): void {
        ;(this.listeners[type] ??= []).push(cb)
    }
    removeEventListener(): void {}
    load(): void {
        this.loadCalls++
        this.paused = true
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

    /** A track running out: a real element is already paused when `ended` fires. */
    end(): void {
        this.paused = true
        this.dispatch('ended')
    }
}

type UsePlayer = (typeof import('@/composables/usePlayer'))['usePlayer']
let usePlayer: UsePlayer

const elFor = (id: string): FakeAudio => FakeAudio.instances.find((a) => a.src === `/stream/${id}`)!

/** Every element that is currently sounding. More than one means overlap. */
const playing = (): string[] =>
    FakeAudio.instances.filter((a) => !a.paused && a.src).map((a) => a.src)

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

// Skipping forward hands playback to the pre-buffered standby element, but the
// outgoing one was never paused. Mid-queue that goes unnoticed, because the very
// next updatePreload() re-points the outgoing element's src and a real element
// stops when its src is reassigned. Stepping to the LAST track there is nothing
// to preload, so updatePreload() bails on `url === preloadedUrl` (both null,
// swapToStandby having just cleared it) — the outgoing element is never touched
// and keeps sounding under the new track.
describe('usePlayer skipping forward', () => {
    it('stops the outgoing track when skipping from the second-to-last to the last', () => {
        const player = usePlayer()
        player.playAlbum([song('A'), song('B'), song('C')], 1) // playing B, C is last

        expect(playing()).toEqual(['/stream/B'])

        player.playNext() // -> C, the last track

        expect(player.currentTrack.value?.id).toBe('C')
        expect(playing()).toEqual(['/stream/C'])
        expect(elFor('B').paused).toBe(true)
    })

    it('leaves only one track sounding when skipping through the whole queue', () => {
        const player = usePlayer()
        player.playAlbum([song('A'), song('B'), song('C'), song('D')], 0)

        for (const expected of ['B', 'C', 'D']) {
            player.playNext()
            expect(player.currentTrack.value?.id).toBe(expected)
            expect(playing()).toHaveLength(1)
        }
    })

    // The natural end-of-track path swaps through the same code, so it must not
    // regress: there the outgoing element has simply finished.
    it('still hands over cleanly when a track ends on its own', () => {
        const player = usePlayer()
        player.playAlbum([song('A'), song('B')], 0)

        elFor('A').end()

        expect(player.currentTrack.value?.id).toBe('B')
        expect(playing()).toEqual(['/stream/B'])
    })

    // Wrapping past the end with repeat on takes the same fast path.
    it('stops the outgoing track when wrapping from the last track with repeat all', () => {
        const player = usePlayer()
        player.toggleRepeat() // none -> all
        player.playAlbum([song('A'), song('B')], 1) // playing B, the last track

        player.playNext() // wraps to A

        expect(player.currentTrack.value?.id).toBe('A')
        expect(playing()).toEqual(['/stream/A'])
    })
})
