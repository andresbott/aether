import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { ref, nextTick } from 'vue'

const play = vi.fn()
const pause = vi.fn()
const playNext = vi.fn()
const playPrevious = vi.fn()
const seek = vi.fn()
const currentTrack = ref<Record<string, unknown> | null>(null)
const isPlaying = ref(false)
const duration = ref(0)

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        currentTrack,
        isPlaying,
        currentTime: ref(12),
        duration,
        play,
        pause,
        playNext,
        playPrevious,
        seek
    })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (id: string, size: number) => `/art/${id}?size=${size}`
    }
}))

type Handler = ((details: { seekTime?: number }) => void) | null

function installMediaSession() {
    const handlers = new Map<string, Handler>()
    const session = {
        metadata: null as unknown,
        playbackState: 'none',
        setActionHandler: vi.fn((action: string, handler: Handler) => {
            handlers.set(action, handler)
        }),
        setPositionState: vi.fn()
    }
    Object.defineProperty(window.navigator, 'mediaSession', {
        configurable: true,
        value: session
    })
    class FakeMediaMetadata {
        constructor(public init: Record<string, unknown>) {
            Object.assign(this, init)
        }
    }
    vi.stubGlobal('MediaMetadata', FakeMediaMetadata)
    return { session, handlers }
}

async function bind() {
    const mod = await import('../useMediaSession')
    mod.resetMediaSessionForTests()
    mod.useMediaSession()
}

beforeEach(() => {
    vi.clearAllMocks()
    currentTrack.value = null
    isPlaying.value = false
    duration.value = 0
})

afterEach(() => {
    vi.unstubAllGlobals()
    // remove the stubbed session so other suites see jsdom's plain navigator
    delete (window.navigator as { mediaSession?: unknown }).mediaSession
})

describe('useMediaSession', () => {
    it('does nothing (and does not throw) without mediaSession support', async () => {
        await expect(bind()).resolves.toBeUndefined()
    })

    it('sets metadata with artwork on track change', async () => {
        const { session } = installMediaSession()
        await bind()
        currentTrack.value = { title: 'T', artist: 'A', album: 'L', coverArt: 'c1' }
        await nextTick()
        const meta = session.metadata as { title: string; artwork: Array<{ src: string; sizes: string }> }
        expect(meta.title).toBe('T')
        expect(meta.artwork.map((a) => a.sizes)).toEqual(['96x96', '256x256', '512x512'])
        expect(meta.artwork[2].src).toBe('/art/c1?size=512')
    })

    it('clears metadata when the track goes away', async () => {
        const { session } = installMediaSession()
        currentTrack.value = { title: 'T', artist: 'A' }
        await bind()
        currentTrack.value = null
        await nextTick()
        expect(session.metadata).toBeNull()
    })

    it('mirrors isPlaying into playbackState', async () => {
        const { session } = installMediaSession()
        await bind()
        isPlaying.value = true
        await nextTick()
        expect(session.playbackState).toBe('playing')
        isPlaying.value = false
        await nextTick()
        expect(session.playbackState).toBe('paused')
    })

    it('wires the transport handlers to the player', async () => {
        const { handlers } = installMediaSession()
        await bind()
        handlers.get('play')?.({})
        handlers.get('pause')?.({})
        handlers.get('nexttrack')?.({})
        handlers.get('previoustrack')?.({})
        handlers.get('seekto')?.({ seekTime: 42 })
        expect(play).toHaveBeenCalledOnce()
        expect(pause).toHaveBeenCalledOnce()
        expect(playNext).toHaveBeenCalledOnce()
        expect(playPrevious).toHaveBeenCalledOnce()
        expect(seek).toHaveBeenCalledWith(42)
    })

    it('publishes position state when duration becomes known', async () => {
        const { session } = installMediaSession()
        await bind()
        duration.value = 180
        await nextTick()
        expect(session.setPositionState).toHaveBeenCalledWith(
            expect.objectContaining({ duration: 180, position: 12 })
        )
    })

    it('is idempotent — a second call registers nothing twice', async () => {
        const { session } = installMediaSession()
        await bind()
        const mod = await import('../useMediaSession')
        mod.useMediaSession()
        const playRegistrations = session.setActionHandler.mock.calls.filter(
            (c) => c[0] === 'play'
        )
        expect(playRegistrations).toHaveLength(1)
    })
})
