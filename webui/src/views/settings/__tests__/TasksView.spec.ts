import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref, computed } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))

const triggerTask = vi.fn()
const createSchedule = vi.fn()
const patchSchedule = vi.fn()
const deleteSchedule = vi.fn()

// A real ref (not a plain variable) so mutating it *after* a component is
// already mounted behaves like a TanStack Query refetch: dependents that
// read `tasks.value` reactively pick up the change without remounting.
// Reset to this single unscheduled task in beforeEach.
const tasksFixtureRef = ref<unknown[]>([
    {
        id: 'scan',
        name: 'Catalog Scan',
        description: 'desc',
        schedules: [],
        lastExecution: null,
        lastExecutionStatus: 'complete'
    }
])

vi.mock('@/composables/useTasks', async (importOriginal) => {
    const actual = await importOriginal<typeof import('@/composables/useTasks')>()
    return {
        ...actual,
        useTasks: () => ({
            tasks: computed(() => tasksFixtureRef.value),
            executions: computed(() => [
                { id: 'a', task_name: 'scan', status: 'complete', queued_at: '2026-01-01T09:00:00Z', ended_at: '2026-01-01T09:00:02Z' }
            ]),
            triggeringTaskId: ref(null),
            tasksQuery: { isLoading: ref(false), isError: ref(false), error: ref(null) },
            executionsQuery: { isLoading: ref(false), isError: ref(false), error: ref(null) },
            triggerTask,
            cancelTaskExecution: vi.fn(),
            cancelMutation: { isPending: ref(false) },
            createSchedule,
            patchSchedule,
            deleteSchedule,
            getStatusSeverity: actual.getStatusSeverity,
            getStatusLabel: actual.getStatusLabel,
            getExecutionLog: vi.fn()
        })
    }
})

import TasksView from '@/views/settings/TasksView.vue'
import ScheduleDialog from '@/components/admin/ScheduleDialog.vue'

// A minimal stand-in for PrimeVue's SplitButton: a primary button forwarding
// the real click event (so the template's `.stop` modifier has an Event to
// call stopPropagation on, matching how the real component forwards it) plus
// one button per menu item invoking that item's `command`.
const splitButtonStub = {
    props: ['label', 'icon', 'model', 'loading', 'disabled'],
    template: `<div class="split-button-stub">
        <button type="button" :disabled="disabled" @click="$emit('click', $event)">{{ label }}</button>
        <button
            v-for="item in model"
            :key="item.label"
            type="button"
            @click="item.command && item.command()"
        >{{ item.label }}</button>
    </div>`
}

const mountView = () =>
    mount(TasksView, {
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: {
                ExecutionHistory: true,
                LogViewer: true,
                ScheduleDialog: true,
                SplitButton: splitButtonStub
            }
        }
    })

beforeEach(() => {
    triggerTask.mockReset()
    createSchedule.mockReset()
    patchSchedule.mockReset()
    deleteSchedule.mockReset()
    tasksFixtureRef.value = [
        {
            id: 'scan',
            name: 'Catalog Scan',
            description: 'desc',
            schedules: [],
            lastExecution: null,
            lastExecutionStatus: 'complete'
        }
    ]
})

