import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import MobileTabBar from '../MobileTabBar.vue'

const push = vi.fn()
let routeName = 'home'

vi.mock('vue-router', () => ({
    useRouter: () => ({ push }),
    useRoute: () => ({ name: routeName, params: {} })
}))

// The drawer subtree pulls auth + folder queries; stub it out — it has its
// own spec.
const mountBar = () =>
    mount(MobileTabBar, {
        global: {
            stubs: {
                MobileMoreDrawer: {
                    name: 'MobileMoreDrawer',
                    template: '<div data-stub-drawer />',
                    props: ['visible']
                }
            }
        }
    })

beforeEach(() => {
    push.mockClear()
    routeName = 'home'
})

describe('MobileTabBar', () => {
    it('renders the four nav tabs plus More', () => {
        const labels = mountBar().findAll('.tab-item').map((b) => b.text())
        expect(labels).toEqual(['Home', 'Library', 'Search', 'Playlists', 'More'])
    })

    it('navigates to a tab route by name', async () => {
        const bar = mountBar()
        await bar.findAll('.tab-item')[1].trigger('click')
        expect(push).toHaveBeenCalledWith({ name: 'library' })
    })

    it('marks the current route active', () => {
        routeName = 'search'
        const bar = mountBar()
        const active = bar.findAll('.tab-item.active')
        expect(active).toHaveLength(1)
        expect(active[0].text()).toBe('Search')
    })

    it('More opens the drawer instead of navigating', async () => {
        const bar = mountBar()
        await bar.findAll('.tab-item')[4].trigger('click')
        expect(push).not.toHaveBeenCalled()
        expect(bar.findComponent({ name: 'MobileMoreDrawer' }).props('visible')).toBe(true)
    })
})
