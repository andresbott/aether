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
        expect(get).toHaveBeenCalledWith('/tasks/executions')
        expect(res).toEqual([{ id: 'a' }])
    })

    it('triggerTask posts to /tasks/{name}/trigger and returns the execution id', async () => {
        post.mockResolvedValue({ data: { execution_id: 'xyz' } })
        const id = await Tasks.triggerTask('scan')
        expect(post).toHaveBeenCalledWith('/tasks/scan/trigger')
        expect(id).toBe('xyz')
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

    it('upsertTask PUTs the body to /tasks/{name}', async () => {
        put.mockResolvedValue({ data: { id: 'scan' } })
        await Tasks.upsertTask('scan', { cron_expression: '0 0 0 * * *', enabled: true })
        expect(put).toHaveBeenCalledWith('/tasks/scan', { cron_expression: '0 0 0 * * *', enabled: true })
    })

    it('deleteTaskSchedule DELETEs /tasks/{name}', async () => {
        del.mockResolvedValue({ data: {} })
        await Tasks.deleteTaskSchedule('scan')
        expect(del).toHaveBeenCalledWith('/tasks/scan')
    })
})
