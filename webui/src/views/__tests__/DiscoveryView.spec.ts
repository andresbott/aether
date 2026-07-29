import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

const replaceSpy = vi.fn()
const route = { query: {} as Record<string, unknown> }
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ replace: replaceSpy, push: vi.fn() })
}))

vi.mock('@/components/library/DiscoverySection.vue', () => ({
    default: {
        name: 'DiscoverySection',
        props: ['sectionKey', 'layout'],
        template: '<div class="stub-section" :data-key="sectionKey" :data-layout="layout"></div>'
    }
}))

import DiscoveryView from '@/views/DiscoveryView.vue'

const stubs = { RouterLink: { template: '<a><slot /></a>' } }
const mountView = () =>
    mount(DiscoveryView, { global: { plugins: [PrimeVue], directives: { tooltip: {} }, stubs } })

beforeEach(() => {
    replaceSpy.mockReset()
    route.query = {}
})

describe('DiscoveryView', () => {
    it('renders the Discovery title', () => {
        expect(mountView().text()).toContain('Discovery')
    })

    it('renders one shelf per section in registry order', () => {
        const keys = mountView()
            .findAll('.stub-section')
            .map((n) => n.attributes('data-key'))
        expect(keys).toEqual([
            'recently-added',
            'favorites',
            'most-played',
            'recently-played',
            'random'
        ])
    })

    it('passes the grid layout to every shelf by default', () => {
        const layouts = mountView()
            .findAll('.stub-section')
            .map((n) => n.attributes('data-layout'))
        expect(new Set(layouts)).toEqual(new Set(['grid']))
    })

    it('passes the list layout to every shelf when the query says list', () => {
        route.query = { view: 'list' }
        const layouts = mountView()
            .findAll('.stub-section')
            .map((n) => n.attributes('data-layout'))
        expect(new Set(layouts)).toEqual(new Set(['list']))
    })
})
