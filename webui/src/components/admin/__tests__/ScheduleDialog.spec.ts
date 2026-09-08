import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))

import ScheduleDialog from '@/components/admin/ScheduleDialog.vue'
import type { Task } from '@/composables/useTasks'
import type { TaskSchedule } from '@/types/tasks'

const stubs = {
    teleport: true,
    Checkbox: {
        props: ['modelValue', 'binary', 'inputId'],
        template:
            '<input type="checkbox" :id="inputId" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)" />'
    }
}

const scanTaskNoSchedules: Task = {
    id: 'scan',
    name: 'Scan',
    description: '',
    schedules: [],
    lastExecution: null,
    lastExecutionStatus: null
}

const twoSchedules: TaskSchedule[] = [
    {
        id: 's1',
        task_name: 'scan',
        cron_expression: '0 0 0 * * *',
        enabled: true,
        created_at: '',
        updated_at: '',
        params: { full: false }
    },
    {
        id: 's2',
        task_name: 'scan',
        cron_expression: '0 0 0 * * 1',
        enabled: false,
        created_at: '',
        updated_at: '',
        params: { full: true }
    }
]

const scanTaskTwoSchedules: Task = {
    ...scanTaskNoSchedules,
    schedules: twoSchedules
}

const otherTask: Task = {
    id: 'fetch-artist-images',
    name: 'Fetch artist images',
    description: '',
    schedules: [],
    lastExecution: null,
    lastExecutionStatus: null
}

const mountDialog = (task: Task | null) =>
    mount(ScheduleDialog, {
        props: { visible: true, task },
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs
        }
    })

describe('ScheduleDialog', () => {
    it('lists one row per existing schedule, flagging paused ones', async () => {
        const w = mountDialog(scanTaskTwoSchedules)
        await flushPromises()
        expect(w.findAll('.schedule-row')).toHaveLength(2)
        expect(w.text()).toContain('(paused)')
    })

    it('shows the Full/Incremental control for the scan task', async () => {
        const w = mountDialog(scanTaskTwoSchedules)
        await flushPromises()
        expect(w.find('#schedule-full').exists()).toBe(true)
        expect(w.text()).toContain('Full')
        expect(w.text()).toContain('Incremental')
    })

    it('does not show the Full/Incremental control for a non-scan task', async () => {
        const w = mountDialog(otherTask)
        await flushPromises()
        expect(w.find('#schedule-full').exists()).toBe(false)
    })

    it('emits create with cron_expression, enabled and params.full when the add form is saved', async () => {
        const w = mountDialog(scanTaskNoSchedules)
        await flushPromises()
        await w.find('#schedule-cron').setValue('0 0 0 * * *')
        await w.find('#schedule-full').setValue(true)
        const addBtn = w.findAll('button').find((b) => b.text().includes('Add schedule'))!
        await addBtn.trigger('click')
        await flushPromises()
        expect(w.emitted('create')).toBeTruthy()
        expect(w.emitted('create')![0]).toEqual([
            { cron_expression: '0 0 0 * * *', enabled: true, params: { full: true } }
        ])
    })

    it('does not emit create and shows an error when cron is empty', async () => {
        const w = mountDialog(scanTaskNoSchedules)
        await flushPromises()
        const addBtn = w.findAll('button').find((b) => b.text().includes('Add schedule'))!
        await addBtn.trigger('click')
        await flushPromises()
        expect(w.emitted('create')).toBeFalsy()
        expect(w.text()).toContain('Cron expression is required')
    })

    it('does not send params for a non-scan task', async () => {
        const w = mountDialog(otherTask)
        await flushPromises()
        await w.find('#schedule-cron').setValue('0 0 0 * * *')
        const addBtn = w.findAll('button').find((b) => b.text().includes('Add schedule'))!
        await addBtn.trigger('click')
        await flushPromises()
        expect(w.emitted('create')![0]).toEqual([
            { cron_expression: '0 0 0 * * *', enabled: true, params: undefined }
        ])
    })

    it('emits patch with the id and body when an existing schedule is edited and saved', async () => {
        const w = mountDialog(scanTaskTwoSchedules)
        await flushPromises()
        const editBtns = w.findAll('[aria-label="Edit schedule"]')
        expect(editBtns).toHaveLength(2)
        await editBtns[0].trigger('click')
        await flushPromises()
        await w.find('#schedule-cron').setValue('0 0 0 * * 2')
        const saveBtn = w.findAll('button').find((b) => b.text().includes('Save changes'))!
        await saveBtn.trigger('click')
        await flushPromises()
        expect(w.emitted('patch')).toBeTruthy()
        expect(w.emitted('patch')![0]).toEqual([
            { id: 's1', body: { cron_expression: '0 0 0 * * 2', enabled: true, params: { full: false } } }
        ])
    })

    it('emits remove with the schedule id when its remove button is clicked', async () => {
        const w = mountDialog(scanTaskTwoSchedules)
        await flushPromises()
        const removeBtns = w.findAll('[aria-label="Remove schedule"]')
        expect(removeBtns).toHaveLength(2)
        await removeBtns[1].trigger('click')
        expect(w.emitted('remove')).toBeTruthy()
        expect(w.emitted('remove')![0]).toEqual(['s2'])
    })
})
