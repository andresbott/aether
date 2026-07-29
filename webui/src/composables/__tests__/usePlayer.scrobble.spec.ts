import { describe, it, expect, vi, beforeEach } from 'vitest'

const scrobbleMock = vi.fn(() => Promise.resolve())
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getStreamUrl: (id: string) => `https://example.test/stream/${id}`,
        scrobble: scrobbleMock
    }
}))

// Minimal Audio stub: records listeners so the test can fire timeupdate, and
// lets the test drive currentTime/duration directly.
class FakeAudio {
    src = ''
    volume = 1
    currentTime = 0
    duration = 0
    preload = ''
    listeners: Record<string, Array<() => void>> = {}
    addEventListener(type: string, cb: () => void) {
        ;(this.listeners[type] ||= []).push(cb)
    }
    removeEventListener() {}
    load() {}
    play() {
        return Promise.resolve()
    }
    pause() {}
    fire(type: string) {
        for (const cb of this.listeners[type] ?? []) cb()
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

let usePlayer: typeof import('@/composables/usePlayer')['usePlayer']

beforeEach(async () => {
    localStorage.clear()
    created.length = 0
    scrobbleMock.mockClear()
    vi.resetModules()
    usePlayer = (await import('@/composables/usePlayer')).usePlayer
})

// The active element is the first one constructed by initAudioElements().
const activeAudio = () => created[0]

describe('usePlayer scrobbling', () => {
    it('scrobbles once past 50% of the duration', () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1', 200)])

        const el = activeAudio()
        el.duration = 200
        el.currentTime = 99
        el.fire('timeupdate')
        expect(scrobbleMock).not.toHaveBeenCalled()

        el.currentTime = 101
        el.fire('timeupdate')
        expect(scrobbleMock).toHaveBeenCalledTimes(1)
        expect(scrobbleMock).toHaveBeenCalledWith('tr-1')

        // Further timeupdates on the same track must not re-submit.
        el.currentTime = 150
        el.fire('timeupdate')
        el.currentTime = 190
        el.fire('timeupdate')
        expect(scrobbleMock).toHaveBeenCalledTimes(1)
    })

    it('scrobbles at 4 minutes for a long track without waiting for 50%', () => {
        const player = usePlayer()
        player.playAlbum([song('tr-long', 1800)])

        const el = activeAudio()
        el.duration = 1800
        el.currentTime = 239
        el.fire('timeupdate')
        expect(scrobbleMock).not.toHaveBeenCalled()

        el.currentTime = 241
        el.fire('timeupdate')
        expect(scrobbleMock).toHaveBeenCalledTimes(1)
    })

    it('does not scrobble tracks shorter than 30 seconds', () => {
        const player = usePlayer()
        player.playAlbum([song('tr-short', 20)])

        const el = activeAudio()
        el.duration = 20
        el.currentTime = 19
        el.fire('timeupdate')
        expect(scrobbleMock).not.toHaveBeenCalled()
    })

    it('re-arms for the next track so each track scrobbles once', () => {
        const player = usePlayer()
        player.playAlbum([song('tr-1', 100), song('tr-2', 100)])

        const el = activeAudio()
        el.duration = 100
        el.currentTime = 60
        el.fire('timeupdate')
        expect(scrobbleMock).toHaveBeenCalledTimes(1)

        player.playQueueItem(1)
        const next = activeAudio()
        next.duration = 100
        next.currentTime = 60
        next.fire('timeupdate')
        expect(scrobbleMock).toHaveBeenCalledTimes(2)
        expect(scrobbleMock).toHaveBeenLastCalledWith('tr-2')
    })
})
