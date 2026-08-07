import { describe, it, expect, vi, beforeEach } from 'vitest'

// The mint returns whatever the test set, so both the recoverable and the
// dead-session paths can be driven.
const remintApiKey = vi.fn(() => Promise.resolve('ok' as 'ok' | 'failed' | 'session-gone'))
vi.mock('@/lib/subsonicSession', () => ({ remintApiKey }))

let streamKey = 'old'
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getStreamUrl: (id: string) => `https://example.test/stream/${id}?apiKey=${streamKey}`,
        scrobble: vi.fn(() => Promise.resolve())
    }
}))

// Audio stub that behaves like a real element on the one point under test:
// assigning a fresh src drops metadata, and setting currentTime before
// 'loadedmetadata' throws InvalidStateError (as browsers do).
class FakeAudio {
    volume = 1
    duration = 0
    preload = ''
    playCalls = 0
    loadCalls = 0
    private _src = ''
    private _currentTime = 0
    hasMetadata = false
    seekErrors = 0
    listeners: Record<string, Array<() => void>> = {}

    get src(): string {
        return this._src
    }
    set src(value: string) {
        this._src = value
        // A new resource resets the element: no metadata, position back to 0.
        this.hasMetadata = false
        this._currentTime = 0
    }
    get currentTime(): number {
        return this._currentTime
    }
    set currentTime(value: number) {
        if (!this.hasMetadata) {
            this.seekErrors++
            throw new DOMException('no metadata', 'InvalidStateError')
        }
        this._currentTime = value
    }
    /** Test helper: position the element as if it were already loaded. */
    seedPosition(seconds: number) {
        this.hasMetadata = true
        this._currentTime = seconds
    }
    addEventListener(type: string, cb: () => void, opts?: { once?: boolean }) {
        const wrapped = opts?.once
            ? () => {
                  this.listeners[type] = (this.listeners[type] ?? []).filter((c) => c !== wrapped)
                  cb()
              }
            : cb
        ;(this.listeners[type] ||= []).push(wrapped)
    }
    removeEventListener() {}
    load() {
        this.loadCalls++
    }
    play() {
        this.playCalls++
        return Promise.resolve()
    }
    pause() {}
    /** Drive the metadata arrival the browser would report after load(). */
    arriveMetadata(duration = 200) {
        this.hasMetadata = true
        this.duration = duration
        this.fire('loadedmetadata')
    }
    fire(type: string) {
        for (const cb of [...(this.listeners[type] ?? [])]) cb()
    }
}

const created: FakeAudio[] = []
vi.stubGlobal(
    'Audio',
    class {
        constructor() {
            const a = new FakeAudio()
            created.push(a)
            return a as unknown as HTMLAudioElement
        }
    }
)

const song = (id: string, duration: number) => ({ id, title: id, duration })

let usePlayer: (typeof import('@/composables/usePlayer'))['usePlayer']

beforeEach(async () => {
    localStorage.clear()
    created.length = 0
    streamKey = 'old'
    remintApiKey.mockClear()
    remintApiKey.mockResolvedValue('ok')
    vi.resetModules()
    usePlayer = (await import('@/composables/usePlayer')).usePlayer
})

const activeAudio = () => created[0]!

describe('usePlayer stream recovery', () => {
    // The regression this file exists for: assigning currentTime straight after
    // a fresh src happens before metadata loads, so the seek is dropped (or
    // throws). The position must be restored from a one-shot 'loadedmetadata'.
    it('restores the position only once metadata has loaded', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200)])
        const el = activeAudio()
        el.seedPosition(42)
        el.playCalls = 0

        streamKey = 'new'
        el.fire('error')
        await vi.waitFor(() => expect(remintApiKey).toHaveBeenCalledTimes(1))

        // The fresh src is installed with the new key, and NOTHING has tried to
        // seek yet — an eager assignment here is exactly the bug.
        expect(el.src).toContain('apiKey=new')
        expect(el.seekErrors).toBe(0)
        expect(el.currentTime).toBe(0)

        el.arriveMetadata()
        expect(el.currentTime).toBe(42)
    })

    it('resumes playback after the position is restored when it was playing', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200)])
        const el = activeAudio()
        el.seedPosition(10)
        el.fire('play') // isPlaying = true
        el.playCalls = 0

        el.fire('error')
        await vi.waitFor(() => expect(remintApiKey).toHaveBeenCalledTimes(1))
        // Not yet: play() waits for the restored position.
        expect(el.playCalls).toBe(0)

        el.arriveMetadata()
        expect(el.currentTime).toBe(10)
        expect(el.playCalls).toBe(1)
    })

    it('does not resume a paused element', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200)])
        const el = activeAudio()
        el.seedPosition(10)
        el.fire('pause') // isPlaying = false
        el.playCalls = 0

        el.fire('error')
        await vi.waitFor(() => expect(remintApiKey).toHaveBeenCalledTimes(1))
        el.arriveMetadata()

        expect(el.currentTime).toBe(10)
        expect(el.playCalls).toBe(0)
    })

    // One attempt per loaded track: a second error on the same track is a real
    // playback failure, not an expired credential.
    it('retries at most once per track', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200)])
        const el = activeAudio()
        el.seedPosition(5)

        el.fire('error')
        await vi.waitFor(() => expect(remintApiKey).toHaveBeenCalledTimes(1))
        el.arriveMetadata()

        el.fire('error')
        await Promise.resolve()
        expect(remintApiKey).toHaveBeenCalledTimes(1)
    })

    it('does not touch the element when the mint fails', async () => {
        remintApiKey.mockResolvedValue('failed')
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200)])
        const el = activeAudio()
        el.seedPosition(30)
        const srcBefore = el.src

        el.fire('error')
        await vi.waitFor(() => expect(remintApiKey).toHaveBeenCalledTimes(1))

        expect(el.src).toBe(srcBefore)
        expect(el.currentTime).toBe(30)
    })

    // clearQueue resets the per-track bookkeeping, so the next session's first
    // track gets a fresh recovery attempt rather than inheriting a spent one.
    it('re-arms the recovery budget after clearQueue', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200)])
        const el = activeAudio()
        el.seedPosition(5)
        el.fire('error')
        await vi.waitFor(() => expect(remintApiKey).toHaveBeenCalledTimes(1))

        player.clearQueue()
        player.playAlbum([song('tr-1', 200)])
        const next = activeAudio()
        next.seedPosition(0)
        next.fire('error')
        await vi.waitFor(() => expect(remintApiKey).toHaveBeenCalledTimes(2))
    })
})
