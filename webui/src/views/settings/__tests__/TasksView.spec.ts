import { describe, it, expect, vi } from 'vitest'
import { ref, computed } from 'vue'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))

const triggerTask = vi.fn()
vi.mock('@/composables/useTasks', async (importOriginal) => {
    const actual = await importOriginal<typeof import('@/composables/useTasks')>()
    return {
        ...actual,
        useTasks: () => ({
            tasks: computed(() => [
                { id: 'scan', name: 'Library Scan', description: 'desc', schedule: null, lastExecution: null, lastExecutionStatus: 'complete' }
            ]),
            executions: computed(() => [
                { id: 'a', task_name: 'scan', status: 'complete', queued_at: '2026-01-01T09:00:00Z', ended_at: '2026-01-01T09:00:02Z' }
            ]),
            triggeringTaskId: ref(null),
            tasksQuery: { isLoading: ref(false), isError: ref(false), error: ref(null) },
            executionsQuery: { isLoading: ref(false), isError: ref(false), error: ref(null) },
            triggerTask,
            cancelTaskExecution: vi.fn(),
            cancelMutation: { isPending: ref(false) },
            upsertTask: vi.fn(),
            patchTask: vi.fn(),
            deleteTaskSchedule: vi.fn(),
            getStatusSeverity: actual.getStatusSeverity,
            getStatusLabel: actual.getStatusLabel,
            getExecutionLog: vi.fn()
        })
    }
})

import TasksView from '@/views/settings/TasksView.vue'

const mountView = () =>
    mount(TasksView, {
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: { ExecutionHistory: true, LogViewer: true, ScheduleDialog: true }
        }
    })

describe('TasksView', () => {
    it('renders Tasks and Queue tabs', () => {
        const w = mountView()
        expect(w.text()).toContain('Tasks')
        expect(w.text()).toContain('Queue')
    })

    it('lists the task and triggers a run when Run is clicked', async () => {
        const w = mountView()
        expect(w.text()).toContain('Library Scan')
        const runBtn = w.findAll('button').find((b) => b.text().includes('Run'))!
        await runBtn.trigger('click')
        expect(triggerTask).toHaveBeenCalled()
    })
})
