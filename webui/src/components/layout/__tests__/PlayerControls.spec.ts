import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { computed, ref } from 'vue'
import PrimeVue from 'primevue/config'
import PlayerControls from '@/components/layout/PlayerControls.vue'

const currentTrack = ref<Record<string, unknown> | null>(null)
const mutate = vi.fn()
const volume = ref(1)
const setVolume = vi.fn((v: number) => {
    volume.value = v
})
const currentTime = ref(0)
const seek = vi.fn((t: number) => {
    currentTime.value = t
})
const isMuted = computed(() => volume.value === 0)
const toggleMute = vi.fn()

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        currentTrack,
        isPlaying: ref(false),
        currentTime,
        duration: ref(100),
        volume,
        isMuted,
        toggleMute,
        shuffle: ref(false),
        repeat: ref('none'),
        hasNext: ref(false),
        hasPrevious: ref(false),
        togglePlayPause: vi.fn(),
        playNext: vi.fn(),
        playPrevious: vi.fn(),
        toggleShuffle: vi.fn(),
        toggleRepeat: vi.fn(),
        seek,
        setVolume
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
    mount(PlayerControls, {
        attachTo: document.body,
        global: { plugins: [PrimeVue], directives: { tooltip: {} } }
    })

// jsdom gives every element a zero-size rect, so stub the rail's geometry:
// x=100..300, i.e. clientX 150 is 25% and 250 is 75%.
const stubRailRect = (rail: Element) => {
    rail.getBoundingClientRect = () =>
        ({ left: 100, width: 200, top: 0, height: 5, right: 300, bottom: 5 }) as DOMRect
}

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

describe('PlayerControls mute button', () => {
    beforeEach(() => {
        volume.value = 1
        toggleMute.mockClear()
    })

    it('mutes the player when the speaker is clicked', async () => {
        const w = mountBar()
        await w.find('.volume-toggle').trigger('click')
        expect(toggleMute).toHaveBeenCalledTimes(1)
    })

    it('shows a muted speaker and an unmute label while silent', async () => {
        const w = mountBar()
        const button = w.find('.volume-toggle')
        expect(button.attributes('aria-label')).toBe('Mute')

        volume.value = 0
        await w.vm.$nextTick()
        expect(button.find('.pi-volume-off').exists()).toBe(true)
        expect(button.attributes('aria-label')).toBe('Unmute')
    })

    // The three loudness steps must be told apart at a glance. PrimeIcons has no
    // slashed-speaker glyph, so silence is `pi-volume-off` (a bare cone) plus a
    // `muted` class the stylesheet draws the slash from — without that class the
    // muted state would look like the quiet one.
    it('marks silence with its own icon, distinct from the quiet and loud states', async () => {
        const w = mountBar()
        const icon = () => w.find('.volume-toggle i')

        volume.value = 1
        await w.vm.$nextTick()
        const loud = icon().classes()

        volume.value = 0.2
        await w.vm.$nextTick()
        const quiet = icon().classes()

        volume.value = 0
        await w.vm.$nextTick()
        const silent = icon().classes()

        expect(loud).toContain('pi-volume-up')
        expect(quiet).toContain('pi-volume-down')
        expect(silent).toContain('pi-volume-off')
        // The slash is what separates silence from quiet; the others must not carry it.
        expect(silent).toContain('muted')
        expect(loud).not.toContain('muted')
        expect(quiet).not.toContain('muted')
    })
})

// `.rail-active` is what the stylesheet keys the knob and the accent fill off,
// so these assert the state, not the paint (scoped styles never apply here —
// PlayerControls.railStyles.spec.ts covers what the class actually does).
describe('PlayerControls rail active state', () => {
    beforeEach(() => {
        volume.value = 1
    })

    it.each([
        ['volume', '.volume-slider'],
        ['progress', '.progress-slider']
    ])('marks the %s rail active only while it is hovered', async (_name, selector) => {
        const w = mountBar()
        const wrapper = w.find(selector)
        expect(wrapper.classes()).not.toContain('rail-active')

        await wrapper.trigger('mouseenter')
        expect(wrapper.classes()).toContain('rail-active')

        await wrapper.trigger('mouseleave')
        expect(wrapper.classes()).not.toContain('rail-active')
    })

    it.each([
        ['volume', '.volume-slider'],
        ['progress', '.progress-slider']
    ])('keeps the %s rail active while dragging away from it', async (_name, selector) => {
        const w = mountBar()
        const wrapper = w.find(selector)
        stubRailRect(w.find(`${selector} .p-slider`).element)

        await wrapper.trigger('mouseenter')
        await wrapper.trigger('mousedown', { button: 0, clientX: 150 })
        // Dragging past the bar's edge fires mouseleave, but the grab is still on.
        await wrapper.trigger('mouseleave')
        expect(wrapper.classes()).toContain('rail-active')

        document.dispatchEvent(new MouseEvent('mouseup'))
        await w.vm.$nextTick()
        expect(wrapper.classes()).not.toContain('rail-active')
    })
})

