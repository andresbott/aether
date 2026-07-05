import { describe, it, expect } from 'vitest'
import {
    EXECUTION_STATUS,
    getStatusSeverity,
    getStatusLabel,
    isActiveStatus,
    hasActiveExecutions,
    deriveTasksWithLastExecution
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
            { id: 'scan', name: 'Scan', description: '' },
            { id: 'scan-full', name: 'Full', description: '' }
        ]
        const execs: ExecutionInfo[] = [
            { id: '1', task_name: 'scan', status: 'complete', queued_at: '2026-01-01T10:00:00Z', ended_at: '' },
            { id: '2', task_name: 'scan', status: 'running', queued_at: '2026-01-01T12:00:00Z', ended_at: '' }
        ]
        const out = deriveTasksWithLastExecution(tasks, execs)
        expect(out.find((t) => t.id === 'scan')?.lastExecutionStatus).toBe('running')
        expect(out.find((t) => t.id === 'scan-full')?.lastExecutionStatus).toBeNull()
    })
})
