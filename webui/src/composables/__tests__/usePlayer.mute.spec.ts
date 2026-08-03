import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'

/** jsdom has no media playback; stand in a minimal recorder (see usePlayer.preload.spec). */
class FakeAudio {
    static instances: FakeAudio[] = []
    src = ''
    preload = ''
    volume = 1
    currentTime = 0
    duration = 0
    paused = true

    constructor() {
        FakeAudio.instances.push(this)
    }
    addEventListener(): void {}
    removeEventListener(): void {}
    load(): void {}
    play(): Promise<void> {
        return Promise.resolve()
    }
    pause(): void {}
}

type UsePlayer = (typeof import('@/composables/usePlayer'))['usePlayer']
let usePlayer: UsePlayer

const loadModule = async () => {
    vi.resetModules()
    ;({ usePlayer } = await import('@/composables/usePlayer'))
}

beforeEach(async () => {
    FakeAudio.instances = []
    vi.stubGlobal('Audio', FakeAudio)
    localStorage.clear()
    await loadModule()
})

afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
})

describe('usePlayer mute', () => {
    it('starts unmuted', () => {
        const player = usePlayer()
        expect(player.isMuted.value).toBe(false)
    })

    it('silences the player and reports muted', () => {
        const player = usePlayer()
        player.setVolume(0.6)

        player.toggleMute()

        expect(player.volume.value).toBe(0)
        expect(player.isMuted.value).toBe(true)
    })

    it('restores the pre-mute volume on the second toggle', () => {
        const player = usePlayer()
        player.setVolume(0.6)

        player.toggleMute()
        player.toggleMute()

        expect(player.volume.value).toBe(0.6)
        expect(player.isMuted.value).toBe(false)
    })

    it('restores the last non-zero volume after the rail was dragged to zero', () => {
        const player = usePlayer()
        player.setVolume(0.4)
        // Dragging the rail all the way down is a mute the user did by hand; the
        // speaker must still know where to come back to.
        player.setVolume(0)
        expect(player.isMuted.value).toBe(true)

        player.toggleMute()

        expect(player.volume.value).toBe(0.4)
    })

    it('comes back to full volume when there is no non-zero volume to restore', () => {
        const player = usePlayer()
        player.setVolume(0)

        player.toggleMute()

        expect(player.volume.value).toBe(1)
    })

    it('remembers the volume to restore across a reload that starts muted', async () => {
        const first = usePlayer()
        first.setVolume(0.3)
        first.toggleMute()
        expect(first.volume.value).toBe(0)

        await loadModule()
        const second = usePlayer()
        expect(second.isMuted.value).toBe(true)

        second.toggleMute()

        expect(second.volume.value).toBe(0.3)
    })
})
