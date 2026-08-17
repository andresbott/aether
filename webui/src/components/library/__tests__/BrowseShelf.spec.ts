import { describe, it, expect } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
import BrowseShelf from '@/components/library/BrowseShelf.vue'

const items = [{ key: 'a' }, { key: 'b' }, { key: 'c' }]

const mountShelf = (props: Record<string, unknown> = {}) =>
    mount(BrowseShelf, {
        props: { title: 'Playlists', to: { name: 'playlists' }, items, ...props },
        slots: { card: '<span class="card-probe">card</span>' },
        global: { stubs: { RouterLink: RouterLinkStub } }
    })

describe('BrowseShelf', () => {
    it('renders the heading, its icon, and one strip item per item', () => {
        const w = mountShelf({ icon: 'pi pi-list' })
        expect(w.find('.shelf-title').text()).toBe('Playlists')
        expect(w.find('.shelf-title i').classes()).toContain('pi-list')
        expect(w.findAll('.shelf-item')).toHaveLength(3)
        expect(w.findAll('.card-probe')).toHaveLength(3)
    })

    it('links See all to the section it samples, labelled per section', () => {
        const w = mountShelf()
        const link = w.findComponent(RouterLinkStub)
        expect(link.props('to')).toEqual({ name: 'playlists' })
        expect(link.attributes('aria-label')).toBe('See all in Playlists')
    })

    it('shows a spinner and no strip while loading', () => {
        const w = mountShelf({ loading: true, items: [] })
        expect(w.find('.shelf-state .pi-spinner').exists()).toBe(true)
        expect(w.find('.shelf-strip').exists()).toBe(false)
    })

    // A failed request must not read as "this section is empty".
    it('renders the error text, distinct from the empty text', () => {
        const w = mountShelf({
            error: true,
            items: [],
            errorText: 'Could not load playlists',
            emptyText: 'No playlists yet'
        })
        expect(w.find('.shelf-state').text()).toContain('Could not load playlists')
        expect(w.text()).not.toContain('No playlists yet')
    })

    // The heading and its link survive an empty section: "See all" is the way to
    // the view that can fix the emptiness.
    it('keeps the heading and See all link when there is nothing to show', () => {
        const w = mountShelf({ items: [], emptyText: 'No playlists yet' })
        expect(w.find('.shelf-title').text()).toBe('Playlists')
        expect(w.findComponent(RouterLinkStub).exists()).toBe(true)
        expect(w.find('.shelf-state').text()).toBe('No playlists yet')
        expect(w.find('.shelf-strip').exists()).toBe(false)
    })

    it('hands each item to the card slot', () => {
        const w = mount(BrowseShelf, {
            props: { title: 'Genres', to: '/genres', items: [{ key: 'Jazz' }, { key: 'Rock' }] },
            slots: { card: '<span class="card-probe">{{ params.item.key }}</span>' },
            global: { stubs: { RouterLink: RouterLinkStub } }
        })
        expect(w.findAll('.card-probe').map((el) => el.text())).toEqual(['Jazz', 'Rock'])
    })
})
