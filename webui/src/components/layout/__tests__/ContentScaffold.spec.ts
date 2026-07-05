import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'

describe('ContentScaffold', () => {
    it('renders title, summary, actions slot and body slot', () => {
        const w = mount(ContentScaffold, {
            props: { title: 'Main', summary: '1,240 albums' },
            slots: { actions: '<button class="act">A</button>', default: '<div class="body">B</div>' }
        })
        expect(w.find('.scaffold-title h1').text()).toBe('Main')
        expect(w.find('.scaffold-summary').text()).toBe('1,240 albums')
        expect(w.find('.scaffold-actions .act').exists()).toBe(true)
        expect(w.find('.content-scaffold-body .body').text()).toBe('B')
    })

    it('omits the summary element when summary is not provided', () => {
        const w = mount(ContentScaffold, { props: { title: 'Main' } })
        expect(w.find('.scaffold-title h1').text()).toBe('Main')
        expect(w.find('.scaffold-summary').exists()).toBe(false)
    })
})
