import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref, computed, reactive } from 'vue'
import PlayerSheet from '../PlayerSheet.vue'
import { resetPlayerSheetForTests, usePlayerSheet } from '@/composables/usePlayerSheet'

// A reactive route stand-in so the route-change watcher can be exercised: the
// mock router's push() mutates it the way a real navigation would.
const routeRef = reactive({ fullPath: '/', name: 'home' })
const push = vi.fn((to: string | { name?: string }) => {
    routeRef.fullPath = typeof to === 'string' ? to : `/${to.name ?? ''}`
    // A real router push adds a history entry.
    window.history.pushState({ ...window.history.state }, '')
    return Promise.resolve()
})
vi.mock('vue-router', () => ({
    useRouter: () => ({ push }),
    useRoute: () => routeRef
}))

const togglePlayPause = vi.fn()
const playNext = vi.fn()
const playPrevious = vi.fn()
const seek = vi.fn()
const toggleShuffle = vi.fn()
const toggleRepeat = vi.fn()
const isPlaying = ref(true)
const currentTrack = ref<Record<string, unknown> | null>({
    title: 'Karma Police',
    artist: 'Radiohead',
    coverArt: 'cov-1',
    albumId: 'al-9',
    artistId: 'ar-4'
})

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        currentTrack,
        isPlaying,
        currentTime: ref(30),
        duration: ref(120),
        shuffle: ref(false),
        repeat: ref('none'),
        hasNext: computed(() => true),
        hasPrevious: computed(() => false),
        togglePlayPause,
        playNext,
        playPrevious,
        seek,
        toggleShuffle,
        toggleRepeat,
        queue: ref([])
    })
}))

const toggleFavorite = vi.fn()
vi.mock('@/composables/useCurrentTrackFavorite', () => ({
    useCurrentTrackFavorite: () => ({ isStarred: computed(() => false), toggleFavorite })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (id: string, size: number) => `/art/${id}?size=${size}`
    }
}))

// QueueBody pulls the whole queue stack; the sheet only needs to place it.
const QueueBodyStub = { name: 'QueueBody', props: ['variant', 'editMode'], template: '<div data-queue-body />' }

const mountSheet = () =>
    mount(PlayerSheet, {
        global: { stubs: { QueueBody: QueueBodyStub, Teleport: true } }
    })

// Minimal history stack stand-in: only `state` has to stay consistent, since
// that is what carries the sheet's entry marker.
let stack: Array<unknown>

beforeEach(() => {
    vi.clearAllMocks()
    resetPlayerSheetForTests()
    routeRef.fullPath = '/'
    routeRef.name = 'home'
    stack = [{ base: true }]
    vi.spyOn(window.history, 'state', 'get').mockImplementation(() => stack[stack.length - 1])
    vi.spyOn(window.history, 'pushState').mockImplementation((state) => {
        stack.push(state)
    })
    vi.spyOn(window.history, 'replaceState').mockImplementation((state) => {
        stack[stack.length - 1] = state
    })
    vi.spyOn(window.history, 'back').mockImplementation(() => {
        // Synchronously dispatch popstate so callback ordering can be verified
        stack.pop()
        window.dispatchEvent(new PopStateEvent('popstate', { state: stack[stack.length - 1] }))
    })
})

