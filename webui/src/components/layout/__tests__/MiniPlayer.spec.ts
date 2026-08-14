import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import MiniPlayer from '../MiniPlayer.vue'

const openSheet = vi.fn()
vi.mock('@/composables/usePlayerSheet', () => ({
    usePlayerSheet: () => ({ open: openSheet, close: vi.fn(), isOpen: ref(false) })
}))

const togglePlayPause = vi.fn()
const playNext = vi.fn()
const isPlaying = ref(false)
const currentTrack = ref<{ title: string; artist: string; coverArt?: string } | null>({
    title: 'Karma Police',
    artist: 'Radiohead',
    coverArt: 'cov-1'
})

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        currentTrack,
        isPlaying,
        currentTime: ref(30),
        duration: ref(120),
        togglePlayPause,
        playNext
    })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (id: string, size: number) => `/art/${id}?size=${size}`
    }
}))

beforeEach(() => {
    openSheet.mockClear()
    togglePlayPause.mockClear()
    playNext.mockClear()
    isPlaying.value = false
})

describe('MiniPlayer', () => {
    it('shows title, artist and cover of the current track', () => {
        const mp = mount(MiniPlayer)
        expect(mp.text()).toContain('Karma Police')
        expect(mp.text()).toContain('Radiohead')
        expect(mp.find('img.mini-cover').attributes('src')).toBe('/art/cov-1?size=96')
    })

    it('play button toggles playback without opening the sheet', async () => {
        const mp = mount(MiniPlayer)
        await mp.find('[aria-label="Play"]').trigger('click')
        expect(togglePlayPause).toHaveBeenCalledOnce()
        expect(openSheet).not.toHaveBeenCalled()
    })

    it('shows Pause while playing', () => {
        isPlaying.value = true
        expect(mount(MiniPlayer).find('[aria-label="Pause"]').exists()).toBe(true)
    })

    it('next button skips without opening the sheet', async () => {
        const mp = mount(MiniPlayer)
        await mp.find('[aria-label="Next track"]').trigger('click')
        expect(playNext).toHaveBeenCalledOnce()
        expect(openSheet).not.toHaveBeenCalled()
    })

    it('tapping the bar opens the player sheet, not a route', async () => {
        const mp = mount(MiniPlayer)
        await mp.find('[aria-label="Open player"]').trigger('click')
        expect(openSheet).toHaveBeenCalledOnce()
    })

    // The open target is a SIBLING under the transport, not a role="button"
    // wrapper around it: nested, Enter/Space on Pause bubbled into the wrapper
    // and opened the sheet (Space never pausing at all under .prevent).
    it('keyboard-activating the transport never opens the sheet', async () => {
        const mp = mount(MiniPlayer)
        const pause = mp.find('[aria-label="Play"]')
        await pause.trigger('keydown.enter')
        await pause.trigger('keydown.space')
        expect(openSheet).not.toHaveBeenCalled()
        expect(mp.find('.mini-player').attributes('role')).toBeUndefined()
    })

    it('renders the progress hairline from currentTime/duration', () => {
        const mp = mount(MiniPlayer)
        expect(mp.find('.mini-progress-fill').attributes('style')).toContain('width: 25%')
    })
})
