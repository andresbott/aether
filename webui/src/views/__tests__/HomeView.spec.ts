import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'

vi.mock('@/components/layout/QueueView.vue', () => ({
    default: {
        name: 'QueueView',
        props: ['variant'],
        template: '<div class="stub-queue-view">{{ variant }}</div>'
    }
}))

import HomeView from '@/views/HomeView.vue'

describe('HomeView', () => {
    it('renders QueueView in the full variant', () => {
        const w = mount(HomeView)
        expect(w.find('.stub-queue-view').text()).toBe('full')
    })
})
