import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'

describe('ContentScaffold', () => {
    it('renders the title and summary', () => {
        const w = mount(ContentScaffold, { props: { title: 'Playlists', summary: '3 playlists' } })
        expect(w.find('.scaffold-title h1').text()).toBe('Playlists')
        expect(w.find('.scaffold-summary').text()).toBe('3 playlists')
    })

    it('renders a #title-actions slot beside the title', () => {
        const w = mount(ContentScaffold, {
            props: { title: 'My Mix' },
            slots: { 'title-actions': '<button class="rename-probe">edit</button>' }
        })
        const title = w.find('.scaffold-title')
        expect(title.find('.rename-probe').exists()).toBe(true)
    })

    it('renders the #actions slot', () => {
        const w = mount(ContentScaffold, {
            props: { title: 'X' },
            slots: { actions: '<button class="act-probe">go</button>' }
        })
        expect(w.find('.scaffold-actions .act-probe').exists()).toBe(true)
    })
})
