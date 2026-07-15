import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'
import PlayerControls from '@/components/layout/PlayerControls.vue'

const currentTrack = ref<Record<string, unknown> | null>(null)
const mutate = vi.fn()

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        currentTrack,
        isPlaying: ref(false),
        currentTime: ref(0),
        duration: ref(100),
        volume: ref(1),
        shuffle: ref(false),
        repeat: ref('none'),
        hasNext: ref(false),
        hasPrevious: ref(false),
        togglePlayPause: vi.fn(),
        playNext: vi.fn(),
        playPrevious: vi.fn(),
        toggleShuffle: vi.fn(),
        toggleRepeat: vi.fn(),
        seek: vi.fn(),
        setVolume: vi.fn()
    })
}))

vi.mock('@/composables/useQueueSidebar', () => ({
    useQueueSidebar: () => ({ sidebarCollapsed: ref(true), toggleSidebar: vi.fn() })
}))

vi.mock('@/composables/useSubsonicQueries', () => ({
    useToggleStar: () => ({ mutate })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => true, getCoverArtUrl: () => 'http://cover/1' }
}))

const mountBar = () =>
    mount(PlayerControls, { global: { plugins: [PrimeVue], directives: { tooltip: {} } } })

describe('PlayerControls now-playing', () => {
    it('shows no now-playing block when nothing is playing', () => {
        currentTrack.value = null
        const w = mountBar()
        expect(w.find('.now-like').exists()).toBe(false)
    })

    it('renders cover, title, artist and a heart button for the current track', () => {
        currentTrack.value = { id: 's1', title: 'Neon Divide', artist: 'Aurora Skies' }
        const w = mountBar()
        expect(w.find('.now-title').text()).toBe('Neon Divide')
        expect(w.find('.now-artist').text()).toBe('Aurora Skies')
        const like = w.find('.now-like')
        expect(like.exists()).toBe(true)
        expect(like.find('.pi-heart').exists()).toBe(true)
    })

    it('toggles favorite and fills the heart when liked', async () => {
        currentTrack.value = { id: 's1', title: 'Neon Divide', artist: 'Aurora Skies' }
        const w = mountBar()
        await w.find('.now-like').trigger('click')
        expect(mutate).toHaveBeenCalledWith({ id: 's1', starred: false })
        await w.vm.$nextTick()
        expect(w.find('.now-like .pi-heart-fill').exists()).toBe(true)
    })
})
