import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'
import type { ViewportTier } from '@/composables/useViewport'
import type { Library } from '@/types/libraries'
import type { User } from '@/types/users'
import type { ExecutionInfo } from '@/types/tasks'

// Mock useViewport with a mutable tier ref so tests can toggle between phone and desktop
const tier = ref<ViewportTier>('desktop')
vi.mock('@/composables/useViewport', () => ({
    useViewport: () => ({ tier }),
    resetViewportForTests: vi.fn()
}))

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))
vi.mock('primevue/useconfirm', () => ({ useConfirm: () => ({ require: vi.fn() }) }))

// ======== LibrariesPanel mocks ========
const libraries = vi.hoisted(() => {
    return { current: [] as Library[] }
})

vi.mock('@/composables/useLibraries', async () => {
    const { ref: vueRef } = await import('vue')
    return {
        useLibraries: () => ({ data: vueRef(libraries.current), isLoading: vueRef(false) }),
        useCreateLibrary: () => ({ mutate: vi.fn(), isPending: vueRef(false) }),
        useUpdateLibrary: () => ({ mutate: vi.fn(), isPending: vueRef(false) }),
        useDeleteLibrary: () => ({ mutate: vi.fn(), isPending: vueRef(false) })
    }
})

// ======== UsersPanel mocks ========
const users = vi.hoisted(() => {
    return { current: [] as User[] }
})

vi.mock('@/composables/useUsers', async () => {
    const { ref: vueRef } = await import('vue')
    return {
        useUsers: () => ({ data: vueRef(users.current), isLoading: vueRef(false) }),
        useCreateUser: () => ({ mutate: vi.fn(), isPending: vueRef(false) }),
        useUpdateUser: () => ({ mutate: vi.fn(), isPending: vueRef(false) }),
        useDeleteUser: () => ({ mutate: vi.fn(), isPending: vueRef(false) })
    }
})

// ======== TasksView mocks ========
vi.mock('@/composables/useTasks', async (importOriginal) => {
    const { ref: vueRef, computed } = await import('vue')
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
            triggeringTaskId: vueRef(null),
            tasksQuery: { isLoading: vueRef(false), isError: vueRef(false), error: vueRef(null) },
            executionsQuery: { isLoading: vueRef(false), isError: vueRef(false), error: vueRef(null) },
            triggerTask: vi.fn(),
            cancelTaskExecution: vi.fn(),
            cancelMutation: { isPending: vueRef(false) },
            upsertTask: vi.fn(),
            patchTask: vi.fn(),
            deleteTaskSchedule: vi.fn(),
            getStatusSeverity: actual.getStatusSeverity,
            getStatusLabel: actual.getStatusLabel,
            getExecutionLog: vi.fn()
        })
    }
})

import LibrariesPanel from '@/components/admin/LibrariesPanel.vue'
import UsersPanel from '@/components/admin/UsersPanel.vue'
import ExecutionHistory from '@/components/admin/ExecutionHistory.vue'
import TasksView from '@/views/settings/TasksView.vue'

function library(over: Partial<Library>): Library {
    return {
        id: 1,
        name: 'Main',
        path: '/srv/music',
        exclude_patterns: [],
        follow_symlinks: true,
        show_artists: true,
        default_view: 'albums',
        icon: 'folder',
        cover_style: 'auto',
        source: 'db',
        last_scan_started_at: '2026-01-01T10:00:00Z',
        created_at: '',
        updated_at: '',
        track_count: 1234,
        ...over
    }
}

function user(over: Partial<User>): User {
    return {
        id: '1',
        login: 'testuser',
        role: 'user',
        enabled: true,
        ...over
    }
}

const execution: ExecutionInfo = {
    id: 'a',
    task_name: 'scan',
    status: 'complete',
    queued_at: '2026-01-01T09:00:00Z',
    started_at: '2026-01-01T09:00:01Z',
    ended_at: '2026-01-01T09:00:04Z'
}

