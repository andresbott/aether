import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import MiniPlayer from '../MiniPlayer.vue'

const push = vi.fn()
vi.mock('vue-router', () => ({
    useRouter: () => ({ push })
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
    push.mockClear()
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

    it('play button toggles playback without navigating', async () => {
        const mp = mount(MiniPlayer)
        await mp.find('[aria-label="Play"]').trigger('click')
        expect(togglePlayPause).toHaveBeenCalledOnce()
        expect(push).not.toHaveBeenCalled()
    })

    it('shows Pause while playing', () => {
        isPlaying.value = true
        expect(mount(MiniPlayer).find('[aria-label="Pause"]').exists()).toBe(true)
    })

    it('next button skips without navigating', async () => {
        const mp = mount(MiniPlayer)
        await mp.find('[aria-label="Next track"]').trigger('click')
        expect(playNext).toHaveBeenCalledOnce()
        expect(push).not.toHaveBeenCalled()
    })

    it('tapping the bar navigates to the Now Playing route', async () => {
        const mp = mount(MiniPlayer)
        await mp.find('[aria-label="Open Now Playing"]').trigger('click')
        expect(push).toHaveBeenCalledWith({ name: 'home' })
    })

    // The open target is a SIBLING under the transport, not a role="button"
    // wrapper around it: nested, Enter/Space on Pause bubbled into the wrapper
    // and navigated (Space never pausing at all under .prevent).
    it('keyboard-activating the transport never navigates', async () => {
        const mp = mount(MiniPlayer)
        const pause = mp.find('[aria-label="Play"]')
        await pause.trigger('keydown.enter')
        await pause.trigger('keydown.space')
        expect(push).not.toHaveBeenCalled()
        expect(mp.find('.mini-player').attributes('role')).toBeUndefined()
    })

    it('renders the progress hairline from currentTime/duration', () => {
        const mp = mount(MiniPlayer)
        expect(mp.find('.mini-progress-fill').attributes('style')).toContain('width: 25%')
    })

    // The bar is the play screen's handle: lifting it opens Now Playing, the
    // counterpart of MobilePlayView's drag down to /browse. It has to feel the
    // same — the bar follows the finger and only the release decides — so the
    // gesture is a drag, not a threshold that fires mid-pull.
    describe('lift the bar to open Now Playing', () => {
        // `String(-0)` is "0", so a bar at rest reads without the sign.
        const AT_REST = 'translateY(0px)'

        const mountBar = (height = 56) => {
            const mp = mount(MiniPlayer)
            Object.defineProperty(mp.find('.mini-player').element, 'offsetHeight', {
                value: height,
                configurable: true
            })
            return mp
        }
        const transform = (mp: ReturnType<typeof mountBar>): string =>
            (mp.find('.mini-player').element as HTMLElement).style.transform
        const lift = async (mp: ReturnType<typeof mountBar>, from: number, to: number) => {
            const el = mp.find('.mini-player')
            await el.trigger('touchstart', { touches: [{ clientY: from }] })
            await el.trigger('touchmove', { touches: [{ clientY: to }] })
        }
        const release = async (mp: ReturnType<typeof mountBar>) => {
            await mp.find('.mini-player').trigger('touchend')
        }

        it('follows the finger upward, with the transition off while it does', async () => {
            const mp = mountBar()
            await lift(mp, 500, 460)
            expect(transform(mp)).toBe('translateY(-40px)')
            expect(mp.find('.mini-player').classes()).toContain('is-dragging')
            expect(push).not.toHaveBeenCalled()
        })

        // The vacated strip is painted by a ::after sized from this var, so no
        // page background flashes under the bar mid-lift.
        it('reports the lift to the CSS that keeps the vacated strip painted', async () => {
            const mp = mountBar()
            await lift(mp, 500, 460)
            expect(mp.find('.mini-player').attributes('style')).toContain('--mini-lift: 40px')
        })

        it('does not follow a downward drag — the bar only goes one way', async () => {
            const mp = mountBar()
            await lift(mp, 500, 560)
            expect(transform(mp)).toBe(AT_REST)
            expect(mp.find('.mini-player').classes()).not.toContain('is-dragging')
        })

        // Past the commit distance the pull is already going to open; 1:1 to the
        // top would fling a 3.5rem bar into the middle of the page.
        it('rubber-bands the travel past the commit distance', async () => {
            const mp = mountBar()
            await lift(mp, 500, 400) // 100px: 64 + 36 * 0.4
            expect(transform(mp)).toBe('translateY(-78.4px)')
        })

        it('settles back and stays when released short of the commit distance', async () => {
            const mp = mountBar()
            await lift(mp, 500, 460)
            await release(mp)
            expect(transform(mp)).toBe(AT_REST)
            expect(mp.find('.mini-player').classes()).not.toContain('is-dragging')
            expect(push).not.toHaveBeenCalled()
        })

        it('lifts clear of its own strip on release, then navigates', async () => {
            const mp = mountBar()
            await lift(mp, 500, 420) // 80px, past the 64px commit
            await release(mp)
            expect(mp.find('.mini-player').classes()).toContain('is-leaving')
            expect(transform(mp)).toBe('translateY(-80px)') // bar height + clearance
            expect(push).not.toHaveBeenCalled()

            await mp.find('.mini-player').trigger('transitionend', { propertyName: 'transform' })
            expect(push).toHaveBeenCalledWith({ name: 'home' })
        })

        it('navigates on a safety timer when no transitionend arrives', async () => {
            vi.useFakeTimers()
            try {
                const mp = mountBar()
                await lift(mp, 500, 420)
                await release(mp)
                expect(push).not.toHaveBeenCalled()
                vi.advanceTimersByTime(400)
                expect(push).toHaveBeenCalledWith({ name: 'home' })
            } finally {
                vi.useRealTimers()
            }
        })

        it('navigates once, whichever of the two arrives first', async () => {
            vi.useFakeTimers()
            try {
                const mp = mountBar()
                await lift(mp, 500, 420)
                await release(mp)
                await mp
                    .find('.mini-player')
                    .trigger('transitionend', { propertyName: 'transform' })
                vi.advanceTimersByTime(400)
                expect(push).toHaveBeenCalledOnce()
            } finally {
                vi.useRealTimers()
            }
        })

        // A drag and a tap are the same touch until the finger moves. The click
        // the browser may still deliver on release is not a tap any more:
        // honouring it would navigate on a pull the user deliberately cancelled…
        it('a cancelled pull does not open through the click that follows it', async () => {
            const mp = mountBar()
            await lift(mp, 500, 470)
            await release(mp)
            await mp.find('[aria-label="Open Now Playing"]').trigger('click')
            expect(push).not.toHaveBeenCalled()
        })

        // …or skip a track the user was only dragging from.
        it('a pull that starts on the transport does not trigger it', async () => {
            const mp = mountBar()
            const next = mp.find('[aria-label="Next track"]')
            await mp.find('.mini-player').trigger('touchstart', { touches: [{ clientY: 500 }] })
            await mp.find('.mini-player').trigger('touchmove', { touches: [{ clientY: 470 }] })
            await release(mp)
            await next.trigger('click')
            expect(playNext).not.toHaveBeenCalled()
        })

        // Only the moved gesture is swallowed — a plain tap still opens, and it
        // opens IMMEDIATELY: it has no motion to finish.
        it('a tap with no movement still opens right away', async () => {
            const mp = mountBar()
            const el = mp.find('.mini-player')
            await el.trigger('touchstart', { touches: [{ clientY: 500 }] })
            await el.trigger('touchend')
            await mp.find('[aria-label="Open Now Playing"]').trigger('click')
            expect(push).toHaveBeenCalledWith({ name: 'home' })
            expect(mp.find('.mini-player').classes()).not.toContain('is-leaving')
        })
    })
})
