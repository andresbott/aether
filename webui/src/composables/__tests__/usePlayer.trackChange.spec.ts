import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import type { Song } from '@/types/subsonic'

const song = (id: string, duration?: number): Song => ({
    id,
    title: `Song ${id}`,
    duration,
    streamUrl: `/stream/${id}`
})

// Whether a tap is being handled right now — see FakeAudio.play().
let tapping = false

/** Runs `fn` the way a tap's click handler would. */
const tap = (fn: () => void): void => {
    tapping = true
    try {
        fn()
    } finally {
        tapping = false
    }
}

/**
 * jsdom implements no media playback, so this stands in for the element, under
 * the strict reading of mobile autoplay policy (mobile Safari's): an element
 * may start sound outside a tap only once a tap has started it. Assigning `src`
 * behaves as on a real element: playback stops, with no `pause` event, and the
 * old stream's metadata is gone.
 */
class FakeAudio {
    static instances: FakeAudio[] = []
    private _src = ''
    preload = ''
    volume = 1
    currentTime = 0
    duration = NaN
    paused = true
    // Set by the first play() inside a tap; from then on play() needs none.
    unlocked = false
    private listeners: Record<string, Array<() => void>> = {}

    constructor() {
        FakeAudio.instances.push(this)
    }
    get src(): string {
        return this._src
    }
    set src(value: string) {
        this._src = value
        this.paused = true
        this.currentTime = 0
        this.duration = NaN
    }
    addEventListener(type: string, cb: () => void): void {
        ;(this.listeners[type] ??= []).push(cb)
    }
    removeEventListener(): void {}
    load(): void {
        this.paused = true
    }
    play(): Promise<void> {
        if (!this.unlocked && !tapping) {
            return Promise.reject(new DOMException('play() needs a tap', 'NotAllowedError'))
        }
        this.unlocked = true
        this.paused = false
        this.dispatch('play')
        return Promise.resolve()
    }
    pause(): void {
        this.paused = true
        this.dispatch('pause')
    }
    dispatch(type: string): void {
        ;(this.listeners[type] ?? []).forEach((cb) => cb())
    }

    /** The track running out: a real element pauses, then reports `ended`. */
    end(): void {
        this.paused = true
        this.dispatch('pause')
        this.dispatch('ended')
    }

    /** The stream's metadata arriving. */
    reportDuration(seconds: number): void {
        this.duration = seconds
        this.dispatch('durationchange')
    }
}

/** The elements making sound right now. */
const sounding = (): FakeAudio[] => FakeAudio.instances.filter((a) => !a.paused)

type UsePlayer = (typeof import('@/composables/usePlayer'))['usePlayer']
let usePlayer: UsePlayer

beforeEach(async () => {
    FakeAudio.instances = []
    tapping = false
    vi.stubGlobal('Audio', FakeAudio)
    localStorage.clear()
    vi.resetModules()
    ;({ usePlayer } = await import('@/composables/usePlayer'))
})

afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
})

// A phone keeps an album going only if every later track plays on the element
// the user's tap started: nobody taps at a track boundary, and the screen may
// be locked. The assertions run straight after the event, with no await, so
// the next track must also start within the `ended` handler itself.
describe('usePlayer moving between tracks', () => {
    it('carries on into the next track on the element the tap started', () => {
        const player = usePlayer()
        tap(() => player.playAlbum([song('A'), song('B'), song('C')], 0))
        const el = sounding()[0]!

        el.end()

        expect(player.currentTrack.value?.id).toBe('B')
        expect(el.src).toBe('/stream/B')
        expect(el.paused).toBe(false)
        expect(player.isPlaying.value).toBe(true)
    })

    it('wraps to the first track on the same element when repeat is on', () => {
        const player = usePlayer()
        player.toggleRepeat() // none -> all
        tap(() => player.playAlbum([song('A'), song('B')], 1)) // B is last
        const el = sounding()[0]!

        el.end()

        expect(player.currentTrack.value?.id).toBe('A')
        expect(el.src).toBe('/stream/A')
        expect(el.paused).toBe(false)
    })

    it('stops on the last track when the queue runs out', () => {
        const player = usePlayer()
        tap(() => player.playAlbum([song('A'), song('B')], 1)) // B is last
        const el = sounding()[0]!

        el.end()

        expect(player.currentTrack.value?.id).toBe('B')
        expect(sounding()).toEqual([])
        expect(player.isPlaying.value).toBe(false)
    })

    it('plays whatever follows the current track once it ends, after a reorder', () => {
        const player = usePlayer()
        tap(() => player.playAlbum([song('A'), song('B'), song('C')], 0))
        const el = sounding()[0]!

        player.moveInQueue([2], 1) // [A, C, B]: C now follows A
        el.end()

        expect(player.currentTrack.value?.id).toBe('C')
        expect(el.src).toBe('/stream/C')
        expect(el.paused).toBe(false)
    })

    it('skips to the next track on the same element', () => {
        const player = usePlayer()
        tap(() => player.playAlbum([song('A'), song('B'), song('C')], 0))
        const el = sounding()[0]!

        tap(() => player.playNext())

        expect(player.currentTrack.value?.id).toBe('B')
        expect(el.src).toBe('/stream/B')
        expect(sounding()).toHaveLength(1)
        expect(el.paused).toBe(false)
        expect(player.isPlaying.value).toBe(true)
    })

    it("shows the next track's length while its stream loads", () => {
        const player = usePlayer()
        tap(() => player.playAlbum([song('A', 200), song('B', 300)], 0))
        const el = sounding()[0]!
        el.reportDuration(200.4)

        el.end()

        // The catalog's length until the stream reports its own.
        expect(player.duration.value).toBe(300)
        el.reportDuration(300.6)
        expect(player.duration.value).toBe(300.6)
    })
})
