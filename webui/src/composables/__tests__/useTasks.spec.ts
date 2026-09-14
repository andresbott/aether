import { describe, it, expect } from 'vitest'
import {
    EXECUTION_STATUS,
    getStatusSeverity,
    getStatusLabel,
    isActiveStatus,
    hasActiveExecutions,
    deriveTasksWithLastExecution,
    progressLabel
} from '@/composables/useTasks'
import type { ExecutionInfo, TaskWithSchedule } from '@/types/tasks'

describe('status helpers', () => {
    it('maps tempo statuses to severities', () => {
        expect(getStatusSeverity(EXECUTION_STATUS.complete)).toBe('success')
        expect(getStatusSeverity(EXECUTION_STATUS.failed)).toBe('danger')
        expect(getStatusSeverity(EXECUTION_STATUS.panicked)).toBe('danger')
        expect(getStatusSeverity(EXECUTION_STATUS.waiting)).toBe('warn')
        expect(getStatusSeverity(EXECUTION_STATUS.running)).toBe('info')
        expect(getStatusSeverity('something-else')).toBe('secondary')
    })

    it('labels waiting as queued', () => {
        expect(getStatusLabel('waiting')).toBe('queued')
        expect(getStatusLabel('running')).toBe('running')
    })

    it('detects active executions', () => {
        expect(isActiveStatus('running')).toBe(true)
        expect(isActiveStatus('complete')).toBe(false)
        expect(hasActiveExecutions([{ status: 'complete' } as ExecutionInfo])).toBe(false)
        expect(hasActiveExecutions([{ status: 'waiting' } as ExecutionInfo])).toBe(true)
        expect(hasActiveExecutions([])).toBe(false)
        expect(hasActiveExecutions(undefined)).toBe(false)
    })
})

describe('deriveTasksWithLastExecution', () => {
    it('attaches the newest execution status per task', () => {
        const tasks: TaskWithSchedule[] = [
            { id: 'scan', name: 'Scan', description: '', schedules: [] },
            { id: 'fetch-artist-images', name: 'Full', description: '', schedules: [] }
        ]
        const execs: ExecutionInfo[] = [
            { id: '1', task_name: 'scan', status: 'complete', queued_at: '2026-01-01T10:00:00Z', ended_at: '' },
            { id: '2', task_name: 'scan', status: 'running', queued_at: '2026-01-01T12:00:00Z', ended_at: '' }
        ]
        const out = deriveTasksWithLastExecution(tasks, execs)
        expect(out.find((t) => t.id === 'scan')?.lastExecutionStatus).toBe('running')
        expect(out.find((t) => t.id === 'fetch-artist-images')?.lastExecutionStatus).toBeNull()
    })

    it('carries the newest execution progress onto the task', () => {
        const tasks: TaskWithSchedule[] = [{ id: 'scan', name: 'Scan', description: '', schedules: [] }]
        const execs: ExecutionInfo[] = [
            {
                id: '2',
                task_name: 'scan',
                status: 'running',
                queued_at: '2026-01-01T12:00:00Z',
                ended_at: '',
                progress: { done: 2, total: 4, stage: 'Saving: a/b.mp3' }
            }
        ]
        const out = deriveTasksWithLastExecution(tasks, execs)
        expect(out[0].lastExecutionProgress).toEqual({ done: 2, total: 4, stage: 'Saving: a/b.mp3' })
    })

    it('has null progress when a task has no executions', () => {
        const tasks: TaskWithSchedule[] = [{ id: 'scan', name: 'Scan', description: '', schedules: [] }]
        const out = deriveTasksWithLastExecution(tasks, [])
        expect(out[0].lastExecutionProgress).toBeNull()
    })
})

describe('progressLabel', () => {
    it('formats a percentage when total is known', () => {
        expect(progressLabel('running', { done: 1, total: 4 })).toBe('25% complete')
    })
    it('says Queued for a waiting task with no measurable progress', () => {
        expect(progressLabel('waiting', null)).toBe('Queued')
    })
    it('falls back to Running when progress is indeterminate', () => {
        expect(progressLabel('running', { done: 0, total: 0 })).toBe('Running')
    })
})
