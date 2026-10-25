import { describe, it, expect, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
import ListCard from '@/components/library/ListCard.vue'
import { LIST_CARD_METRICS } from '@/composables/useListCardMetrics'

const to = { name: 'album', params: { id: 'al1' } }

const mountCard = (props: Record<string, unknown> = {}) =>
    mount(ListCard, {
        props: { to, title: 'Album One', ...props },
        global: { stubs: { RouterLink: RouterLinkStub } }
    })

const metaParts = (w: ReturnType<typeof mountCard>) => w.findAll('.meta-part').map((p) => p.text())

describe('ListCard', () => {
    it('links to its item and renders the title, subtitle and meta line', () => {
        const w = mountCard({ subtitle: 'The Artist', meta: ['9 songs', '43 min'] })
        expect(w.findComponent(RouterLinkStub).props('to')).toEqual(to)
        expect(w.find('.card-title').text()).toBe('Album One')
        expect(w.find('.card-subtitle').text()).toBe('The Artist')
        expect(metaParts(w)).toEqual(['9 songs', '43 min'])
    })

    it('drops empty meta parts, and the lines that are left with nothing', () => {
        expect(metaParts(mountCard({ meta: ['', '43 min'] }))).toEqual(['43 min'])
        const empty = mountCard({ meta: [''], details: [''] })
        expect(empty.find('.card-facts').exists()).toBe(false)
        expect(empty.find('.card-subtitle').exists()).toBe(false)
    })

    it('puts the details on a second line, under the meta', () => {
        const w = mountCard({ meta: ['2004', '9 songs'], details: ['EP', '', 'Latin Pop'] })
        const lines = w.findAll('.card-facts > .card-line')
        expect(lines).toHaveLength(2)
        expect(lines[0].classes()).toContain('card-meta')
        expect(lines[1].classes()).toContain('card-details')
        expect(lines[1].findAll('.meta-part').map((p) => p.text())).toEqual(['EP', 'Latin Pop'])
    })

    it('shows the details alone when there is no meta', () => {
        const w = mountCard({ details: ['Latin Pop'] })
        expect(w.find('.card-meta').exists()).toBe(false)
        expect(w.find('.card-details').text()).toBe('Latin Pop')
    })

    it("renders a part's icon after its text, named for screen readers", () => {
        const label = 'Shared by bob — view only'
        const w = mountCard({
            meta: ['3 songs', { text: 'by bob', icon: 'ms-lock', iconLabel: label }]
        })
        expect(metaParts(w)).toEqual(['3 songs', 'by bob'])
        const icon = w.findAll('.meta-part')[1].find('i')
        expect(icon.classes()).toContain('ms-lock')
        expect(icon.attributes('role')).toBe('img')
        expect(icon.attributes('aria-label')).toBe(label)
        expect(w.findAll('.meta-part')[0].find('i').exists()).toBe(false)
    })

    it('shows the cover', () => {
        const w = mountCard({ coverUrl: 'cover:1' })
        expect(w.find('.card-cover img').attributes('src')).toBe('cover:1')
        // The title beside it names the item, so the image is not announced too.
        expect(w.find('.card-cover img').attributes('alt')).toBe('')
    })

    it('shows the placeholder icon without a cover', () => {
        const w = mountCard({ placeholderIcon: 'ms-person' })
        expect(w.find('.cover-placeholder i').classes()).toContain('ms-person')
        expect(w.find('img').exists()).toBe(false)
    })

    it('rounds the cover only when asked (artists, stations)', () => {
        expect(mountCard({ round: true }).find('.list-card').classes()).toContain('round')
        expect(mountCard().find('.list-card').classes()).not.toContain('round')
    })

    // jsdom has no matchMedia, so the viewport reads as the desktop tier.
    it('binds the size the virtual lists reserve for a card', () => {
        const { cover, pad, gap } = LIST_CARD_METRICS.wide
        const style = mountCard().find('.list-card').attributes('style')
        expect(style).toContain(`--list-card-cover: ${cover}px`)
        expect(style).toContain(`--list-card-pad: ${pad}px`)
        expect(style).toContain(`--list-card-gap: ${gap}px`)
    })
})

describe('ListCard placeholder', () => {
    const mountPlaceholder = () =>
        mount(ListCard, {
            props: { starrable: true, coverUrl: 'cover:1' },
            global: { stubs: { RouterLink: RouterLinkStub } }
        })

    it('renders the card shape with no link, image or heart while the item loads', () => {
        const w = mountPlaceholder()
        expect(w.findComponent(RouterLinkStub).exists()).toBe(false)
        const card = w.find('.list-card.placeholder')
        expect(card.exists()).toBe(true)
        expect(card.attributes('aria-hidden')).toBe('true')
        expect(w.find('img').exists()).toBe(false)
        expect(w.find('.row-star').exists()).toBe(false)
    })

    // The same size as a loaded card, so the slot does not jump when it arrives.
    it('takes the size of a loaded card', () => {
        const { cover } = LIST_CARD_METRICS.wide
        expect(mountPlaceholder().find('.list-card').attributes('style')).toContain(
            `--list-card-cover: ${cover}px`
        )
    })
})

describe('ListCard favorite toggle', () => {
    it('renders only when the item can be starred', () => {
        expect(mountCard().find('.row-star').exists()).toBe(false)
        expect(mountCard({ starrable: true }).find('.row-star').exists()).toBe(true)
    })

    it('shows and names the current state', () => {
        const off = mountCard({ starrable: true }).find('.row-star')
        expect(off.attributes('aria-label')).toBe('Add to favorites')
        expect(off.find('i').classes()).toContain('mso-favorite')
        expect(off.classes()).not.toContain('is-starred')

        const on = mountCard({ starrable: true, starred: true }).find('.row-star')
        expect(on.attributes('aria-label')).toBe('Remove from favorites')
        expect(on.find('i').classes()).toContain('ms-favorite')
        expect(on.classes()).toContain('is-starred')
    })

    // The card is a link, so the heart must not open the item.
    it('emits star and swallows the click', () => {
        const w = mountCard({ starrable: true })
        const click = new MouseEvent('click', { bubbles: true, cancelable: true })
        const stop = vi.spyOn(click, 'stopPropagation')
        w.find('.row-star').element.dispatchEvent(click)
        expect(click.defaultPrevented).toBe(true)
        expect(stop).toHaveBeenCalled()
        expect(w.emitted('star')).toHaveLength(1)
    })
})
