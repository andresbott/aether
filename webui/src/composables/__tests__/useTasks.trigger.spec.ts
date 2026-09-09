import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'

const triggerTask = vi.fn()
const listTasks = vi.fn()
const listExecutions = vi.fn()
const createSchedule = vi.fn()
const patchSchedule = vi.fn()
const deleteSchedule = vi.fn()
vi.mock('@/lib/api/Tasks', () => ({
    triggerTask: (...a: unknown[]) => triggerTask(...a),
    listTasks: (...a: unknown[]) => listTasks(...a),
    listExecutions: (...a: unknown[]) => listExecutions(...a),
    cancelExecution: vi.fn(),
    getTask: vi.fn(),
    createSchedule: (...a: unknown[]) => createSchedule(...a),
    patchSchedule: (...a: unknown[]) => patchSchedule(...a),
    deleteSchedule: (...a: unknown[]) => deleteSchedule(...a),
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
    createSchedule.mockReset()
    patchSchedule.mockReset()
    deleteSchedule.mockReset()
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

    it('calls the API by task name for a plain trigger', async () => {
        triggerTask.mockResolvedValue({ execution_id: 'x', reused: false })
        const { captured } = withTasks()

        captured.api!.triggerTask(scanTask)
        await flushPromises()
        await flushPromises()

        expect(triggerTask).toHaveBeenCalledWith('scan')
    })
})

describe('useTasks id-based schedule mutations', () => {
    it('creates a schedule for a task by name', async () => {
        createSchedule.mockResolvedValue({
            id: 's1',
            task_name: 'scan',
            cron_expression: '0 0 0 * * *',
            enabled: true,
            created_at: '',
            updated_at: ''
        })
        const { captured } = withTasks()

        await captured.api!.createSchedule('scan', {
            cron_expression: '0 0 0 * * *',
            enabled: true,
            params: { full: false }
        })

        expect(createSchedule).toHaveBeenCalledWith('scan', {
            cron_expression: '0 0 0 * * *',
            enabled: true,
            params: { full: false }
        })
    })

    it('patches a schedule by id, not by task name alone', async () => {
        patchSchedule.mockResolvedValue({
            id: 's1',
            task_name: 'scan',
            cron_expression: '0 0 0 * * *',
            enabled: false,
            created_at: '',
            updated_at: ''
        })
        const { captured } = withTasks()

        await captured.api!.patchSchedule('scan', 's1', { enabled: false })

        expect(patchSchedule).toHaveBeenCalledWith('scan', 's1', { enabled: false })
    })

    it('deletes a schedule by id', async () => {
        deleteSchedule.mockResolvedValue(undefined)
        const { captured } = withTasks()

        await captured.api!.deleteSchedule('scan', 's1')

        expect(deleteSchedule).toHaveBeenCalledWith('scan', 's1')
    })
})
