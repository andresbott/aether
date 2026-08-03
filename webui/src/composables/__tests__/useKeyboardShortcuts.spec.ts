import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, ref, computed } from 'vue'
import { mount } from '@vue/test-utils'
import { useKeyboardShortcuts } from '@/composables/useKeyboardShortcuts'
import { useShortcutHelp } from '@/composables/useShortcutHelp'

const currentTrack = ref<Record<string, unknown> | null>({ id: 's1', title: 'T' })
const isPlaying = ref(false)
const volume = ref(0.5)
const currentTime = ref(30)
const duration = ref(100)

const togglePlayPause = vi.fn()
const playNext = vi.fn()
const playPrevious = vi.fn()
const seek = vi.fn()
const setVolume = vi.fn()
const toggleMute = vi.fn()

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        currentTrack,
        isPlaying,
        volume,
        currentTime,
        duration,
        isMuted: computed(() => volume.value === 0),
        togglePlayPause,
        playNext,
        playPrevious,
        seek,
        setVolume,
        toggleMute
    })
}))

const toggleSidebar = vi.fn()
const expandSidebar = vi.fn()
vi.mock('@/composables/useQueueSidebar', () => ({
    useQueueSidebar: () => ({ sidebarCollapsed: ref(true), toggleSidebar, expandSidebar })
}))

const toggleFavorite = vi.fn()
vi.mock('@/composables/useCurrentTrackFavorite', () => ({
    useCurrentTrackFavorite: () => ({ isStarred: computed(() => false), toggleFavorite })
}))

const routerPush = vi.fn()
vi.mock('vue-router', () => ({
    useRouter: () => ({ push: routerPush })
}))

const Host = defineComponent({
    setup() {
        useKeyboardShortcuts()
        return () => null
    }
})

const press = (key: string, init: Partial<KeyboardEventInit> = {}): KeyboardEvent => {
    const event = new KeyboardEvent('keydown', { key, cancelable: true, ...init })
    document.dispatchEvent(event)
    return event
}

// Dispatch from inside a focused text field, which the guard must ignore.
const pressIn = (tag: string, key: string): void => {
    const el = document.createElement(tag)
    document.body.appendChild(el)
    el.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }))
    el.remove()
}

beforeEach(() => {
    vi.clearAllMocks()
    volume.value = 0.5
    currentTime.value = 30
    duration.value = 100
    currentTrack.value = { id: 's1', title: 'T' }
    useShortcutHelp().close()
})

describe('useKeyboardShortcuts playback', () => {
    it('toggles playback on Space', () => {
        mount(Host)
        press(' ')
        expect(togglePlayPause).toHaveBeenCalledTimes(1)
    })

    // `B` for back, leaving `P` to playlists.
    it('steps tracks with N and B', () => {
        mount(Host)
        press('n')
        press('b')
        expect(playNext).toHaveBeenCalledTimes(1)
        expect(playPrevious).toHaveBeenCalledTimes(1)
    })

    it('seeks five seconds either way with the arrow keys', () => {
        mount(Host)
        press('ArrowRight')
        expect(seek).toHaveBeenLastCalledWith(35)
        press('ArrowLeft')
        expect(seek).toHaveBeenLastCalledWith(25)
    })

    it('stops seeking at the ends of the track', () => {
        mount(Host)
        currentTime.value = 2
        press('ArrowLeft')
        expect(seek).toHaveBeenLastCalledWith(0)
        currentTime.value = 98
        press('ArrowRight')
        expect(seek).toHaveBeenLastCalledWith(100)
    })

    it('does not seek when nothing is loaded', () => {
        mount(Host)
        duration.value = 0
        press('ArrowRight')
        expect(seek).not.toHaveBeenCalled()
    })
})

describe('useKeyboardShortcuts volume', () => {
    it('steps the volume by five percent', () => {
        mount(Host)
        press('ArrowUp')
        expect(setVolume).toHaveBeenLastCalledWith(0.55)
        press('ArrowDown')
        expect(setVolume).toHaveBeenLastCalledWith(0.45)
    })

    // Floating-point drift would otherwise show a volume of 0.5500000000000001
    // and accumulate over a run of presses.
    it('keeps the stepped volume free of floating-point drift', () => {
        mount(Host)
        volume.value = 0.3
        press('ArrowUp')
        expect(setVolume).toHaveBeenLastCalledWith(0.35)
    })

    it('stops at silence and at full volume', () => {
        mount(Host)
        volume.value = 0.02
        press('ArrowDown')
        expect(setVolume).toHaveBeenLastCalledWith(0)
        volume.value = 0.98
        press('ArrowUp')
        expect(setVolume).toHaveBeenLastCalledWith(1)
    })

    // Volume owns the vertical arrows now, so the page can no longer be scrolled
    // with them — but PageUp/PageDown/Home/End are deliberately unbound and still
    // scroll, which is the fallback that makes taking ↑/↓ acceptable.
    it.each(['PageUp', 'PageDown', 'Home', 'End'])('leaves %s to the browser', (key) => {
        mount(Host)
        const event = press(key)
        expect(event.defaultPrevented).toBe(false)
        expect(setVolume).not.toHaveBeenCalled()
    })

    it('toggles mute on M', () => {
        mount(Host)
        press('m')
        expect(toggleMute).toHaveBeenCalledTimes(1)
    })
})