describe('TasksView', () => {
    it('renders Tasks and Queue tabs', () => {
        const w = mountView()
        expect(w.text()).toContain('Tasks')
        expect(w.text()).toContain('Queue')
    })

    it('lists the task and triggers a plain run when Run is clicked', async () => {
        const w = mountView()
        expect(w.text()).toContain('Catalog Scan')
        const runBtn = w.findAll('button').find((b) => b.text().includes('Run'))!
        await runBtn.trigger('click')
        expect(triggerTask).toHaveBeenCalledWith(expect.objectContaining({ id: 'scan' }))
    })

    it('shows "Not scheduled" when a task has no schedules', () => {
        const w = mountView()
        expect(w.text()).toContain('Not scheduled')
    })

    it('renders the single schedule\'s preset label, and "(paused)" when it is disabled', () => {
        tasksFixtureRef.value = [
            {
                id: 'scan',
                name: 'Catalog Scan',
                description: 'desc',
                schedules: [
                    {
                        id: 's1',
                        task_name: 'scan',
                        cron_expression: '0 0 0 * * *',
                        enabled: false,
                        created_at: '',
                        updated_at: ''
                    }
                ],
                lastExecution: null,
                lastExecutionStatus: 'complete'
            }
        ]
        const w = mountView()
        expect(w.text()).toContain('Daily (paused)')
    })

    it('summarizes multiple schedules in the Schedule column', () => {
        tasksFixtureRef.value = [
            {
                id: 'scan',
                name: 'Catalog Scan',
                description: 'desc',
                schedules: [
                    {
                        id: 's1',
                        task_name: 'scan',
                        cron_expression: '0 0 0 * * *',
                        enabled: true,
                        created_at: '',
                        updated_at: ''
                    },
                    {
                        id: 's2',
                        task_name: 'scan',
                        cron_expression: '0 0 0 * * 1',
                        enabled: true,
                        created_at: '',
                        updated_at: ''
                    }
                ],
                lastExecution: null,
                lastExecutionStatus: 'complete'
            }
        ]
        const w = mountView()
        expect(w.text()).toContain('2 schedules')
    })

    it('calls createSchedule with the task id when the dialog emits create', async () => {
        const w = mountView()
        await w.find('[aria-label="Schedule"]').trigger('click')
        const body = { cron_expression: '0 0 0 * * *', enabled: true, params: { full: false } }
        w.findComponent(ScheduleDialog).vm.$emit('create', body)
        await flushPromises()
        expect(createSchedule).toHaveBeenCalledWith('scan', body)
    })

    it('calls patchSchedule with the task id and schedule id when the dialog emits patch', async () => {
        const w = mountView()
        await w.find('[aria-label="Schedule"]').trigger('click')
        w.findComponent(ScheduleDialog).vm.$emit('patch', { id: 's1', body: { enabled: false } })
        await flushPromises()
        expect(patchSchedule).toHaveBeenCalledWith('scan', 's1', { enabled: false })
    })

    it('calls deleteSchedule with the task id and schedule id when the dialog emits remove', async () => {
        const w = mountView()
        await w.find('[aria-label="Schedule"]').trigger('click')
        w.findComponent(ScheduleDialog).vm.$emit('remove', 's1')
        await flushPromises()
        expect(deleteSchedule).toHaveBeenCalledWith('scan', 's1')
    })

    it('re-derives the open dialog\'s task reactively so a post-mutation refetch shows the fresh schedules list', async () => {
        const w = mountView()
        await w.find('[aria-label="Schedule"]').trigger('click')

        // Sanity: dialog opened for `scan`, which currently has no schedules.
        expect(w.findComponent(ScheduleDialog).props('task')).toEqual(
            expect.objectContaining({ id: 'scan', schedules: [] })
        )

        // Simulate what actually happens after createSchedule succeeds: the
        // mutation's onSuccess invalidates the tasks query, it refetches, and
        // deriveTasksWithLastExecution rebuilds brand-new Task objects with
        // the new schedule included. The dialog is never closed in between.
        tasksFixtureRef.value = [
            {
                id: 'scan',
                name: 'Catalog Scan',
                description: 'desc',
                schedules: [
                    {
                        id: 's1',
                        task_name: 'scan',
                        cron_expression: '0 0 0 * * *',
                        enabled: true,
                        created_at: '',
                        updated_at: '',
                        params: { full: false }
                    }
                ],
                lastExecution: null,
                lastExecutionStatus: 'complete'
            }
        ]
        await flushPromises()

        const dialogTask = w.findComponent(ScheduleDialog).props('task') as { schedules: unknown[] } | null
        expect(dialogTask?.schedules).toHaveLength(1)
    })

    it('shows the percentage and italic stage line while running', () => {
        tasksFixtureRef.value = [
            {
                id: 'scan',
                name: 'Catalog Scan',
                description: 'desc',
                schedules: [],
                lastExecution: null,
                lastExecutionStatus: 'running',
                lastExecutionProgress: { done: 1, total: 4, stage: 'Extracting metadata: A/B/01.mp3' }
            }
        ]
        const w = mountView()
        expect(w.find('.task-progress').text()).toContain('25% complete')
        expect(w.find('.task-stage').text()).toContain('Extracting metadata: A/B/01.mp3')
    })

    it('shows no stage line when the task is idle', () => {
        const w = mountView()
        expect(w.find('.task-stage').exists()).toBe(false)
    })
})
