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
    // No ContentScaffold here (the one main-content view without one): a fixed
    // header over both panels showed the queue's heading above the player face
    // and put its hamburger where the URL bar sits. The heading belongs to the
    // queue panel and rides in with it.
    it('renders no scaffold header — the heading lives in the queue panel', () => {
        const w = mountView()
        expect(w.find('.content-scaffold-header').exists()).toBe(false)
        expect(w.find('.scaffold-nav-btn').exists()).toBe(false)
        const heading = w.find('.play-queue .queue-heading')
        expect(heading.find('h2').text()).toBe('Queue')
        expect(heading.find('.queue-heading-summary').text()).toBe('3 tracks • 3 min')
    })

    // The face's drag-down replaces the hamburger, so it needs the same
    // non-gesture path every other view has: a chevron to /browse. What its
    // click does is covered with the gesture it stands in for (below).
    it('the player face carries a nav hint in the hamburger\'s place', () => {
        const w = mountView()
        const hint = w.find('button.play-nav-hint')
        expect(hint.exists()).toBe(true)
        expect(hint.attributes('aria-label')).toBe('Open navigation')
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
        const actions = w.find('.queue-heading-actions')
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
    // it phones would have no way to save or clear the queue. The trio
    // collapses behind the heading's ⋮ overflow (a Popover teleported to
    // body), labeled so the popover reads as a menu.
    const openOverflow = async (w: ReturnType<typeof mountView>) => {
        await w.find('.queue-heading-actions .queue-overflow-btn').trigger('click')
    }
    // The Popover teleports to document.body and mounted wrappers accumulate
    // across tests, so always take the LAST match — that is this test's panel.
    const overflowAction = (selector: string): HTMLElement => {
        const all = document.body.querySelectorAll<HTMLElement>(selector)
        const el = all[all.length - 1]
        expect(el).toBeTruthy()
        return el
    }

    it('edit, save and clear collapse behind the heading ⋮ menu', async () => {
        const w = mountView()
        route.hash = '#queue'
        await w.vm.$nextTick()
        // Not inline in the heading…
        expect(w.find('.queue-heading-actions .queue-action-save').exists()).toBe(false)
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
    // queue): the queue LIST's touch handler owns the switch (the list, not the
    // whole panel, so the heading's own drag below stays a separate gesture). A
    // downward pull that starts with the list at its top returns to the player
    // face — through the hash, so every path runs through one place.
    describe('swipe back down to the player face', () => {
        const swipe = async (w: ReturnType<typeof mountView>, from: number, to: number) => {
            const panel = w.find('.play-queue-list')
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
            const panel = w.find('.play-queue-list')
            await panel.trigger('touchstart', { touches: [{ clientY: 100 }] })
            await panel.trigger('touchmove', { touches: [{ clientY: 180 }] })
            await panel.trigger('touchmove', { touches: [{ clientY: 260 }] })
            expect(replace).toHaveBeenCalledOnce()
        })
    })

    // The list gesture above only arms with the list at its top, so reading
    // down a long queue used to mean scrolling all the way back up before the
    // way out worked. Dragging the heading is the way out at ANY list
    // position — and since the header is not a scroller, either direction
    // counts rather than the user having to guess which way the stack runs.
    describe('drag the queue heading to leave the queue', () => {
        const dragHeader = async (
            w: ReturnType<typeof mountView>,
            from: number,
            to: number,
            target = '.queue-heading'
        ) => {
            const el = w.find(target)
            await el.trigger('touchstart', { touches: [{ clientY: from }] })
            await el.trigger('touchmove', { touches: [{ clientY: to }] })
        }

        /** Mounted showing the queue, its list scrolled well away from the top. */
        const mountOnScrolledQueue = async () => {
            const w = mountView()
            route.hash = '#queue'
            await w.vm.$nextTick()
            w.find('.stub-queue-body').element.scrollTop = 300
            replace.mockClear()
            return w
        }

        it('returns to the player face with the list scrolled down', async () => {
            const w = await mountOnScrolledQueue()
            await dragHeader(w, 100, 180)
            expect(replace).toHaveBeenCalledWith({ hash: '' })
        })

        it('takes an upward drag too — the header is not a scroller', async () => {
            const w = await mountOnScrolledQueue()
            await dragHeader(w, 180, 100)
            expect(replace).toHaveBeenCalledWith({ hash: '' })
        })

        // A tap on shuffle / repeat / ⋮ must stay a tap.
        it('ignores a drag that does not clear the threshold', async () => {
            const w = await mountOnScrolledQueue()
            await dragHeader(w, 100, 130)
            expect(replace).not.toHaveBeenCalled()
        })

        // The heading sits INSIDE the queue panel, above the list: a drag on it
        // must not also arm the list's own pull-from-top handler, or one gesture
        // would fire the switch twice.
        it('fires once per gesture', async () => {
            const w = await mountOnScrolledQueue()
            const header = w.find('.queue-heading')
            await header.trigger('touchstart', { touches: [{ clientY: 100 }] })
            await header.trigger('touchmove', { touches: [{ clientY: 180 }] })
            await header.trigger('touchmove', { touches: [{ clientY: 260 }] })
            expect(replace).toHaveBeenCalledOnce()
        })

        // While the face is up the heading is scrolled off-screen with its
        // panel, so there is nothing to go back to.
        it('does nothing over the player face', async () => {
            const w = mountView()
            await dragHeader(w, 100, 180)
            expect(replace).not.toHaveBeenCalled()
        })
    })

    // The face's own gesture, replacing the hamburger the scaffold header used
    // to carry: dragging the face down leaves for /browse. It has to feel like
    // its mirror image, the native-scroll swipe up to the queue — so the view
    // FOLLOWS the finger and only the release decides, rather than a threshold
    // firing mid-gesture and jumping.
    describe('drag the player face down to leave for /browse', () => {
        // The transform is bound at all times, so "not moved" reads as an
        // explicit zero rather than an absent style.
        const AT_REST = 'translateY(0px)'

        /** Mounted with a real height, so the commit distance is the screen's. */
        const mountWithHeight = (height = 800) => {
            const w = mountView()
            Object.defineProperty(w.find('.mobile-play-view').element, 'offsetHeight', {
                value: height,
                configurable: true
            })
            return w
        }
        const transform = (w: ReturnType<typeof mountView>): string | undefined =>
            (w.find('.mobile-play-view').element as HTMLElement).style.transform
        const drag = async (w: ReturnType<typeof mountView>, from: number, to: number) => {
            const face = w.find('.play-face')
            await face.trigger('touchstart', { touches: [{ clientY: from }] })
            await face.trigger('touchmove', { touches: [{ clientY: to }] })
        }
        const release = async (w: ReturnType<typeof mountView>) => {
            await w.find('.play-face').trigger('touchend')
        }

        it('follows the finger 1:1, with the transition off while it does', async () => {
            const w = mountWithHeight()
            await drag(w, 100, 190)
            expect(transform(w)).toBe('translateY(90px)')
            expect(w.find('.mobile-play-view').classes()).toContain('is-dragging')
            // Still moving: nothing has been decided yet.
            expect(push).not.toHaveBeenCalled()
        })

        it('does not follow an upward drag — that one reveals the queue', async () => {
            const w = mountWithHeight()
            await drag(w, 200, 100)
            expect(transform(w)).toBe(AT_REST)
            expect(w.find('.mobile-play-view').classes()).not.toContain('is-dragging')
        })

        // Pulled back up past the start: the view returns to rest but the finger
        // still owns the motion, so the transition stays off.
        it('clamps at rest when the finger comes back up', async () => {
            const w = mountWithHeight()
            const face = w.find('.play-face')
            await face.trigger('touchstart', { touches: [{ clientY: 100 }] })
            await face.trigger('touchmove', { touches: [{ clientY: 190 }] })
            await face.trigger('touchmove', { touches: [{ clientY: 40 }] })
            expect(transform(w)).toBe('translateY(0px)')
            expect(w.find('.mobile-play-view').classes()).toContain('is-dragging')
        })

        it('springs back and stays when released short of the commit distance', async () => {
            const w = mountWithHeight()
            await drag(w, 100, 220) // 120px, under a fifth of 800
            await release(w)
            expect(transform(w)).toBe('translateY(0px)')
            expect(w.find('.mobile-play-view').classes()).not.toContain('is-dragging')
            expect(push).not.toHaveBeenCalled()
        })

        // Past the commit distance the motion the finger started carries
        // through: the view slides the rest of the way out, and only THEN does
        // the route change.
        it('slides the rest of the way out on release, then navigates', async () => {
            const w = mountWithHeight()
            await drag(w, 100, 320) // 220px, past a fifth of 800
            await release(w)
            expect(w.find('.mobile-play-view').classes()).toContain('is-leaving')
            expect(transform(w)).toBe('translateY(800px)')
            expect(push).not.toHaveBeenCalled()

            await w
                .find('.mobile-play-view')
                .trigger('transitionend', { propertyName: 'transform' })
            expect(push).toHaveBeenCalledWith({ name: 'browse' })
        })

        it('scales the commit distance with the screen', async () => {
            const w = mountWithHeight(300) // a fifth is 60px, under the floor
            await drag(w, 100, 170) // 70px clears the 64px floor
            await release(w)
            expect(w.find('.mobile-play-view').classes()).toContain('is-leaving')
        })

        // A transition that never runs never ends (reduced motion switches it
        // off, and a browser may drop it), so the leave cannot depend on the
        // event alone or the view would sit half-off-screen forever.
        it('navigates on a safety timer when no transitionend arrives', async () => {
            vi.useFakeTimers()
            try {
                const w = mountWithHeight()
                await drag(w, 100, 320)
                await release(w)
                expect(push).not.toHaveBeenCalled()
                vi.advanceTimersByTime(400)
                expect(push).toHaveBeenCalledWith({ name: 'browse' })
            } finally {
                vi.useRealTimers()
            }
        })

        it('navigates once, whichever of the two arrives first', async () => {
            vi.useFakeTimers()
            try {
                const w = mountWithHeight()
                await drag(w, 100, 320)
                await release(w)
                await w
                    .find('.mobile-play-view')
                    .trigger('transitionend', { propertyName: 'transform' })
                vi.advanceTimersByTime(400)
                expect(push).toHaveBeenCalledOnce()
            } finally {
                vi.useRealTimers()
            }
        })

        // The chevron is the non-gesture path to the same place, so it plays the
        // same slide-out rather than cutting straight to the route.
        it('the nav hint plays the same slide-out', async () => {
            const w = mountWithHeight()
            await w.find('.play-nav-hint').trigger('click')
            expect(w.find('.mobile-play-view').classes()).toContain('is-leaving')
            expect(push).not.toHaveBeenCalled()
            await w
                .find('.mobile-play-view')
                .trigger('transitionend', { propertyName: 'transform' })
            expect(push).toHaveBeenCalledWith({ name: 'browse' })
        })

        // Dragging the slider a little off-axis must not start pulling the view
        // away mid-seek.
        it('leaves the seek bar alone', async () => {
            const w = mountWithHeight()
            const seekBar = w.find('.play-seek')
            await seekBar.trigger('touchstart', { touches: [{ clientY: 100 }] })
            await seekBar.trigger('touchmove', { touches: [{ clientY: 320 }] })
            expect(transform(w)).toBe(AT_REST)
        })

        // Part-way to the queue a downward drag is the user scrolling back to
        // the face, not asking to leave the view.
        it('does not arm while the panels are scrolled toward the queue', async () => {
            const w = mountWithHeight()
            w.find('.play-panels').element.scrollTop = 40
            await drag(w, 100, 320)
            expect(transform(w)).toBe(AT_REST)
        })

        it('does not arm from the queue panel', async () => {
            const w = mountWithHeight()
            route.hash = '#queue'
            await w.vm.$nextTick()
            await drag(w, 100, 320)
            expect(transform(w)).toBe(AT_REST)
        })
    })

    // `/#queue` is the queue panel's address: the drawer's Queue entry lands
    // there, and the hash follows a manual swipe so the drawer highlight and a
    // reload both mean the visible panel.
    describe('the #queue hash addresses the queue panel', () => {
        it('arriving with #queue lands on the queue panel without animating', () => {
            route.hash = '#queue'
            const scrollTo = vi.fn()
            ;(Element.prototype as unknown as Record<string, unknown>).scrollTo = scrollTo
            try {
                mountView()
                expect(scrollTo).toHaveBeenCalledWith(
                    expect.objectContaining({ behavior: 'auto' })
                )
            } finally {
                delete (Element.prototype as unknown as Record<string, unknown>).scrollTo
            }
        })

        it('a hash change while mounted scrolls the panel container to the panel it names', async () => {
            const w = mountView()
            const panels = w.find('.play-panels')
            Object.defineProperty(panels.element, 'clientHeight', { value: 800 })
            const scrollTo = vi.fn()
            ;(panels.element as unknown as Record<string, unknown>).scrollTo = scrollTo
            route.hash = '#queue'
            await w.vm.$nextTick()
            expect(scrollTo).toHaveBeenCalledWith({ top: 800, behavior: 'smooth' })
            route.hash = ''
            await w.vm.$nextTick()
            expect(scrollTo).toHaveBeenLastCalledWith({ top: 0, behavior: 'smooth' })
        })

        // scrollIntoView reveals its target in every scrollable ANCESTOR and in
        // the visual viewport: on mobile Chrome that offset the visual viewport
        // by the URL bar's height, sliding the whole app under the URL bar with
        // dead space below it — and nothing at document level could scroll it
        // back. The switch must move this container and nothing else.
        it('never reveals a panel with scrollIntoView', async () => {
            const scrollIntoView = vi.fn()
            ;(Element.prototype as unknown as Record<string, unknown>).scrollIntoView =
                scrollIntoView
            try {
                const w = mountView()
                const panels = w.find('.play-panels')
                Object.defineProperty(panels.element, 'clientHeight', { value: 800 })
                route.hash = '#queue'
                await w.vm.$nextTick()
                route.hash = ''
                await w.vm.$nextTick()
                await w.find('.play-swipe-hint').trigger('click')
                expect(scrollIntoView).not.toHaveBeenCalled()
            } finally {
                delete (Element.prototype as unknown as Record<string, unknown>).scrollIntoView
            }
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
