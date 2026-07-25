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

    it('shows no back button by default', () => {
        const w = mount(ContentScaffold, { props: { title: 'X' } })
        expect(w.find('.scaffold-back').exists()).toBe(false)
    })

    it('emits back when the back button is clicked', async () => {
        const w = mount(ContentScaffold, { props: { title: 'X', showBack: true } })
        expect(w.find('.scaffold-back').exists()).toBe(true)
        await w.find('.scaffold-back').trigger('click')
        expect(w.emitted('back')).toHaveLength(1)
    })

    it('centers the header on the shared content column', () => {
        const w = mount(ContentScaffold, { props: { title: 'Library' } })
        const inner = w.find('.content-scaffold-header .scaffold-header-inner')
        expect(inner.exists()).toBe(true)
        expect(inner.classes()).toContain('content-col')
        expect(inner.find('h1').text()).toBe('Library')
    })
})