describe('Settings tables hide low-value columns on phones', () => {
    beforeEach(() => {
        tier.value = 'desktop'
    })

    afterEach(() => {
        tier.value = 'desktop'
    })

    describe('LibrariesPanel', () => {
        it('shows Tracks and Last scan columns on desktop', async () => {
            tier.value = 'desktop'
            libraries.current = [library({})]
            const w = mount(LibrariesPanel, {
                global: {
                    plugins: [PrimeVue],
                    directives: { tooltip: {} },
                    stubs: { teleport: true, ConfirmDialog: true, LibraryDialog: true }
                }
            })
            await flushPromises()
            expect(w.text()).toContain('Tracks')
            expect(w.text()).toContain('Last scan')
        })

        it('hides Tracks and Last scan columns on phone', async () => {
            tier.value = 'phone'
            libraries.current = [library({})]
            const w = mount(LibrariesPanel, {
                global: {
                    plugins: [PrimeVue],
                    directives: { tooltip: {} },
                    stubs: { teleport: true, ConfirmDialog: true, LibraryDialog: true }
                }
            })
            await flushPromises()
            expect(w.text()).not.toContain('Tracks')
            expect(w.text()).not.toContain('Last scan')
            // Should still show Name and Path
            expect(w.text()).toContain('Name')
            expect(w.text()).toContain('Path')
        })
    })

    describe('UsersPanel', () => {
        it('shows Status column on desktop', async () => {
            tier.value = 'desktop'
            users.current = [user({})]
            const w = mount(UsersPanel, {
                global: {
                    plugins: [PrimeVue],
                    directives: { tooltip: {} },
                    stubs: { teleport: true, ConfirmDialog: true, UserDialog: true }
                }
            })
            await flushPromises()
            expect(w.text()).toContain('Status')
        })

        it('hides Status column on phone', async () => {
            tier.value = 'phone'
            users.current = [user({})]
            const w = mount(UsersPanel, {
                global: {
                    plugins: [PrimeVue],
                    directives: { tooltip: {} },
                    stubs: { teleport: true, ConfirmDialog: true, UserDialog: true }
                }
            })
            await flushPromises()
            expect(w.text()).not.toContain('Status')
            // Should still show Login and Role
            expect(w.text()).toContain('Login')
            expect(w.text()).toContain('Role')
        })
    })

    describe('ExecutionHistory', () => {
        it('shows Queued and Duration columns on desktop', () => {
            tier.value = 'desktop'
            const w = mount(ExecutionHistory, {
                props: { executions: [execution], isLoading: false },
                global: { plugins: [PrimeVue], directives: { tooltip: {} } }
            })
            expect(w.text()).toContain('Queued')
            expect(w.text()).toContain('Duration')
        })

        it('hides Queued and Duration columns on phone', () => {
            tier.value = 'phone'
            const w = mount(ExecutionHistory, {
                props: { executions: [execution], isLoading: false },
                global: { plugins: [PrimeVue], directives: { tooltip: {} } }
            })
            expect(w.text()).not.toContain('Queued')
            expect(w.text()).not.toContain('Duration')
            // Should still show Task and Status
            expect(w.text()).toContain('Task')
            expect(w.text()).toContain('Status')
        })
    })

    describe('TasksView tasks table', () => {
        it('shows Schedule column and calendar icon on desktop', () => {
            tier.value = 'desktop'
            const w = mount(TasksView, {
                global: {
                    plugins: [PrimeVue],
                    directives: { tooltip: {} },
                    stubs: { ExecutionHistory: true, LogViewer: true, ScheduleDialog: true }
                }
            })
            expect(w.text()).toContain('Schedule')
            expect(w.find('.pi-calendar').exists()).toBe(true)
        })

        it('hides Schedule column and calendar icon column on phone', () => {
            tier.value = 'phone'
            const w = mount(TasksView, {
                global: {
                    plugins: [PrimeVue],
                    directives: { tooltip: {} },
                    stubs: { ExecutionHistory: true, LogViewer: true, ScheduleDialog: true }
                }
            })
            expect(w.text()).not.toContain('Schedule')
            expect(w.find('.pi-calendar').exists()).toBe(false)
            // Should still show Name and Actions
            expect(w.text()).toContain('Name')
            expect(w.text()).toContain('Actions')
        })
    })
})
