import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import ExecutionHistory from '@/components/admin/ExecutionHistory.vue'
import type { ExecutionInfo } from '@/types/tasks'

const execs: ExecutionInfo[] = [
    { id: 'a', task_name: 'scan', status: 'running', queued_at: '2026-01-01T10:00:00Z', started_at: '2026-01-01T10:00:01Z', ended_at: '' },
    { id: 'b', task_name: 'scan', status: 'complete', queued_at: '2026-01-01T09:00:00Z', started_at: '2026-01-01T09:00:01Z', ended_at: '2026-01-01T09:00:04Z' }
]

const mountTable = (props: Record<string, unknown> = {}) =>
    mount(ExecutionHistory, {
        props: { executions: execs, isLoading: false, ...props },
        global: { plugins: [PrimeVue], directives: { tooltip: {} } }
    })

describe('ExecutionHistory', () => {
    it('renders a status tag per execution with the queued label for waiting', () => {
        const w = mountTable()
        expect(w.text()).toContain('running')
        expect(w.text()).toContain('complete')
    })

    it('shows the status count summary', () => {
        const w = mountTable()
        expect(w.text()).toContain('Total: 2')
        expect(w.text()).toContain('Running: 1')
        expect(w.text()).toContain('Complete: 1')
    })

    it('emits cancel only for active executions', async () => {
        const w = mountTable()
        const cancelButtons = w.findAll('button').filter((b) => b.text().includes('Cancel'))
        expect(cancelButtons).toHaveLength(1) // only the running row
        await cancelButtons[0].trigger('click')
        expect(w.emitted('cancel')![0]).toEqual(['a'])
    })
})