describe('PlayerControls volume rail', () => {
    beforeEach(() => {
        volume.value = 1
        setVolume.mockClear()
    })

    it('updates the volume while the pointer is held down on the rail', async () => {
        const w = mountBar()
        const rail = w.find('.volume-slider .p-slider')
        stubRailRect(rail.element)

        await rail.trigger('mousedown', { button: 0, clientX: 150 })
        expect(setVolume).toHaveBeenLastCalledWith(0.25)

        // Still held: moving the pointer keeps updating without a mouseup.
        document.dispatchEvent(new MouseEvent('mousemove', { clientX: 250 }))
        expect(setVolume).toHaveBeenLastCalledWith(0.75)
        await w.vm.$nextTick()
        expect(w.find('.volume-slider .p-slider-range').attributes('style')).toContain('75%')

        document.dispatchEvent(new MouseEvent('mouseup'))
        // Released: further movement is ignored.
        setVolume.mockClear()
        document.dispatchEvent(new MouseEvent('mousemove', { clientX: 120 }))
        expect(setVolume).not.toHaveBeenCalled()
    })

    it('clamps positions outside the rail to 0 and 100', async () => {
        const w = mountBar()
        const rail = w.find('.volume-slider .p-slider')
        stubRailRect(rail.element)

        await rail.trigger('mousedown', { button: 0, clientX: 150 })
        document.dispatchEvent(new MouseEvent('mousemove', { clientX: 20 }))
        expect(setVolume).toHaveBeenLastCalledWith(0)
        document.dispatchEvent(new MouseEvent('mousemove', { clientX: 900 }))
        expect(setVolume).toHaveBeenLastCalledWith(1)
        document.dispatchEvent(new MouseEvent('mouseup'))
    })

    it('leaves presses that start on the handle to PrimeVue', async () => {
        const w = mountBar()
        stubRailRect(w.find('.volume-slider .p-slider').element)

        await w.find('.volume-slider .p-slider-handle').trigger('mousedown', {
            button: 0,
            clientX: 150
        })
        expect(setVolume).not.toHaveBeenCalled()
        document.dispatchEvent(new MouseEvent('mouseup'))
    })
})

describe('PlayerControls progress rail', () => {
    beforeEach(() => {
        currentTime.value = 0
        seek.mockClear()
    })

    it('seeks while the pointer is held down on the rail', async () => {
        const w = mountBar()
        const rail = w.find('.progress-slider .p-slider')
        stubRailRect(rail.element)

        // duration is mocked at 100s, so a percentage maps 1:1 to seconds.
        await rail.trigger('mousedown', { button: 0, clientX: 150 })
        expect(seek).toHaveBeenLastCalledWith(25)

        // Still held: moving the pointer keeps seeking without a mouseup.
        document.dispatchEvent(new MouseEvent('mousemove', { clientX: 250 }))
        expect(seek).toHaveBeenLastCalledWith(75)
        await w.vm.$nextTick()
        expect(w.find('.progress-slider .p-slider-range').attributes('style')).toContain('75%')
        expect(w.findAll('.time-label')[0].text()).toBe('1:15')

        document.dispatchEvent(new MouseEvent('mouseup'))
        // Released: further movement is ignored.
        seek.mockClear()
        document.dispatchEvent(new MouseEvent('mousemove', { clientX: 120 }))
        expect(seek).not.toHaveBeenCalled()
    })

    it('clamps positions outside the rail to the start and end of the track', async () => {
        const w = mountBar()
        const rail = w.find('.progress-slider .p-slider')
        stubRailRect(rail.element)

        await rail.trigger('mousedown', { button: 0, clientX: 150 })
        document.dispatchEvent(new MouseEvent('mousemove', { clientX: 20 }))
        expect(seek).toHaveBeenLastCalledWith(0)
        document.dispatchEvent(new MouseEvent('mousemove', { clientX: 900 }))
        expect(seek).toHaveBeenLastCalledWith(100)
        document.dispatchEvent(new MouseEvent('mouseup'))
    })

    it('leaves presses that start on the handle to PrimeVue', async () => {
        const w = mountBar()
        stubRailRect(w.find('.progress-slider .p-slider').element)

        await w.find('.progress-slider .p-slider-handle').trigger('mousedown', {
            button: 0,
            clientX: 150
        })
        expect(seek).not.toHaveBeenCalled()
        document.dispatchEvent(new MouseEvent('mouseup'))
    })
})
