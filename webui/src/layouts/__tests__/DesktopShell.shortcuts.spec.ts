import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { reactive, ref } from 'vue'

const togglePlayPause = vi.fn()

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        currentTrack: ref(null),
        isPlaying: ref(false),
        volume: ref(1),
        currentTime: ref(0),
        duration: ref(0),
        isMuted: ref(false),
        togglePlayPause,
        playNext: vi.fn(),
        playPrevious: vi.fn(),
        seek: vi.fn(),
        setVolume: vi.fn(),
        toggleMute: vi.fn()
    })
}))

vi.mock('@/composables/useQueueSync', () => ({
    useQueueSync: () => ({
        restore: vi.fn().mockResolvedValue(undefined),
        start: vi.fn(),
        stop: vi.fn()
    })
}))

vi.mock('@/composables/useCurrentTrackFavorite', () => ({
    useCurrentTrackFavorite: () => ({ isStarred: ref(false), toggleFavorite: vi.fn() })
}))

vi.mock('vue-router', () => ({
    useRoute: () => reactive({ name: 'library', meta: { flush: true } }),
    useRouter: () => ({ push: vi.fn() }),
    RouterView: { template: '<div class="router-outlet" />' },
    RouterLink: { template: '<a><slot /></a>' }
}))

vi.mock('@/store/uiStore', () => ({
    useUiStore: () => reactive({ queueSidebarCollapsed: ref(false) })
}))

vi.mock('@/composables/useScrollbarWidth', () => ({
    useScrollbarWidth: () => ref(0)
}))

import DesktopShell from '@/layouts/DesktopShell.vue'
import { useShortcutHelp } from '@/composables/useShortcutHelp'

const mountLayout = () =>
    mount(DesktopShell, {
        global: {
            directives: { tooltip: {} },
            stubs: {
                AppSidebar: true,
                PlayerControls: true,
                QueueSidebar: true
            }
        }
    })

beforeEach(() => {
    vi.clearAllMocks()
    useShortcutHelp().close()
})

// The bindings live in the desktop shell, not globally: the settings layout and
// mobile shell are separate shells and deliberately get no player shortcuts.
describe('DesktopShell keyboard shortcuts', () => {
    it('binds the player shortcuts', () => {
        mountLayout()
        document.dispatchEvent(new KeyboardEvent('keydown', { key: ' ' }))
        expect(togglePlayPause).toHaveBeenCalledTimes(1)
    })

    it('renders the help overlay so ? can bring it up', () => {
        const w = mountLayout()
        expect(w.findComponent({ name: 'ShortcutHelpOverlay' }).exists()).toBe(true)
    })

    it('unbinds them when the layout goes away', () => {
        mountLayout().unmount()
        document.dispatchEvent(new KeyboardEvent('keydown', { key: ' ' }))
        expect(togglePlayPause).not.toHaveBeenCalled()
    })
})
