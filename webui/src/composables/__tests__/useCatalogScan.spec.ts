import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import type { MockInstance } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import type { ExecutionInfo } from '@/types/tasks'

const triggerTask = vi.fn()
const listExecutions = vi.fn()
vi.mock('@/lib/api/Tasks', () => ({
    triggerTask: (...a: unknown[]) => triggerTask(...a),
    listExecutions: (...a: unknown[]) => listExecutions(...a),
    listTasks: vi.fn(),
    cancelExecution: vi.fn(),
    getTask: vi.fn(),
    createSchedule: vi.fn(),
    patchSchedule: vi.fn(),
    deleteSchedule: vi.fn(),
    getExecutionLog: vi.fn()
}))

const toastAdd = vi.fn()
vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: toastAdd }) }))

import { useCatalogScan } from '@/composables/useCatalogScan'

let wrapper: VueWrapper | undefined
let queryClient: QueryClient
let invalidate: MockInstance

function mountScan() {
    const captured: { api?: ReturnType<typeof useCatalogScan> } = {}
    const Host = defineComponent({
        setup() {
            captured.api = useCatalogScan()
            return () => h('div')
        }
    })
    queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    invalidate = vi.spyOn(queryClient, 'invalidateQueries')
    wrapper = mount(Host, { global: { plugins: [[VueQueryPlugin, { queryClient }]] } })
    return captured.api!
}

// Query results reach their observers on a zero-delay timer and the watcher
// runs after that, so a few turns of the event loop are needed, not one.
async function settle() {
    await flushPromises()
    await flushPromises()
    await flushPromises()
}

// One more round of the executions poll.
async function poll() {
    await queryClient.refetchQueries({ queryKey: ['tasks', 'executions'] })
    await settle()
}

// What a settled scan refreshed — the trigger's own refetch of the
// executions list is bookkeeping, not a refresh of the page.
function refreshed(): unknown[] {
    return invalidate.mock.calls
        .map(([filters]) => (filters as { queryKey?: unknown[] } | undefined)?.queryKey)
        .filter((key) => key?.[0] !== 'tasks')
}

const PAGE_DATA = [['scan-folders'], ['libraries'], ['subsonic']]

function run(over: Partial<ExecutionInfo> = {}): ExecutionInfo {
    return {
        id: 'r1',
        task_name: 'scan',
        status: 'running',
        queued_at: '2026-09-21T10:00:00Z',
        ended_at: '',
        ...over
    }
}

beforeEach(() => {
    triggerTask.mockReset()
    listExecutions.mockReset().mockResolvedValue([])
    toastAdd.mockReset()
})

afterEach(() => {
    wrapper?.unmount()
    queryClient?.clear()
})

describe('useCatalogScan trigger', () => {
    it('triggers the incremental scan task by name', async () => {
        triggerTask.mockResolvedValue({ execution_id: 'r1', reused: false })
        const scan = mountScan()
        await settle()

        scan.start()
        await settle()

        expect(triggerTask).toHaveBeenCalledWith('scan')
        expect(toastAdd).not.toHaveBeenCalled()
    })

    it('says so when the trigger joined a scan already in progress', async () => {
        triggerTask.mockResolvedValue({ execution_id: 'r1', reused: true })
        const scan = mountScan()
        await settle()

        scan.start()
        await settle()

        expect(toastAdd).toHaveBeenCalledWith(
            expect.objectContaining({ severity: 'info', summary: 'Already in progress' })
        )
    })

    it('reports a trigger the server refused', async () => {
        triggerTask.mockRejectedValue(new Error('task queue is full'))
        const scan = mountScan()
        await settle()

        scan.start()
        await settle()

        expect(toastAdd).toHaveBeenCalledWith(
            expect.objectContaining({
                severity: 'error',
                summary: 'Could not start the scan',
                detail: 'task queue is full'
            })
        )
        expect(scan.starting.value).toBe(false)
    })
})

describe('useCatalogScan progress', () => {
    it('is running, with the run’s progress, while a scan runs', async () => {
        listExecutions.mockResolvedValue([run({ progress: { done: 1, total: 4 } })])
        const scan = mountScan()
        await settle()

        expect(scan.running.value).toBe(true)
        expect(scan.progressText.value).toBe('25% complete')
    })

    it('is running while a scan waits in the queue', async () => {
        listExecutions.mockResolvedValue([run({ status: 'waiting' })])
        const scan = mountScan()
        await settle()

        expect(scan.running.value).toBe(true)
        expect(scan.progressText.value).toBe('Queued')
    })

    it('is idle once the last scan has settled', async () => {
        listExecutions.mockResolvedValue([run({ status: 'complete' })])
        const scan = mountScan()
        await settle()

        expect(scan.running.value).toBe(false)
    })
})

describe('useCatalogScan when a scan settles', () => {
    it('refreshes the page’s data when a scan it saw running completes', async () => {
        listExecutions.mockResolvedValue([run()])
        mountScan()
        await settle()

        listExecutions.mockResolvedValue([run({ status: 'complete' })])
        await poll()

        expect(refreshed()).toEqual(PAGE_DATA)
        expect(toastAdd).not.toHaveBeenCalled()
    })

    it('reports a scan that failed, and still refreshes', async () => {
        listExecutions.mockResolvedValue([run()])
        mountScan()
        await settle()

        listExecutions.mockResolvedValue([run({ status: 'failed' })])
        await poll()

        expect(refreshed()).toEqual(PAGE_DATA)
        expect(toastAdd).toHaveBeenCalledWith(
            expect.objectContaining({ severity: 'error', summary: 'Catalog scan failed' })
        )
    })

    // Canceling happens on the Tasks page, on purpose: nothing to report.
    it('does not report a canceled scan', async () => {
        listExecutions.mockResolvedValue([run()])
        mountScan()
        await settle()

        listExecutions.mockResolvedValue([run({ status: 'canceled' })])
        await poll()

        expect(refreshed()).toEqual(PAGE_DATA)
        expect(toastAdd).not.toHaveBeenCalled()
    })

    it('leaves the page alone for a scan that had settled before it loaded', async () => {
        listExecutions.mockResolvedValue([run({ status: 'failed' })])
        mountScan()
        await settle()
        await poll()

        expect(refreshed()).toEqual([])
        expect(toastAdd).not.toHaveBeenCalled()
    })

    it('refreshes after a triggered scan that finished before any poll saw it running', async () => {
        triggerTask.mockResolvedValue({ execution_id: 'r2', reused: false })
        const scan = mountScan()
        await settle()

        listExecutions.mockResolvedValue([run({ id: 'r2', status: 'complete' })])
        scan.start()
        await settle()

        expect(refreshed()).toEqual(PAGE_DATA)
    })

    // A poll can fetch the finished run before the trigger has returned its
    // id; the refetch after the trigger then returns the very same list.
    it('refreshes when a poll saw the triggered run finish before the trigger returned', async () => {
        let resolveTrigger!: (value: unknown) => void
        triggerTask.mockReturnValue(new Promise((resolve) => (resolveTrigger = resolve)))
        const scan = mountScan()
        await settle()

        scan.start()
        listExecutions.mockResolvedValue([run({ id: 'r2', status: 'complete' })])
        await poll()
        expect(refreshed()).toEqual([])

        resolveTrigger({ execution_id: 'r2', reused: false })
        await settle()

        expect(refreshed()).toEqual(PAGE_DATA)
    })
})
