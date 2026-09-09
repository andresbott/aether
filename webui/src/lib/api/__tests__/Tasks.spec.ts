import { describe, it, expect, vi, beforeEach } from 'vitest'

const get = vi.fn()
const post = vi.fn()
const put = vi.fn()
const patch = vi.fn()
const del = vi.fn()

vi.mock('@/lib/api/client', () => ({
    apiClient: {
        get: (...a: unknown[]) => get(...a),
        post: (...a: unknown[]) => post(...a),
        put: (...a: unknown[]) => put(...a),
        patch: (...a: unknown[]) => patch(...a),
        delete: (...a: unknown[]) => del(...a)
    }
}))

import * as Tasks from '@/lib/api/Tasks'

beforeEach(() => {
    get.mockReset(); post.mockReset(); put.mockReset(); patch.mockReset(); del.mockReset()
})

describe('Tasks API', () => {
    it('listExecutions hits the global executions endpoint', async () => {
        get.mockResolvedValue({ data: { executions: [{ id: 'a' }] } })
        const res = await Tasks.listExecutions()
        expect(get).toHaveBeenCalledWith('/tasks/executions', { signal: undefined })
        expect(res).toEqual([{ id: 'a' }])
    })

    it('triggerTask posts to /tasks/{name}/trigger with no body and returns the result with the reused flag', async () => {
        post.mockResolvedValue({ data: { execution_id: 'xyz', reused: true } })
        const res = await Tasks.triggerTask('scan')
        expect(post).toHaveBeenCalledWith('/tasks/scan/trigger')
        expect(res).toEqual({ execution_id: 'xyz', reused: true })
    })

    it('cancelExecution posts to /tasks/executions/{id}/cancel', async () => {
        post.mockResolvedValue({ data: {} })
        await Tasks.cancelExecution('eid')
        expect(post).toHaveBeenCalledWith('/tasks/executions/eid/cancel')
    })

    it('getExecutionLog requests the logs as text', async () => {
        get.mockResolvedValue({ data: 'line1\nline2' })
        const text = await Tasks.getExecutionLog('eid')
        expect(get).toHaveBeenCalledWith('/tasks/executions/eid/logs', { responseType: 'text' })
        expect(text).toBe('line1\nline2')
    })

    it('createSchedule POSTs the body to /tasks/{name}/schedules', async () => {
        post.mockResolvedValue({ data: { id: 's1', task_name: 'scan', cron_expression: '0 0 0 * * *', enabled: true } })
        await Tasks.createSchedule('scan', { cron_expression: '0 0 0 * * *', enabled: true, params: { full: false } })
        expect(post).toHaveBeenCalledWith('/tasks/scan/schedules', {
            cron_expression: '0 0 0 * * *',
            enabled: true,
            params: { full: false }
        })
    })

    it('patchSchedule PATCHes /tasks/{name}/schedules/{id} with the body', async () => {
        patch.mockResolvedValue({ data: { id: 's1', task_name: 'scan', cron_expression: '0 0 0 * * *', enabled: false } })
        await Tasks.patchSchedule('scan', 's1', { enabled: false })
        expect(patch).toHaveBeenCalledWith('/tasks/scan/schedules/s1', { enabled: false })
    })

    it('deleteSchedule DELETEs /tasks/{name}/schedules/{id}', async () => {
        del.mockResolvedValue({ data: {} })
        await Tasks.deleteSchedule('scan', 's1')
        expect(del).toHaveBeenCalledWith('/tasks/scan/schedules/s1')
    })
})
