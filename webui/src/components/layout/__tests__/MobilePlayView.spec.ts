import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { reactive, ref } from 'vue'
import PrimeVue from 'primevue/config'

const push = vi.fn()
const replace = vi.fn()
const route = reactive({ name: 'home', path: '/', fullPath: '/', hash: '' })
vi.mock('vue-router', () => ({
    useRouter: () => ({ push, replace }),
    useRoute: () => route
}))

// ContentScaffold decides the hamburger from the viewport singleton.
vi.mock('@/composables/useViewport', () => ({
    useViewport: () => ({ shell: ref('mobile'), tier: ref('phone'), isTouch: ref(true) })
}))

const song = (id: string, extra: Record<string, unknown> = {}) => ({
    id,
    title: `Song ${id}`,
    artist: 'Artist',
    album: 'Album',
    duration: 60,
    ...extra
})

const queue = ref<Array<Record<string, unknown>>>([])
const currentTrack = ref<Record<string, unknown> | null>(null)
const isPlaying = ref(false)
const shuffle = ref(false)
const repeat = ref<'none' | 'all' | 'one'>('none')
const seek = vi.fn()
const togglePlayPause = vi.fn()
const playNext = vi.fn()
const playPrevious = vi.fn()
const toggleShuffle = vi.fn()
const toggleRepeat = vi.fn()

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        queue,
        currentTrack,
        isPlaying,
        shuffle,
        repeat,
        currentTime: ref(30),
        duration: ref(120),
        hasNext: ref(true),
        hasPrevious: ref(true),
        seek,
        togglePlayPause,
        playNext,
        playPrevious,
        toggleShuffle,
        toggleRepeat
    })
}))

const toggleFavorite = vi.fn()
const isStarred = ref(false)
vi.mock('@/composables/useCurrentTrackFavorite', () => ({
    useCurrentTrackFavorite: () => ({ isStarred, toggleFavorite })
}))

const openSaveDialog = vi.fn()
const clearQueue = vi.fn()
vi.mock('@/composables/useQueueActions', () => ({
    useQueueActions: () => ({
        showSaveDialog: ref(false),
        playlistName: ref(''),
        openSaveDialog,
        handleSave: vi.fn(),
        isSaving: ref(false),
        clearQueue
    })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (id: string, size: number) => `/art/${id}?size=${size}`
    }
}))

vi.mock('@/components/layout/QueueBody.vue', () => ({
    default: {
        name: 'QueueBody',
        props: ['variant', 'editMode'],
        template: '<div class="stub-queue-body">{{ variant }}</div>'
    }
}))

vi.mock('@/components/layout/SavePlaylistDialog.vue', () => ({
    default: { name: 'SavePlaylistDialog', template: '<div class="stub-save-dialog"></div>' }
}))

import MobilePlayView from '@/components/layout/MobilePlayView.vue'

const mountView = () =>
    mount(MobilePlayView, { global: { plugins: [PrimeVue], directives: { tooltip: {} } } })

beforeEach(() => {
    queue.value = [
        song('1'),
        song('2', { albumId: 'al2', artistId: 'ar2', coverArt: 'cov-2' }),
        song('3')
    ]
    currentTrack.value = queue.value[1]
    isPlaying.value = false
    shuffle.value = false
    repeat.value = 'none'
    isStarred.value = false
    route.hash = ''
    vi.clearAllMocks()
})