describe('useKeyboardShortcuts app actions', () => {
    it('favorites the current track on L', () => {
        mount(Host)
        press('l')
        expect(toggleFavorite).toHaveBeenCalledTimes(1)
    })

    it('toggles the queue sidebar on Q', () => {
        mount(Host)
        press('q')
        expect(toggleSidebar).toHaveBeenCalledTimes(1)
    })

    it('goes to the search view on S', () => {
        mount(Host)
        press('s')
        expect(routerPush).toHaveBeenCalledWith({ name: 'search' })
    })

    // `/` is unbound now, so Firefox's quick-find is reachable again.
    it('leaves / to the browser', () => {
        mount(Host)
        const event = press('/')
        expect(event.defaultPrevented).toBe(false)
        expect(routerPush).not.toHaveBeenCalled()
    })

    // Now Playing is the `home` route — HomeView renders QueueView at `/`.
    it('opens Now Playing on C', () => {
        mount(Host)
        press('c')
        expect(routerPush).toHaveBeenCalledWith({ name: 'home' })
    })

    // The library root, with no folderId: that is the cross-collection entry
    // which opens on the Discover feed. A folder-scoped push would land on a
    // single library instead, which is not what the sidebar's Library entry does.
    it('opens the library on D', () => {
        mount(Host)
        press('d')
        expect(routerPush).toHaveBeenCalledWith({ name: 'library' })
    })

    it.each([
        ['r', 'radio'],
        ['g', 'genres'],
        ['p', 'playlists']
    ])('navigates on %s', (key, route) => {
        mount(Host)
        press(key)
        expect(routerPush).toHaveBeenCalledWith({ name: route })
    })

    it('opens the help overlay on ? and closes it on Escape', () => {
        const help = useShortcutHelp()
        mount(Host)
        press('?')
        expect(help.open.value).toBe(true)
        press('Escape')
        expect(help.open.value).toBe(false)
    })

    // The overlay is a toggle so the same key puts it away again, which is what
    // a second `?` press means to anyone who opened it by accident.
    it('closes the help overlay when ? is pressed again', () => {
        const help = useShortcutHelp()
        mount(Host)
        press('?')
        press('?')
        expect(help.open.value).toBe(false)
    })

    it('leaves Escape alone when the overlay is not up, so dialogs keep it', () => {
        mount(Host)
        const event = press('Escape')
        expect(event.defaultPrevented).toBe(false)
    })
})

describe('useKeyboardShortcuts guards', () => {
    it.each(['input', 'textarea', 'select'])('ignores keys typed into a %s', (tag) => {
        mount(Host)
        pressIn(tag, ' ')
        pressIn(tag, 'n')
        expect(togglePlayPause).not.toHaveBeenCalled()
        expect(playNext).not.toHaveBeenCalled()
    })

    it('ignores a modified press so browser shortcuts keep working', () => {
        mount(Host)
        press('n', { ctrlKey: true })
        press(' ', { metaKey: true })
        expect(playNext).not.toHaveBeenCalled()
        expect(togglePlayPause).not.toHaveBeenCalled()
    })

    // Space scrolls the page and the arrows scroll it, so a handled key must not
    // also reach the browser.
    it('swallows the keys it handles', () => {
        mount(Host)
        expect(press(' ').defaultPrevented).toBe(true)
        expect(press('s').defaultPrevented).toBe(true)
        expect(press('ArrowRight').defaultPrevented).toBe(true)
        expect(press('ArrowUp').defaultPrevented).toBe(true)
    })

    // The rails are focusable role=slider elements that handle their own arrows.
    // Now that volume owns ↑/↓ as well as seek owning ←/→, a press on a focused
    // handle must still reach PrimeVue or keyboard seeking breaks.
    it('leaves the arrows to a focused slider handle', () => {
        mount(Host)
        const handle = document.createElement('span')
        handle.setAttribute('role', 'slider')
        document.body.appendChild(handle)
        for (const key of ['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight']) {
            handle.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true }))
        }
        handle.remove()
        expect(setVolume).not.toHaveBeenCalled()
        expect(seek).not.toHaveBeenCalled()
    })

    it('lets unhandled keys through untouched', () => {
        mount(Host)
        expect(press('z').defaultPrevented).toBe(false)
    })

    it('stops listening once the component is gone', () => {
        const w = mount(Host)
        w.unmount()
        press(' ')
        expect(togglePlayPause).not.toHaveBeenCalled()
    })
})
