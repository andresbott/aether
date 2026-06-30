import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))

import ScheduleDialog from '@/components/admin/ScheduleDialog.vue'
import type { Task } from '@/composables/useTasks'

const baseTask: Task = {
    id: 'scan', name: 'Scan', description: '',
    schedule: null, lastExecution: null, lastExecutionStatus: null
}

const mountDialog = (task: Task | null) =>
    mount(ScheduleDialog, {
        props: { visible: true, task },
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: { teleport: true }
        }
    })

describe('ScheduleDialog', () => {
    it('emits save with the chosen cron when Save is clicked', async () => {
        const w = mountDialog(baseTask)
        await flushPromises()
        const input = w.find('#schedule-cron')
        await input.setValue('0 0 0 * * *')
        const saveBtn = w.findAll('button').find((b) => b.text().includes('Save'))!
        await saveBtn.trigger('click')
        await flushPromises()
        expect(w.emitted('save')).toBeTruthy()
        expect(w.emitted('save')![0]).toEqual([{ cron_expression: '0 0 0 * * *', enabled: true }])
    })

    it('does not emit save and shows an error when cron is empty', async () => {
        const w = mountDialog(baseTask)
        await flushPromises()
        const saveBtn = w.findAll('button').find((b) => b.text().includes('Save'))!
        await saveBtn.trigger('click')
        await flushPromises()
        expect(w.emitted('save')).toBeFalsy()
        expect(w.text()).toContain('Cron expression is required')
    })

    it('shows a Remove schedule button only when the task already has a schedule', async () => {
        const without = mountDialog(baseTask)
        await flushPromises()
        expect(without.findAll('button').some((b) => b.text().includes('Remove schedule'))).toBe(false)
        without.unmount()

        const withSched = mountDialog({
            ...baseTask,
            schedule: { id: 1, task_name: 'scan', cron_expression: '0 0 0 * * *', enabled: true, created_at: '', updated_at: '' }
        })
        await flushPromises()
        expect(withSched.findAll('button').some((b) => b.text().includes('Remove schedule'))).toBe(true)
        withSched.unmount()
    })
})
