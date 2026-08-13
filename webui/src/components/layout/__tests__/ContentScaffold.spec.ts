import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { computed, ref } from 'vue'
import PrimeVue from 'primevue/config'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'

// Mock useViewport for secondary actions tests
const tier = ref<'phone' | 'tablet' | 'desktop'>('desktop')
vi.mock('@/composables/useViewport', () => ({
    useViewport: () => ({
        tier: computed(() => tier.value),
        shell: computed(() => (tier.value === 'phone' ? 'mobile' : 'desktop')),
        isTouch: ref(false)
    })
}))

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

describe('secondary actions', () => {
    const mountWithSecondary = () =>
        mount(ContentScaffold, {
            props: { title: 'Radio' },
            slots: {
                actions: '<button data-primary>Add</button>',
                'secondary-actions': '<button data-secondary>Import</button>'
            },
            global: { plugins: [PrimeVue] },
            attachTo: document.body
        })

    it('renders secondary actions inline on desktop', () => {
        tier.value = 'desktop'
        const scaffold = mountWithSecondary()
        expect(scaffold.find('[data-secondary]').exists()).toBe(true)
        expect(scaffold.find('.scaffold-overflow-btn').exists()).toBe(false)
    })

    it('collapses secondary actions behind ⋮ on phone', async () => {
        tier.value = 'phone'
        const scaffold = mountWithSecondary()
        // Not inline…
        expect(scaffold.find('[data-secondary]').exists()).toBe(false)
        const overflow = scaffold.find('.scaffold-overflow-btn')
        expect(overflow.exists()).toBe(true)
        // …but reachable through the popover.
        await overflow.trigger('click')
        expect(document.body.querySelector('[data-secondary]')).toBeTruthy()
    })

    it('renders no ⋮ when the slot is absent', () => {
        tier.value = 'phone'
        const scaffold = mount(ContentScaffold, {
            props: { title: 'Radio' },
            global: { plugins: [PrimeVue] }
        })
        expect(scaffold.find('.scaffold-overflow-btn').exists()).toBe(false)
    })
})
