import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
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
        global: { plugins: [PrimeVue], directives: { tooltip: {} } },
        attachTo: document.body
    })

describe('ScheduleDialog', () => {
    it('emits save with the chosen cron when Save is clicked', async () => {
        const w = mountDialog(baseTask)
        const input = document.querySelector('#schedule-cron') as HTMLInputElement
        input.value = '0 0 0 * * *'
        input.dispatchEvent(new Event('input'))
        await w.vm.$nextTick()
        const saveBtn = [...document.querySelectorAll('button')].find((b) => b.textContent?.includes('Save'))!
        saveBtn.click()
        await w.vm.$nextTick()
        expect(w.emitted('save')).toBeTruthy()
        expect(w.emitted('save')![0]).toEqual([{ cron_expression: '0 0 0 * * *', enabled: true }])
    })

    it('does not emit save and shows an error when cron is empty', async () => {
        const w = mountDialog(baseTask)
        const saveBtn = [...document.querySelectorAll('button')].find((b) => b.textContent?.includes('Save'))!
        saveBtn.click()
        await w.vm.$nextTick()
        expect(w.emitted('save')).toBeFalsy()
        expect(document.body.textContent).toContain('Cron expression is required')
    })

    it('shows a Remove schedule button only when the task already has a schedule', () => {
        const without = mountDialog(baseTask)
        expect([...document.querySelectorAll('button')].some((b) => b.textContent?.includes('Remove schedule'))).toBe(false)
        without.unmount()
        const withSched = mountDialog({
            ...baseTask,
            schedule: { id: 1, task_name: 'scan', cron_expression: '0 0 0 * * *', enabled: true, created_at: '', updated_at: '' }
        })
        expect([...document.querySelectorAll('button')].some((b) => b.textContent?.includes('Remove schedule'))).toBe(true)
        withSched.unmount()
    })
})
