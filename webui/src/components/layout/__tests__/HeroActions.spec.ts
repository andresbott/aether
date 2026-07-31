import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import HeroActions from '@/components/layout/HeroActions.vue'

const mountActions = (props: Record<string, unknown> = {}) =>
    mount(HeroActions, {
        props,
        global: { plugins: [PrimeVue], directives: { tooltip: {} } }
    })

describe('HeroActions', () => {
    it('always renders Play and emits play on click', async () => {
        const w = mountActions()
        const play = w.find('.hero-action-play')
        expect(play.exists()).toBe(true)
        await play.trigger('click')
        expect(w.emitted('play')).toHaveLength(1)
    })

    it('omits Add to queue and Favorite by default', () => {
        const w = mountActions()
        expect(w.find('.hero-action-queue').exists()).toBe(false)
        expect(w.find('.hero-action-star').exists()).toBe(false)
    })

    it('renders Add to queue when canQueue and emits queue', async () => {
        const w = mountActions({ canQueue: true })
        const q = w.find('.hero-action-queue')
        expect(q.exists()).toBe(true)
        await q.trigger('click')
        expect(w.emitted('queue')).toHaveLength(1)
    })

    it('renders a filled heart when canStar and starred, and emits star', async () => {
        const w = mountActions({ canStar: true, starred: true })
        const star = w.find('.hero-action-star')
        expect(star.exists()).toBe(true)
        expect(star.find('.pi-heart-fill').exists()).toBe(true)
        await star.trigger('click')
        expect(w.emitted('star')).toHaveLength(1)
    })

    it('shows the outline heart when not starred', () => {
        const w = mountActions({ canStar: true, starred: false })
        expect(w.find('.hero-action-star .pi-heart').exists()).toBe(true)
        expect(w.find('.hero-action-star .pi-heart-fill').exists()).toBe(false)
    })

    it('labels the favorite toggle for screen readers', () => {
        const off = mountActions({ canStar: true, starred: false })
        expect(off.find('.hero-action-star').attributes('aria-label')).toBe('Add to favorites')
        const on = mountActions({ canStar: true, starred: true })
        expect(on.find('.hero-action-star').attributes('aria-label')).toBe('Remove from favorites')
    })

    it('disables Play when playDisabled', () => {
        const w = mountActions({ playDisabled: true })
        expect(w.find('.hero-action-play').attributes('disabled')).toBeDefined()
    })

    it('puts Play in a loading state when busy', () => {
        const w = mountActions({ busy: true })
        expect(w.find('.hero-action-play').classes()).toContain('p-button-loading')
    })

    it('uses a custom play label', () => {
        const w = mountActions({ playLabel: 'Play all' })
        expect(w.find('.hero-action-play').text()).toContain('Play all')
    })
})
