import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

// The session check answers whatever the test set, so the reachable, the
// unreachable and the refused paths can all be driven.
type SessionCheck = 'ok' | 'unreachable' | 'refused'
const { checkSession, reportNetworkError, reconnectListeners, httpFetch } = vi.hoisted(() => ({
    checkSession: vi.fn<() => Promise<SessionCheck>>(),
    reportNetworkError: vi.fn(),
    reconnectListeners: [] as Array<() => void>,
    httpFetch: vi.fn()
}))

vi.mock('@/composables/useConnectivity', () => ({
    reportNetworkError,
    onReconnect: (cb: () => void) => {
        reconnectListeners.push(cb)
        return () => {}
    },
    dismissBanner: vi.fn(),
    markOnline: vi.fn(),
    useConnectivity: () => ({ isOffline: { value: false } })
}))

vi.mock('@/lib/host', () => ({ httpFetch }))

let streamKey = 'old'
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getStreamUrl: (id: string) => `https://example.test/stream/${id}?apiKey=${streamKey}`,
        scrobble: vi.fn(() => Promise.resolve()),
        checkSession
    }
}))

// Audio stub that behaves like a real element on the points under test:
// assigning a fresh src drops metadata, setting currentTime before
// 'loadedmetadata' throws InvalidStateError (as browsers do), and play() can be
// made to refuse.
class FakeAudio {
    volume = 1
    duration = 0
    preload = ''
    paused = true
    ended = false
    error: { code: number } | null = null
    playCalls = 0
    loadCalls = 0
    playResult: () => Promise<void> = () => Promise.resolve()
    private _src = ''
    private _currentTime = 0
    hasMetadata = false
    seekErrors = 0
    listeners: Record<string, Array<(e: Event) => void>> = {}

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
    addEventListener(type: string, cb: (e: Event) => void, opts?: { once?: boolean }) {
        const wrapped = opts?.once
            ? (e: Event) => {
                  this.listeners[type] = (this.listeners[type] ?? []).filter((c) => c !== wrapped)
                  cb(e)
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
        return this.playResult()
    }
    pause() {}
    /** Drive the metadata arrival the browser would report after load(). */
    arriveMetadata(duration = 200) {
        this.hasMetadata = true
        this.duration = duration
        this.fire('loadedmetadata')
    }
    /** A load failure; MediaError code 2 is MEDIA_ERR_NETWORK, 4 SRC_NOT_SUPPORTED. */
    fail(code = 4) {
        this.error = { code }
        this.fire('error')
    }
    /** Playback reaching `seconds`, as the browser reports it. */
    progress(seconds: number) {
        this._currentTime = seconds
        this.fire('timeupdate')
    }
    fire(type: string) {
        for (const cb of [...(this.listeners[type] ?? [])]) cb({ type } as Event)
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

/** Let pending promises and timers up to `ms` run. */
const settle = (ms = 0) => vi.advanceTimersByTimeAsync(ms)

beforeEach(async () => {
    vi.useFakeTimers()
    localStorage.clear()
    created.length = 0
    reconnectListeners.length = 0
    streamKey = 'old'
    checkSession.mockReset()
    checkSession.mockResolvedValue('ok')
    reportNetworkError.mockClear()
    httpFetch.mockReset()
    vi.resetModules()
    usePlayer = (await import('@/composables/usePlayer')).usePlayer
})

afterEach(() => {
    vi.useRealTimers()
})

const activeAudio = () => created[0]!

describe('usePlayer stream recovery', () => {
    // Assigning currentTime straight after a fresh src happens before metadata
    // loads, so the seek is dropped (or throws). The position must be restored
    // from a one-shot 'loadedmetadata'.
    it('restores the position only once metadata has loaded', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200)])
        const el = activeAudio()
        el.seedPosition(42)

        streamKey = 'new'
        el.fail()
        await settle()

        // The session was checked (re-minting an expired key on the way), the
        // fresh src carries the new key, and NOTHING has tried to seek yet.
        expect(checkSession).toHaveBeenCalledTimes(1)
        expect(el.src).toContain('apiKey=new')
        expect(el.seekErrors).toBe(0)
        expect(el.currentTime).toBe(0)

        el.arriveMetadata()
        expect(el.currentTime).toBe(42)
    })

