import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
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

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        currentTrack,
        isPlaying: ref(false),
        currentTime,
        duration: ref(100),
        volume,
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
