import { describe, it, expect, vi, beforeEach } from 'vitest'
import { pollReindex } from '@/composables/useReindexPolling'
import * as TasksApi from '@/lib/api/Tasks'

vi.mock('@/lib/api/Tasks')

describe('pollReindex', () => {
    beforeEach(() => vi.restoreAllMocks())

    it('resolves when every id reaches complete', async () => {
        vi.mocked(TasksApi.listExecutions)
            .mockResolvedValueOnce([{ id: 'a', task_name: 'reindex', status: 'running', queued_at: '', ended_at: '' }])
            .mockResolvedValue([{ id: 'a', task_name: 'reindex', status: 'complete', queued_at: '', ended_at: '' }])
        const res = await pollReindex(['a'], { intervalMs: 1, timeoutMs: 1000 })
        expect(res).toEqual({ failed: 0 })
    })

    it('counts a failed execution', async () => {
        vi.mocked(TasksApi.listExecutions).mockResolvedValue([
            { id: 'a', task_name: 'reindex', status: 'failed', queued_at: '', ended_at: '' }
        ])
        const res = await pollReindex(['a'], { intervalMs: 1, timeoutMs: 1000 })
        expect(res).toEqual({ failed: 1 })
    })

    it('returns {failed:0} for an empty id list without polling', async () => {
        const res = await pollReindex([], { intervalMs: 1, timeoutMs: 1000 })
        expect(res).toEqual({ failed: 0 })
        expect(TasksApi.listExecutions).not.toHaveBeenCalled()
    })
})
