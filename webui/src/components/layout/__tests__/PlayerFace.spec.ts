import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'

const push = vi.fn()
vi.mock('vue-router', () => ({
    useRouter: () => ({ push })
}))

const queue = ref<Array<Record<string, unknown>>>([])
const currentTrack = ref<Record<string, unknown> | null>(null)
const isPlaying = ref(false)
const seek = vi.fn()
const togglePlayPause = vi.fn()
const playNext = vi.fn()
const playPrevious = vi.fn()

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        queue,
        currentTrack,
        isPlaying,
        currentTime: ref(30),
        duration: ref(120),
        hasNext: ref(true),
        hasPrevious: ref(true),
        seek,
        togglePlayPause,
        playNext,
        playPrevious
    })
}))

const toggleFavorite = vi.fn()
const isStarred = ref(false)
vi.mock('@/composables/useCurrentTrackFavorite', () => ({
    useCurrentTrackFavorite: () => ({ isStarred, toggleFavorite })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (id: string, size: number) => `/art/${id}?size=${size}`
    }
}))

import PlayerFace from '@/components/layout/PlayerFace.vue'

beforeEach(() => {
    currentTrack.value = {
        id: '2',
        title: 'Song 2',
        artist: 'Artist',
        albumId: 'al2',
        artistId: 'ar2',
        coverArt: 'cov-2'
    }
    isStarred.value = false
    isPlaying.value = false
    vi.clearAllMocks()
})

describe('PlayerFace', () => {
    it('shows track, artist and cover art', () => {
        const w = mount(PlayerFace)
        expect(w.find('.play-title').text()).toBe('Song 2')
        expect(w.find('.play-artist').text()).toBe('Artist')
        expect(w.find('img.play-cover').attributes('src')).toBe('/art/cov-2?size=512')
    })

    it('wires the prev / play / next transport to the player', async () => {
        const w = mount(PlayerFace)
        await w.find('[aria-label="Play"]').trigger('click')
        expect(togglePlayPause).toHaveBeenCalledOnce()
        await w.find('[aria-label="Next track"]').trigger('click')
        expect(playNext).toHaveBeenCalledOnce()
        await w.find('[aria-label="Previous track"]').trigger('click')
        expect(playPrevious).toHaveBeenCalledOnce()
    })

    it('carries no shuffle/repeat — those are queue behaviour (QueuePanel)', () => {
        const w = mount(PlayerFace)
        expect(w.find('[aria-label="Shuffle"]').exists()).toBe(false)
        expect(w.find('[aria-label="Repeat"]').exists()).toBe(false)
    })

    it('seeking through the range input calls seek', async () => {
        const w = mount(PlayerFace)
        await w.find('input[type="range"]').setValue('45')
        expect(seek).toHaveBeenCalledWith(45)
    })

    it('title and artist navigate to the album and artist routes', async () => {
        const w = mount(PlayerFace)
        await w.find('.play-title').trigger('click')
        expect(push).toHaveBeenCalledWith({ name: 'album', params: { id: 'al2' } })
        await w.find('.play-artist').trigger('click')
        expect(push).toHaveBeenCalledWith({ name: 'artist', params: { id: 'ar2' } })
    })

    it('disables the title/artist links when the track has no ids', () => {
        currentTrack.value = { id: '1', title: 'Song 1', artist: 'Artist' }
        const w = mount(PlayerFace)
        expect(w.find('.play-title').attributes('disabled')).toBeDefined()
        expect(w.find('.play-artist').attributes('disabled')).toBeDefined()
    })

    it('double-tapping the cover flips the favorite; a single tap does not', async () => {
        const w = mount(PlayerFace)
        const art = w.find('.play-art')
        await art.trigger('click')
        expect(toggleFavorite).not.toHaveBeenCalled()
        await art.trigger('click')
        expect(toggleFavorite).toHaveBeenCalledOnce()
    })

    it('shows the heart indicator on the cover only while starred', async () => {
        const w = mount(PlayerFace)
        expect(w.find('.play-favorite-indicator').exists()).toBe(false)
        isStarred.value = true
        await w.vm.$nextTick()
        const heart = w.find('.play-art .play-favorite-indicator')
        expect(heart.classes()).toContain('pi-heart-fill')
        expect(heart.attributes('aria-hidden')).toBe('true')
    })

    // The two swipe affordances are real buttons, so neither destination is
    // gesture-only — but they only EMIT: the sheet owns navigation.
    it('the ⌄ hint emits collapse and the ⌃ hint emits show-queue', async () => {
        const w = mount(PlayerFace)
        await w.find('button.play-nav-hint').trigger('click')
        expect(w.emitted('collapse')).toHaveLength(1)
        await w.find('button.play-swipe-hint').trigger('click')
        expect(w.emitted('show-queue')).toHaveLength(1)
        expect(push).not.toHaveBeenCalled()
    })
})
