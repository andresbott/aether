import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick, ref } from 'vue'

vi.mock('@/components/layout/QueueView.vue', () => ({
    default: {
        name: 'QueueView',
        props: ['variant'],
        template: '<div class="stub-queue-view">{{ variant }}</div>'
    }
}))

vi.mock('@/components/layout/MobilePlayView.vue', () => ({
    default: { name: 'MobilePlayView', template: '<div class="stub-play-view"></div>' }
}))

const replace = vi.fn()
vi.mock('vue-router', () => ({
    useRouter: () => ({ replace })
}))

const shell = ref<'desktop' | 'mobile'>('desktop')
vi.mock('@/composables/useViewport', () => ({
    useViewport: () => ({ shell })
}))

const queue = ref<Array<{ id: string }>>([])
vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ queue })
}))

import HomeView from '@/views/HomeView.vue'

beforeEach(() => {
    replace.mockClear()
    shell.value = 'desktop'
    queue.value = [{ id: '1' }]
})

describe('HomeView', () => {
    it('desktop renders QueueView in the full variant', () => {
        const w = mount(HomeView)
        expect(w.find('.stub-queue-view').text()).toBe('full')
        expect(w.find('.stub-play-view').exists()).toBe(false)
    })

    // Desktop keeps Now Playing on `/` even with nothing queued — the queue
    // list's empty state is the desktop answer, not a redirect.
    it('desktop stays on Now Playing when the queue is empty', () => {
        queue.value = []
        const w = mount(HomeView)
        expect(w.find('.stub-queue-view').exists()).toBe(true)
        expect(replace).not.toHaveBeenCalled()
    })

    it('mobile renders the play view when the queue has tracks', () => {
        shell.value = 'mobile'
        const w = mount(HomeView)
        expect(w.find('.stub-play-view').exists()).toBe(true)
        expect(w.find('.stub-queue-view').exists()).toBe(false)
        expect(replace).not.toHaveBeenCalled()
    })

    // The phone mimics the desktop flow: an empty queue means there is
    // nothing to play, so `/` lands on the browse page — the phone's nav
    // surface — instead.
    it('mobile with an empty queue replaces the route with the browse page', () => {
        shell.value = 'mobile'
        queue.value = []
        const w = mount(HomeView)
        expect(replace).toHaveBeenCalledWith({ name: 'browse' })
        // Nothing renders in the gap while the redirect is in flight.
        expect(w.find('.stub-play-view').exists()).toBe(false)
        expect(w.find('.stub-queue-view').exists()).toBe(false)
    })

    it('redirects when the queue empties while the play view is on screen', async () => {
        shell.value = 'mobile'
        mount(HomeView)
        expect(replace).not.toHaveBeenCalled()
        queue.value = []
        await nextTick()
        expect(replace).toHaveBeenCalledWith({ name: 'browse' })
    })
})