describe('PlayerSheet', () => {
    it('renders nothing while closed', () => {
        expect(mountSheet().find('.player-sheet').exists()).toBe(false)
    })

    it('opens with artwork, meta and transport', async () => {
        const sheet = mountSheet()
        usePlayerSheet().open()
        await sheet.vm.$nextTick()
        expect(sheet.find('.player-sheet').exists()).toBe(true)
        expect(sheet.find('img.sheet-cover').attributes('src')).toBe('/art/cov-1?size=512')
        expect(sheet.text()).toContain('Karma Police')
        expect(sheet.text()).toContain('Radiohead')
        expect(sheet.find('[aria-label="Pause"]').exists()).toBe(true)
        expect(sheet.find('[aria-label="Previous track"]').attributes('disabled')).toBeDefined()
    })

    it('chevron closes the sheet', async () => {
        const sheet = mountSheet()
        usePlayerSheet().open()
        await sheet.vm.$nextTick()
        await sheet.find('[aria-label="Close player"]').trigger('click')
        expect(usePlayerSheet().isOpen.value).toBe(false)
    })

    it('seek input drives player.seek with absolute seconds', async () => {
        const sheet = mountSheet()
        usePlayerSheet().open()
        await sheet.vm.$nextTick()
        const range = sheet.find('input[type="range"]')
        ;(range.element as HTMLInputElement).value = '90'
        await range.trigger('input')
        expect(seek).toHaveBeenCalledWith(90)
    })

    it('title navigates to the album and closes', async () => {
        const backSpy = vi.spyOn(window.history, 'back')
        const sheet = mountSheet()
        usePlayerSheet().open()
        await sheet.vm.$nextTick()
        await sheet.find('.sheet-title').trigger('click')
        expect(push).toHaveBeenCalledWith({ name: 'album', params: { id: 'al-9' } })
        expect(usePlayerSheet().isOpen.value).toBe(false)
        // Navigation must happen AFTER popstate consumption, not before
        const backCallOrder = backSpy.mock.invocationCallOrder[0]
        const pushCallOrder = push.mock.invocationCallOrder[0]
        expect(pushCallOrder).toBeGreaterThan(backCallOrder)
    })

    it('queue button swaps to the queue face', async () => {
        const sheet = mountSheet()
        usePlayerSheet().open()
        await sheet.vm.$nextTick()
        await sheet.find('[aria-label="Show queue"]').trigger('click')
        expect(sheet.find('[data-queue-body]').exists()).toBe(true)
        expect(sheet.find('.sheet-cover').exists()).toBe(false)
    })

    it('Escape closes the sheet', async () => {
        const sheet = mountSheet()
        usePlayerSheet().open()
        await sheet.vm.$nextTick()
        window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
        await sheet.vm.$nextTick()
        expect(usePlayerSheet().isOpen.value).toBe(false)
    })

    it('swipe down on the header dismisses', async () => {
        const sheet = mountSheet()
        usePlayerSheet().open()
        await sheet.vm.$nextTick()
        const header = sheet.find('.sheet-header')
        await header.trigger('touchstart', { touches: [{ clientY: 100 }] })
        await header.trigger('touchend', { changedTouches: [{ clientY: 220 }] })
        expect(usePlayerSheet().isOpen.value).toBe(false)
    })

    it('a short drag on the header is not a dismissal', async () => {
        const sheet = mountSheet()
        usePlayerSheet().open()
        await sheet.vm.$nextTick()
        const header = sheet.find('.sheet-header')
        await header.trigger('touchstart', { touches: [{ clientY: 100 }] })
        await header.trigger('touchend', { changedTouches: [{ clientY: 140 }] })
        expect(usePlayerSheet().isOpen.value).toBe(true)
    })

    // C1: the route change is itself a navigation with its own history entry, so
    // the sheet must NOT consume an entry when reacting to it — history.back()
    // there pops the route the user just navigated to and bounces them back.
    it('a route change from outside the sheet closes it without eating the navigation', async () => {
        const backSpy = vi.spyOn(window.history, 'back')
        const sheet = mountSheet()
        usePlayerSheet().open()
        await sheet.vm.$nextTick()
        const depth = stack.length
        // A bottom-tab tap: the router navigates, pushing its own entry.
        window.history.pushState({ ...window.history.state }, '')
        routeRef.fullPath = '/library'
        await sheet.vm.$nextTick()
        expect(usePlayerSheet().isOpen.value).toBe(false)
        expect(backSpy).not.toHaveBeenCalled()
        // The new route's entry is still on top: the navigation stuck.
        expect(stack.length).toBe(depth + 1)
        expect((stack[stack.length - 1] as Record<string, unknown>).aetherPlayerSheet).toBeUndefined()
    })

    // C3: shell swap / login gate can unmount the sheet while it is open.
    it('unmounting while open leaves the sheet closed for the next mount', async () => {
        const backSpy = vi.spyOn(window.history, 'back')
        const sheet = mountSheet()
        usePlayerSheet().open()
        await sheet.vm.$nextTick()
        sheet.unmount()
        expect(usePlayerSheet().isOpen.value).toBe(false)
        // A shell swap is not a back navigation: nothing may be consumed.
        expect(backSpy).not.toHaveBeenCalled()
        // Remount (the other shell): closed, and no stale marker left behind.
        const remounted = mountSheet()
        await remounted.vm.$nextTick()
        expect(remounted.find('.player-sheet').exists()).toBe(false)
        expect((stack[stack.length - 1] as Record<string, unknown>).aetherPlayerSheet).toBeUndefined()
        // A later system back is not swallowed by the orphaned entry.
        stack.pop()
        window.dispatchEvent(new PopStateEvent('popstate', { state: stack[stack.length - 1] }))
        expect(backSpy).not.toHaveBeenCalled()
    })

    it('in-sheet navigation still consumes its own entry exactly once', async () => {
        const backSpy = vi.spyOn(window.history, 'back')
        const sheet = mountSheet()
        usePlayerSheet().open()
        await sheet.vm.$nextTick()
        await sheet.find('.sheet-title').trigger('click')
        await sheet.vm.$nextTick()
        expect(push).toHaveBeenCalledTimes(1)
        expect(backSpy).toHaveBeenCalledTimes(1)
    })
})
