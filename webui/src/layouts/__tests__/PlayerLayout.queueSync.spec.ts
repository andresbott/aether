import { describe, it, expect, vi, beforeEach } from 'vitest'
import { reactive, ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'

const route = reactive({ name: 'library', meta: { flush: true } })
vi.mock('vue-router', () => ({
    useRoute: () => route,
    RouterView: { template: '<div class="router-outlet" />' },
    RouterLink: { template: '<a><slot /></a>' }
}))

const checkScreenWidth = vi.fn()
vi.mock('@/store/uiStore', () => ({
    useUiStore: () => reactive({ checkScreenWidth, queueSidebarCollapsed: ref(false) })
}))

vi.mock('@/composables/useScrollbarWidth', () => ({
    useScrollbarWidth: () => ref(0)
}))

const start = vi.fn()
const stop = vi.fn()
const restore = vi.fn(() => Promise.resolve())
vi.mock('@/composables/useQueueSync', () => ({
    useQueueSync: () => ({ start, stop, restore })
}))

import PlayerLayout from '@/layouts/PlayerLayout.vue'

const mountLayout = () =>
    mount(PlayerLayout, {
        global: {
            directives: { tooltip: {} },
            stubs: {
                AppSidebar: true,
                PlayerControls: true,
                QueueSidebar: true,
                Toast: true
            }
        }
    })

describe('PlayerLayout play queue sync', () => {
    beforeEach(() => {
        start.mockClear()
        stop.mockClear()
        restore.mockClear()
    })

    // The session has to be pulled once the player shell exists, so a user opening
    // the app in a second browser lands on the queue they left elsewhere.
    it('restores the saved session on mount', async () => {
        mountLayout()
        await flushPromises()
        expect(restore).toHaveBeenCalledTimes(1)
    })

    // Saving must only begin after the restore has been adopted; starting first
    // would let a debounced local save race the incoming server state.
    it('starts syncing after the restore resolves', async () => {
        const order: string[] = []
        restore.mockImplementation(async () => {
            order.push('restore')
        })
        start.mockImplementation(() => {
            order.push('start')
        })

        mountLayout()
        await flushPromises()

        expect(order).toEqual(['restore', 'start'])
    })

    it('stops the sync when the layout unmounts', async () => {
        const w = mountLayout()
        await flushPromises()
        w.unmount()
        expect(stop).toHaveBeenCalledTimes(1)
    })
})
