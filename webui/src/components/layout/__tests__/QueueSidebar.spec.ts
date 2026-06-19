import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'

const sidebarCollapsed = ref(false)
vi.mock('@/composables/useQueueSidebar', () => ({
    useQueueSidebar: () => ({
        sidebarCollapsed,
        sidebarWidth: ref(480),
        toggleSidebar: vi.fn(),
        setSidebarWidth: vi.fn()
    })
}))

const queue = ref<any[]>([])
const playQueueItem = vi.fn()
vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ queue, currentIndex: ref(0), playQueueItem })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '' }
}))

vi.mock('@/components/layout/QueueView.vue', () => ({
    default: {
        name: 'QueueView',
        props: ['variant'],
        template: '<div class="stub-queue-view">{{ variant }}<slot name="header-start" /></div>'
    }
}))

import QueueSidebar from '@/components/layout/QueueSidebar.vue'

const mountSidebar = () =>
    mount(QueueSidebar, { global: { plugins: [PrimeVue], directives: { tooltip: {} } } })

beforeEach(() => {
    sidebarCollapsed.value = false
    queue.value = []
    playQueueItem.mockReset()
})

describe('QueueSidebar', () => {
    it('renders QueueView (sidebar variant) when expanded', () => {
        sidebarCollapsed.value = false
        const w = mountSidebar()
        expect(w.find('.stub-queue-view').text()).toContain('sidebar')
    })

    it('shows the collapsed cover strip and no QueueView when collapsed', () => {
        sidebarCollapsed.value = true
        queue.value = [{ id: '1', title: 'A', artist: 'B' }]
        const w = mountSidebar()
        expect(w.find('.stub-queue-view').exists()).toBe(false)
        expect(w.find('.collapsed-item').exists()).toBe(true)
    })

    it('clicking a collapsed cover plays that queue item', async () => {
        sidebarCollapsed.value = true
        queue.value = [{ id: '1', title: 'A', artist: 'B' }]
        const w = mountSidebar()
        await w.find('.collapsed-item').trigger('click')
        expect(playQueueItem).toHaveBeenCalledWith(0)
    })
})
