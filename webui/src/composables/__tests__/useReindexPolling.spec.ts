import { describe, it, expect, vi, beforeEach } from 'vitest'
import { pollReindex } from '@/composables/useReindexPolling'
import * as TasksApi from '@/lib/api/Tasks'

vi.mock('@/lib/api/Tasks')

const exec = (id: string, status: string) => ({ id, task_name: 'reindex', status, queued_at: '', ended_at: '' })

describe('pollReindex', () => {
    beforeEach(() => vi.restoreAllMocks())

    it('settles with nothing failed or pending when every id reaches complete', async () => {
        vi.mocked(TasksApi.listExecutions)
            .mockResolvedValueOnce([exec('a', 'running')])
            .mockResolvedValue([exec('a', 'complete')])
        const res = await pollReindex(['a'], { intervalMs: 1, timeoutMs: 1000 })
        expect(res).toEqual({ failed: 0, pending: 0 })
    })

    it('counts an execution that ends in a non-complete terminal status as failed', async () => {
        vi.mocked(TasksApi.listExecutions).mockResolvedValue([exec('a', 'failed')])
        const res = await pollReindex(['a'], { intervalMs: 1, timeoutMs: 1000 })
        expect(res).toEqual({ failed: 1, pending: 0 })
    })

    it('returns nothing to settle for an empty id list without polling', async () => {
        const res = await pollReindex([], { intervalMs: 1, timeoutMs: 1000 })
        expect(res).toEqual({ failed: 0, pending: 0 })
        expect(TasksApi.listExecutions).not.toHaveBeenCalled()
    })

    it('treats an id seen running that later drops out of history as complete', async () => {
        vi.mocked(TasksApi.listExecutions)
            .mockResolvedValueOnce([exec('a', 'running')])
            .mockResolvedValue([exec('other', 'complete')]) // 'a' rolled out after being seen
        const res = await pollReindex(['a'], { intervalMs: 1, timeoutMs: 1000 })
        expect(res).toEqual({ failed: 0, pending: 0 })
    })

    it('keeps an id that has NEVER been seen pending instead of assuming complete', async () => {
        // A non-empty list that never contains 'a' (enqueue lag or a history
        // window too small to hold it): the old heuristic deleted it as complete
        // on the first poll. It must stay pending and time out — never a false
        // success.
        vi.mocked(TasksApi.listExecutions).mockResolvedValue([exec('other', 'complete')])
        const res = await pollReindex(['a'], { intervalMs: 1, timeoutMs: 20 })
        expect(res).toEqual({ failed: 0, pending: 1 })
    })

    it('reports the id pending (not a clean success) when the executions poll keeps failing', async () => {
        vi.mocked(TasksApi.listExecutions).mockRejectedValue(new Error('network down'))
        const res = await pollReindex(['a'], { intervalMs: 1, timeoutMs: 20 })
        expect(res).toEqual({ failed: 0, pending: 1 })
    })

    it('returns immediately with the id pending when the signal is already aborted', async () => {
        const controller = new AbortController()
        controller.abort()
        const res = await pollReindex(['a'], { intervalMs: 1, timeoutMs: 50, signal: controller.signal })
        expect(res).toEqual({ failed: 0, pending: 1 })
        expect(TasksApi.listExecutions).not.toHaveBeenCalled()
    })
})