describe('MobilePlayView', () => {
    // The queue heading is rendered in BOTH panel states so the header height
    // cannot change with the swap; the `queue-up` class is what reveals it
    // (the fade itself is CSS, guarded by the layoutStyles spec).
    it('always renders the queue heading, unrevealed over the player face', () => {
        const w = mountView()
        expect(w.find('h1').text()).toBe('Queue')
        expect(w.find('.scaffold-summary').text()).toBe('3 tracks • 3 min')
        expect(w.find('.mobile-play-view').classes()).not.toContain('queue-up')
        // First-class screen: the drawer trigger, not a dismiss chevron.
        expect(w.find('[aria-label="Open navigation"]').exists()).toBe(true)
    })

    it('reveals the queue heading while the queue panel is the visible one', async () => {
        const w = mountView()
        route.hash = '#queue'
        await w.vm.$nextTick()
        expect(w.find('.mobile-play-view').classes()).toContain('queue-up')
        route.hash = ''
        await w.vm.$nextTick()
        expect(w.find('.mobile-play-view').classes()).not.toContain('queue-up')
    })

    it('shows track, artist and cover art on the player face', () => {
        const w = mountView()
        expect(w.find('.play-title').text()).toBe('Song 2')
        expect(w.find('.play-artist').text()).toBe('Artist')
        expect(w.find('img.play-cover').attributes('src')).toBe('/art/cov-2?size=512')
    })

    it('wires the transport to the player', async () => {
        const w = mountView()
        await w.find('[aria-label="Play"]').trigger('click')
        expect(togglePlayPause).toHaveBeenCalledOnce()
        await w.find('[aria-label="Next track"]').trigger('click')
        expect(playNext).toHaveBeenCalledOnce()
        await w.find('[aria-label="Previous track"]').trigger('click')
        expect(playPrevious).toHaveBeenCalledOnce()
    })

    // Shuffle and repeat are queue behaviour, so they live in the queue
    // header, not the face's transport row (which keeps prev/play/next only).
    it('shuffle and repeat live in the queue header, wired to the player', async () => {
        const w = mountView()
        expect(w.find('.play-transport [aria-label="Shuffle"]').exists()).toBe(false)
        expect(w.find('.play-transport [aria-label="Repeat"]').exists()).toBe(false)
        const actions = w.find('.scaffold-actions')
        await actions.find('[aria-label="Shuffle"]').trigger('click')
        expect(toggleShuffle).toHaveBeenCalledOnce()
        await actions.find('[aria-label="Repeat"]').trigger('click')
        expect(toggleRepeat).toHaveBeenCalledOnce()
    })

    it('shuffle and repeat read their pressed state from the player', () => {
        shuffle.value = true
        repeat.value = 'all'
        const w = mountView()
        const shuffleBtn = w.find('.queue-action-shuffle')
        const repeatBtn = w.find('.queue-action-repeat')
        expect(shuffleBtn.attributes('aria-pressed')).toBe('true')
        expect(shuffleBtn.classes()).toContain('is-active')
        expect(repeatBtn.attributes('aria-pressed')).toBe('true')
        expect(repeatBtn.classes()).toContain('is-active')
    })

    it('seeking through the range input calls seek', async () => {
        const w = mountView()
        const range = w.find('input[type="range"]')
        await range.setValue('45')
        expect(seek).toHaveBeenCalledWith(45)
    })

    it('title and artist navigate to the album and artist routes', async () => {
        const w = mountView()
        await w.find('.play-title').trigger('click')
        expect(push).toHaveBeenCalledWith({ name: 'album', params: { id: 'al2' } })
        await w.find('.play-artist').trigger('click')
        expect(push).toHaveBeenCalledWith({ name: 'artist', params: { id: 'ar2' } })
    })

    it('disables the title/artist links when the track has no ids', () => {
        currentTrack.value = queue.value[0] // no albumId/artistId
        const w = mountView()
        expect(w.find('.play-title').attributes('disabled')).toBeDefined()
        expect(w.find('.play-artist').attributes('disabled')).toBeDefined()
    })

    it('double-tapping the cover flips the favorite; a single tap does not', async () => {
        const w = mountView()
        const art = w.find('.play-art')
        await art.trigger('click')
        expect(toggleFavorite).not.toHaveBeenCalled()
        await art.trigger('click')
        expect(toggleFavorite).toHaveBeenCalledOnce()
    })

    it('shows the heart indicator on the cover only while the track is starred', async () => {
        const w = mountView()
        expect(w.find('.play-favorite-indicator').exists()).toBe(false)
        isStarred.value = true
        await w.vm.$nextTick()
        // On the cover's corner, not floating on the screen.
        const heart = w.find('.play-art .play-favorite-indicator')
        expect(heart.classes()).toContain('pi-heart-fill')
        expect(heart.attributes('aria-hidden')).toBe('true')
    })

    // The queue is revealed by swiping the player face up, not by a header
    // toggle: both panels render together inside a vertical snap scroller
    // (the snap CSS itself is guarded by the layoutStyles spec).
    it('stacks the play face and the queue panel together, with no header toggle', () => {
        const w = mountView()
        expect(w.find('.play-panels .play-face').exists()).toBe(true)
        expect(w.find('.play-panels .play-queue .stub-queue-body').text()).toBe('sidebar')
        expect(w.find('.play-queue-toggle').exists()).toBe(false)
    })

    // The queue keeps the queue management the desktop header has — without
    // it phones would have no way to save or clear the queue. On phone tier
    // the trio collapses behind the scaffold's ⋮ overflow (a Popover
    // teleported to body), labeled so the popover reads as a menu.
    const openOverflow = async (w: ReturnType<typeof mountView>) => {
        await w.find('.scaffold-actions .scaffold-overflow-btn').trigger('click')
    }
    // The Popover teleports to document.body and mounted wrappers accumulate
    // across tests, so always take the LAST match — that is this test's panel.
    const overflowAction = (selector: string): HTMLElement => {
        const all = document.body.querySelectorAll<HTMLElement>(selector)
        const el = all[all.length - 1]
        expect(el).toBeTruthy()
        return el
    }

    it('edit, save and clear collapse behind the header ⋮ menu', async () => {
        const w = mountView()
        route.hash = '#queue'
        await w.vm.$nextTick()
        // Not inline in the header…
        expect(w.find('.scaffold-actions .queue-action-save').exists()).toBe(false)
        // …but reachable, labeled, through the overflow popover.
        await openOverflow(w)
        const save = overflowAction('.queue-action-save')
        expect(save.textContent).toContain('Save as playlist')
        save.click()
        expect(openSaveDialog).toHaveBeenCalledOnce()
        overflowAction('.queue-action-clear').click()
        expect(clearQueue).toHaveBeenCalledOnce()
    })

    it('the pencil in the ⋮ menu toggles edit mode on the queue body', async () => {
        const w = mountView()
        route.hash = '#queue'
        await w.vm.$nextTick()
        const body = w.findComponent({ name: 'QueueBody' })
        expect(body.props('editMode')).toBe(false)
        await openOverflow(w)
        overflowAction('.queue-action-edit').click()
        await w.vm.$nextTick()
        expect(body.props('editMode')).toBe(true)
    })

    // Edit mode is queue-panel UI: leaving the queue for the player face ends
    // the editing session, so returning to the queue never lands on a stale
    // selection.
    it('scrolling back to the player face exits queue edit mode', async () => {
        const w = mountView()
        route.hash = '#queue'
        await w.vm.$nextTick()
        await openOverflow(w)
        overflowAction('.queue-action-edit').click()
        await w.vm.$nextTick()
        const body = w.findComponent({ name: 'QueueBody' })
        expect(body.props('editMode')).toBe(true)

        // Back to the face via the hash (the hint button and swipe-back both
        // route through it); the swipe path flips currentPanel the same way.
        route.hash = ''
        await w.vm.$nextTick()
        expect(body.props('editMode')).toBe(false)
    })

    // The swipe is the primary gesture, but the hint chevron is a real button
    // so pointer and AT users get a non-gesture path to the same place. It
    // goes through the hash so the header reveal starts with the request (the
    // hash watcher owns the scroll — covered below).
    it('the swipe hint addresses the queue panel through the hash', async () => {
        const w = mountView()
        const hint = w.find('button.play-swipe-hint')
        expect(hint.attributes('aria-label')).toBe('Show queue')
        await hint.trigger('click')
        expect(replace).toHaveBeenCalledWith({ hash: '#queue' })
    })

    // The way BACK is not native scroll chaining (a chained drag hands the
    // mandatory-snap container no momentum, so it settles right back on the
    // queue): the queue panel's touch handler owns the switch. A downward
    // pull that starts with the list at its top returns to the player face —
    // through the hash, so the header reveal follows the same one path.
    describe('swipe back down to the player face', () => {
        const swipe = async (w: ReturnType<typeof mountView>, from: number, to: number) => {
            const panel = w.find('.play-queue')
            await panel.trigger('touchstart', { touches: [{ clientY: from }] })
            await panel.trigger('touchmove', { touches: [{ clientY: to }] })
        }

        it('a downward pull from the queue top addresses the face panel', async () => {
            const w = mountView()
            await swipe(w, 100, 180)
            expect(replace).toHaveBeenCalledWith({ hash: '' })
        })

        it('ignores a pull that does not clear the threshold', async () => {
            const w = mountView()
            await swipe(w, 100, 130)
            expect(replace).not.toHaveBeenCalled()
        })

        it('does not fire while the queue list is scrolled down — that pull scrolls the list', async () => {
            const w = mountView()
            const list = w.find('.stub-queue-body')
            list.element.scrollTop = 50
            await list.trigger('touchstart', { touches: [{ clientY: 100 }] })
            await list.trigger('touchmove', { touches: [{ clientY: 200 }] })
            expect(replace).not.toHaveBeenCalled()
        })

        it('fires once per gesture', async () => {
            const w = mountView()
            const panel = w.find('.play-queue')
            await panel.trigger('touchstart', { touches: [{ clientY: 100 }] })
            await panel.trigger('touchmove', { touches: [{ clientY: 180 }] })
            await panel.trigger('touchmove', { touches: [{ clientY: 260 }] })
            expect(replace).toHaveBeenCalledOnce()
        })
    })

    // `/#queue` is the queue panel's address: the drawer's Queue entry lands
    // there, and the hash follows a manual swipe so the drawer highlight and a
    // reload both mean the visible panel.
    describe('the #queue hash addresses the queue panel', () => {
        it('arriving with #queue lands on the queue panel without animating', () => {
            route.hash = '#queue'
            const scrollIntoView = vi.fn()
            ;(Element.prototype as unknown as Record<string, unknown>).scrollIntoView =
                scrollIntoView
            try {
                mountView()
                expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'auto', block: 'start' })
            } finally {
                delete (Element.prototype as unknown as Record<string, unknown>).scrollIntoView
            }
        })

        it('a hash change while mounted scrolls to the panel it names', async () => {
            const w = mountView()
            const toQueue = vi.fn()
            const toFace = vi.fn()
            w.find('.play-queue').element.scrollIntoView = toQueue
            w.find('.play-face').element.scrollIntoView = toFace
            route.hash = '#queue'
            await w.vm.$nextTick()
            expect(toQueue).toHaveBeenCalledWith({ behavior: 'smooth', block: 'start' })
            route.hash = ''
            await w.vm.$nextTick()
            expect(toFace).toHaveBeenCalledWith({ behavior: 'smooth', block: 'start' })
        })

        // Immediately at the midpoint crossing, not after the snap settles:
        // the header fade must run during the gesture, not pop in after it.
        it('a swipe crossing the midpoint rewrites the hash right away, and back', async () => {
            const w = mountView()
            const panels = w.find('.play-panels')
            Object.defineProperty(panels.element, 'clientHeight', { value: 800 })
            panels.element.scrollTop = 401
            await panels.trigger('scroll')
            expect(replace).toHaveBeenCalledWith({ hash: '#queue' })

            panels.element.scrollTop = 399
            await panels.trigger('scroll')
            expect(replace).toHaveBeenCalledWith({ hash: '' })
        })

        // A hash-initiated smooth scroll passes the same positions a swipe
        // does; the handler must not read them as a user swipe and flip the
        // panel straight back mid-flight.
        it('a programmatic scroll to the queue is not mistaken for a swipe back', async () => {
            const w = mountView()
            const panels = w.find('.play-panels')
            Object.defineProperty(panels.element, 'clientHeight', { value: 800 })
            route.hash = '#queue'
            await w.vm.$nextTick()
            replace.mockClear()

            // The smooth scroll is still on the face's side of the midpoint.
            panels.element.scrollTop = 100
            await panels.trigger('scroll')
            expect(replace).not.toHaveBeenCalled()
            expect(w.find('.mobile-play-view').classes()).toContain('queue-up')

            // Crossing to the target ends the exemption; a real swipe back
            // down afterwards flips the panel again.
            panels.element.scrollTop = 800
            await panels.trigger('scroll')
            panels.element.scrollTop = 0
            await panels.trigger('scroll')
            expect(replace).toHaveBeenCalledWith({ hash: '' })
        })

        // A finger taking over mid-flight IS a user swipe again: the touch
        // cancels the exemption so the panel follows the gesture.
        it('a touch on the panels cancels the programmatic-scroll exemption', async () => {
            const w = mountView()
            const panels = w.find('.play-panels')
            Object.defineProperty(panels.element, 'clientHeight', { value: 800 })
            route.hash = '#queue'
            await w.vm.$nextTick()
            replace.mockClear()

            await panels.trigger('touchstart', { touches: [{ clientY: 100 }] })
            panels.element.scrollTop = 100
            await panels.trigger('scroll')
            expect(replace).toHaveBeenCalledWith({ hash: '' })
        })
    })
})
