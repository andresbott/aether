import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'

const triggerTask = vi.fn()
const listTasks = vi.fn()
const listExecutions = vi.fn()
vi.mock('@/lib/api/Tasks', () => ({
    triggerTask: (...a: unknown[]) => triggerTask(...a),
    listTasks: (...a: unknown[]) => listTasks(...a),
    listExecutions: (...a: unknown[]) => listExecutions(...a),
    cancelExecution: vi.fn(),
    getTask: vi.fn(),
    upsertTask: vi.fn(),
    patchTask: vi.fn(),
    deleteTaskSchedule: vi.fn(),
    getExecutionLog: vi.fn()
}))

const toastAdd = vi.fn()
vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: toastAdd }) }))

import { useTasks } from '@/composables/useTasks'
import type { Task } from '@/composables/useTasks'

function withTasks() {
    const captured: { api?: ReturnType<typeof useTasks> } = {}
    const Host = defineComponent({
        setup() {
            captured.api = useTasks()
            return () => h('div')
        }
    })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    mount(Host, { global: { plugins: [[VueQueryPlugin, { queryClient }]] } })
    return { captured }
}

const scanTask = { id: 'scan', name: 'Scan', description: '' } as Task

beforeEach(() => {
    triggerTask.mockReset()
    listTasks.mockReset().mockResolvedValue([])
    listExecutions.mockReset().mockResolvedValue([])
    toastAdd.mockReset()
})

describe('useTasks trigger coalescing feedback', () => {
    it('shows an info toast when a trigger coalesces onto a running run (reused)', async () => {
        triggerTask.mockResolvedValue({ execution_id: 'x', reused: true })
        const { captured } = withTasks()

        captured.api!.triggerTask(scanTask)
        await flushPromises()
        await flushPromises()

        expect(toastAdd).toHaveBeenCalledTimes(1)
        expect(toastAdd.mock.calls[0][0]).toMatchObject({ severity: 'info' })
    })

    it('shows no toast when a fresh run is enqueued (not reused)', async () => {
        triggerTask.mockResolvedValue({ execution_id: 'x', reused: false })
        const { captured } = withTasks()

        captured.api!.triggerTask(scanTask)
        await flushPromises()
        await flushPromises()

        expect(toastAdd).not.toHaveBeenCalled()
    })
})