    it('resumes playback after the position is restored', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200)])
        const el = activeAudio()
        el.seedPosition(10)
        el.playCalls = 0

        el.fail()
        await settle()
        // Not yet: play() waits for the restored position.
        expect(el.playCalls).toBe(0)

        el.arriveMetadata()
        expect(el.currentTime).toBe(10)
        expect(el.playCalls).toBe(1)
    })

    // A pause the user (or the system: headphones out, a call) made means no
    // sound is wanted, so a recovered stream is left paused.
    it('does not resume after a pause', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200)])
        const el = activeAudio()
        el.seedPosition(10)
        el.fire('pause')
        el.playCalls = 0

        el.fail()
        await settle()
        el.arriveMetadata()

        expect(el.currentTime).toBe(10)
        expect(el.playCalls).toBe(0)
    })

    it('waits for the network instead of touching the element when the server is unreachable', async () => {
        checkSession.mockResolvedValue('unreachable')
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200)])
        const el = activeAudio()
        el.seedPosition(30)
        const srcBefore = el.src

        el.fail(2)
        await settle()

        expect(reportNetworkError).toHaveBeenCalledTimes(1)
        expect(el.src).toBe(srcBefore)
        expect(el.currentTime).toBe(30)

        // The backoff runs out and the server answers again.
        checkSession.mockResolvedValue('ok')
        await settle(1_000)
        expect(checkSession).toHaveBeenCalledTimes(2)
        expect(el.loadCalls).toBe(1)
        el.arriveMetadata()
        expect(el.currentTime).toBe(30)
    })

    it('tries again as soon as connectivity comes back', async () => {
        checkSession.mockResolvedValue('unreachable')
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200)])
        const el = activeAudio()
        el.seedPosition(30)

        el.fail(2)
        await settle()
        expect(checkSession).toHaveBeenCalledTimes(1)

        checkSession.mockResolvedValue('ok')
        reconnectListeners.forEach((cb) => cb())
        await settle()

        expect(checkSession).toHaveBeenCalledTimes(2)
        expect(el.loadCalls).toBe(1)
    })

    it('leaves a refused session to the session handling', async () => {
        checkSession.mockResolvedValue('refused')
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200)])
        const el = activeAudio()
        el.seedPosition(30)
        const srcBefore = el.src

        el.fail()
        await settle(60_000)

        expect(checkSession).toHaveBeenCalledTimes(1)
        expect(reportNetworkError).not.toHaveBeenCalled()
        expect(el.src).toBe(srcBefore)
    })

    // The server answers, the key is good, and the track still will not load:
    // the file is unplayable. Skip it rather than retry forever.
    it('skips a track that keeps failing while the server answers', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200), song('tr-2', 200)])
        const el = activeAudio()

        el.fail()
        await settle()
        expect(player.currentTrack.value?.id).toBe('tr-1')

        el.fail()
        await settle(1_000)
        expect(player.currentTrack.value?.id).toBe('tr-1')

        el.fail()
        await settle(2_000)
        expect(player.currentTrack.value?.id).toBe('tr-2')
    })

    // A transfer that broke off says nothing about the file: keep retrying.
    it('never skips a track for network errors', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200), song('tr-2', 200)])
        const el = activeAudio()

        for (const wait of [0, 1_000, 2_000, 4_000]) {
            el.fail(2)
            await settle(wait)
        }
        expect(player.currentTrack.value?.id).toBe('tr-1')
        expect(el.loadCalls).toBe(4)
    })

    // Regression: clearQueue empties the audio element's src, and the browser
    // fires an 'error' for the now-sourceless element. That is a deliberate stop,
    // not a connectivity failure.
    it('does nothing for the error a cleared queue fires', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200)])
        const el = activeAudio()
        el.seedPosition(5)

        player.clearQueue() // sets currentTrack = null and audio.src = ''
        el.fire('error')
        await settle(60_000)

        expect(checkSession).not.toHaveBeenCalled()
        expect(reportNetworkError).not.toHaveBeenCalled()
    })

    it('drops a recovery the next track overtook', async () => {
        let answer!: (v: SessionCheck) => void
        checkSession.mockReturnValue(new Promise<SessionCheck>((r) => (answer = r)))
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200), song('tr-2', 200)])
        const el = activeAudio()

        el.fail()
        await settle()
        player.playNext()
        const loadsBefore = el.loadCalls
        answer('ok')
        await settle()

        expect(el.src).toContain('tr-2')
        expect(el.loadCalls).toBe(loadsBefore)
    })
})

