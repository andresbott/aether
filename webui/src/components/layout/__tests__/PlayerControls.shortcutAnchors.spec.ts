import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { computed, ref } from 'vue'
import PrimeVue from 'primevue/config'
import PlayerControls from '@/components/layout/PlayerControls.vue'
import { SHORTCUTS, type ShortcutAnchor } from '@/utils/shortcuts'

const volume = ref(1)

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        currentTrack: ref({ id: 's1', title: 'Neon Divide', artist: 'Aurora Skies' }),
        isPlaying: ref(false),
        currentTime: ref(0),
        duration: ref(100),
        volume,
        isMuted: computed(() => volume.value === 0),
        toggleMute: vi.fn(),
        shuffle: ref(false),
        repeat: ref('none'),
        hasNext: ref(true),
        hasPrevious: ref(true),
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
    useToggleStar: () => ({ mutate: vi.fn() })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => true, getCoverArtUrl: () => 'http://cover/1' }
}))

const mountBar = () =>
    mount(PlayerControls, {
        attachTo: document.body,
        global: { plugins: [PrimeVue], directives: { tooltip: {} } }
    })

// The help overlay pins each badge by querying `[data-shortcut="<anchor>"]`, so a
// renamed class or a moved button silently loses its badge unless the anchors are
// asserted against the registry itself.
// Side-placed badges are the sidebar's nav entries, not the player bar, so they
// are excluded here and asserted by AppSidebar.shortcutAnchor.spec.ts. Derived
// from the registry rather than listed by name: the two specs then partition
// every anchor between them, so a newly added one cannot be missed by both.
const BAR_ANCHORS = SHORTCUTS.filter((s) => s.anchor && s.place !== 'side').map(
    (s) => s.anchor as ShortcutAnchor
)

describe('PlayerControls shortcut anchors', () => {
    it('carries a data-shortcut anchor for every anchored registry entry', () => {
        const w = mountBar()
        for (const anchor of BAR_ANCHORS) {
            expect(
                w.find(`[data-shortcut="${anchor}"]`).exists(),
                `missing anchor: ${anchor}`
            ).toBe(true)
        }
    })

    it('puts each anchor on the control that performs the action', () => {
        const w = mountBar()
        expect(w.find('[data-shortcut="play-pause"]').classes()).toContain('play-btn')
        expect(w.find('[data-shortcut="mute"]').classes()).toContain('volume-toggle')
        expect(w.find('[data-shortcut="volume"]').classes()).toContain('volume-slider')
        expect(w.find('[data-shortcut="queue"]').classes()).toContain('queue-toggle')
        expect(w.find('[data-shortcut="favorite"]').classes()).toContain('now-like')
    })

    // The seek badge has to land on the progress bar itself, not on the row that
    // also holds the two time labels — that would centre it over the wrong span.
    it('anchors seek to the progress rail', () => {
        const w = mountBar()
        expect(w.find('[data-shortcut="progress"]').classes()).toContain('progress-slider')
    })

    it('anchors next and previous to their own buttons, not the same one', () => {
        const w = mountBar()
        const next = w.find('[data-shortcut="next"]')
        const previous = w.find('[data-shortcut="previous"]')
        expect(next.find('.pi-step-forward').exists()).toBe(true)
        expect(previous.find('.pi-step-backward').exists()).toBe(true)
    })
})
