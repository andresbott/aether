import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick, reactive } from 'vue'
import MobileTabBar from '../MobileTabBar.vue'

const push = vi.fn()
const route = reactive({ name: 'home', params: {}, fullPath: '/' })

vi.mock('vue-router', () => ({
    useRouter: () => ({ push }),
    useRoute: () => route
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
    route.name = 'home'
    route.fullPath = '/'
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

    it('marks the current route active, visually and for AT', () => {
        route.name = 'search'
        const bar = mountBar()
        const active = bar.findAll('.tab-item.active')
        expect(active).toHaveLength(1)
        expect(active[0].text()).toBe('Search')
        // aria-current: the colour change alone says nothing to a screen reader.
        expect(active[0].attributes('aria-current')).toBe('page')
        expect(bar.findAll('[aria-current="page"]')).toHaveLength(1)
    })

    it('More opens the drawer instead of navigating', async () => {
        const bar = mountBar()
        await bar.findAll('.tab-item')[4].trigger('click')
        expect(push).not.toHaveBeenCalled()
        expect(bar.findComponent({ name: 'MobileMoreDrawer' }).props('visible')).toBe(true)
    })

    // The tab bar never unmounts on navigation, so without this a system-back
    // press navigated the app UNDERNEATH a drawer that stayed open.
    it('closes the More drawer when the route changes underneath it', async () => {
        const bar = mountBar()
        await bar.findAll('.tab-item')[4].trigger('click')
        expect(bar.findComponent({ name: 'MobileMoreDrawer' }).props('visible')).toBe(true)
        route.fullPath = '/genres'
        await nextTick()
        expect(bar.findComponent({ name: 'MobileMoreDrawer' }).props('visible')).toBe(false)
    })
})