describe('usePlayer stall watchdog', () => {
    it('recovers a load that sits waiting without progress', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200)])
        const el = activeAudio()
        el.paused = false
        // Playing, at 12s — the progress stands the start-of-play watch down.
        el.progress(12)

        el.fire('waiting')
        await settle(14_000)
        expect(checkSession).not.toHaveBeenCalled()

        await settle(1_000)
        expect(checkSession).toHaveBeenCalledTimes(1)
        expect(el.loadCalls).toBe(1)
    })

    it('stands down once playback moves again', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200)])
        const el = activeAudio()
        el.paused = false
        // Playing, at 12s — the progress stands the start-of-play watch down.
        el.progress(12)

        el.fire('waiting')
        await settle(5_000)
        el.progress(13)
        await settle(20_000)

        expect(checkSession).not.toHaveBeenCalled()
    })

    it('does not watch a paused player', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200)])
        const el = activeAudio()
        player.pause()

        el.fire('stalled')
        await settle(30_000)

        expect(checkSession).not.toHaveBeenCalled()
    })

    // The autoplay policy refused: only a tap can start sound, so retrying would
    // just loop.
    it('gives up on a play() the autoplay policy refused', async () => {
        const player = usePlayer()
        const el = activeAudio()
        el.playResult = () => Promise.reject(new DOMException('needs a tap', 'NotAllowedError'))
        player.playAlbum([song('tr-1', 200)])
        await settle(30_000)

        expect(checkSession).not.toHaveBeenCalled()
        expect(el.playCalls).toBe(1)
    })

    // The pause a browser fires as a track ends is not the user's: the next
    // track must still be watched.
    it('keeps watching across a track change', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200), song('tr-2', 200)])
        const el = activeAudio()
        el.ended = true
        el.fire('pause')
        el.fire('ended')
        el.ended = false
        expect(player.currentTrack.value?.id).toBe('tr-2')

        await settle(15_000)
        expect(checkSession).toHaveBeenCalledTimes(1)
    })
})

describe('usePlayer next-track warm-up', () => {
    const audioAnswer = () => ({
        ok: true,
        status: 206,
        headers: new Headers({ 'content-type': 'audio/flac' }),
        arrayBuffer: () => Promise.resolve(new ArrayBuffer(0))
    })

    it('asks for the first bytes of the next track near the end of this one', async () => {
        httpFetch.mockResolvedValue(audioAnswer())
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200), song('tr-2', 200)])
        const el = activeAudio()
        el.duration = 200

        el.progress(100)
        expect(httpFetch).not.toHaveBeenCalled()

        el.progress(175)
        el.progress(176)
        await settle()
        expect(httpFetch).toHaveBeenCalledTimes(1)
        const [url, init] = httpFetch.mock.calls[0]!
        expect(url).toContain('/stream/tr-2')
        expect(init.headers.Range).toBe('bytes=0-65535')
        expect(checkSession).not.toHaveBeenCalled()
    })

    it('checks the session when the warm-up is refused, so the next URL gets a good key', async () => {
        httpFetch.mockResolvedValue({
            ok: true,
            status: 200,
            headers: new Headers({ 'content-type': 'application/json' }),
            arrayBuffer: () => Promise.resolve(new ArrayBuffer(0))
        })
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200), song('tr-2', 200)])
        const el = activeAudio()
        el.duration = 200

        el.progress(180)
        await settle()
        expect(checkSession).toHaveBeenCalledTimes(1)
    })

    it('raises the banner when the warm-up finds no server', async () => {
        httpFetch.mockRejectedValue(new TypeError('Failed to fetch'))
        checkSession.mockResolvedValue('unreachable')
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200), song('tr-2', 200)])
        const el = activeAudio()
        el.duration = 200

        el.progress(180)
        await settle()
        expect(reportNetworkError).toHaveBeenCalledTimes(1)
    })

    it('does nothing at the end of the queue', async () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200)])
        const el = activeAudio()
        el.duration = 200

        el.progress(190)
        await settle()
        expect(httpFetch).not.toHaveBeenCalled()
    })
})
