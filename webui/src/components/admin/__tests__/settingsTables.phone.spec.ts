import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'
import type { ViewportTier } from '@/composables/useViewport'
import type { Library } from '@/types/libraries'
import type { ScanFolder } from '@/types/scanFolders'
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
    const mutation = () => ({ mutate: vi.fn(), isPending: vueRef(false), error: vueRef(null), reset: vi.fn() })
    return {
        useLibraries: () => ({ data: vueRef(libraries.current), isLoading: vueRef(false) }),
        useCreateLibrary: mutation,
        useUpdateLibrary: mutation,
        useDeleteLibrary: mutation
    }
})

// ======== ScanFoldersPanel mocks ========
const scanFolders = vi.hoisted(() => {
    return { current: [] as ScanFolder[] }
})

vi.mock('@/composables/useScanFolders', async () => {
    const { ref: vueRef } = await import('vue')
    return {
        useScanFolders: () => ({ data: vueRef(scanFolders.current), isLoading: vueRef(false) })
    }
})

// ======== UsersPanel mocks ========
const users = vi.hoisted(() => {
    return { current: [] as User[] }
})

vi.mock('@/composables/useUsers', async () => {
    const { ref: vueRef } = await import('vue')
    const mutation = () => ({ mutate: vi.fn(), isPending: vueRef(false), error: vueRef(null), reset: vi.fn() })
    return {
        useUsers: () => ({ data: vueRef(users.current), isLoading: vueRef(false) }),
        useCreateUser: mutation,
        useUpdateUser: mutation,
        useDeleteUser: mutation
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
                { id: 'scan', name: 'Library Scan', description: 'desc', schedules: [], lastExecution: null, lastExecutionStatus: 'complete' }
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
            createSchedule: vi.fn(),
            patchSchedule: vi.fn(),
            deleteSchedule: vi.fn(),
            getStatusSeverity: actual.getStatusSeverity,
            getStatusLabel: actual.getStatusLabel,
            getExecutionLog: vi.fn()
        })
    }
})

import LibrariesPanel from '@/components/admin/LibrariesPanel.vue'
import ScanFoldersPanel from '@/components/admin/ScanFoldersPanel.vue'
import UsersPanel from '@/components/admin/UsersPanel.vue'
import ExecutionHistory from '@/components/admin/ExecutionHistory.vue'
import TasksView from '@/views/settings/TasksView.vue'

function library(over: Partial<Library>): Library {
    return {
        id: 1,
        name: 'Main',
        show_artists: true,
        default_view: 'albums',
        icon: 'folder',
        filters: [],
        created_at: '',
        updated_at: '',
        track_count: 1234,
        ...over
    }
}

function scanFolder(over: Partial<ScanFolder>): ScanFolder {
    return {
        name: 'Music',
        path: '/mnt/music',
        exclude_patterns: ['*.tmp'],
        follow_symlinks: true,
        available: true,
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

// The scan row now renders a real PrimeVue SplitButton (Run + "Full scan"
// menu item); stub it the same way TasksView.spec.ts does so these
// column-visibility tests don't depend on its overlay/menu internals.
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
        it('shows the Tracks column on desktop', async () => {
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
        })

        it('hides the Tracks column header on phone', async () => {
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
            // Should still show Name header
            expect(w.text()).toContain('Name')
        })

        it('shows the Filters column on desktop', async () => {
            tier.value = 'desktop'
            libraries.current = [library({ filters: [{ field: 'genre', values: ['Rock'] }] })]
            const w = mount(LibrariesPanel, {
                global: {
                    plugins: [PrimeVue],
                    directives: { tooltip: {} },
                    stubs: { teleport: true, ConfirmDialog: true, LibraryDialog: true }
                }
            })
            await flushPromises()
            expect(w.text()).toContain('Filters')
            expect(w.text()).toContain('Genre: Rock')
        })

        it('hides the Filters column header on phone but keeps the summary under the name', async () => {
            tier.value = 'phone'
            libraries.current = [library({ filters: [{ field: 'genre', values: ['Rock'] }] })]
            const w = mount(LibrariesPanel, {
                global: {
                    plugins: [PrimeVue],
                    directives: { tooltip: {} },
                    stubs: { teleport: true, ConfirmDialog: true, LibraryDialog: true }
                }
            })
            await flushPromises()
            expect(w.text()).not.toContain('Filters')
            // The filter summary that would sit in the hidden column still
            // reaches the phone admin, moved under the library's name.
            expect(w.text()).toContain('Genre: Rock')
            expect(w.text()).toContain('Name')
        })
    })

    describe('ScanFoldersPanel', () => {
        it('shows the Path, Excludes and Symlinks columns on desktop', async () => {
            tier.value = 'desktop'
            scanFolders.current = [scanFolder({})]
            const w = mount(ScanFoldersPanel, {
                global: { plugins: [PrimeVue], directives: { tooltip: {} } }
            })
            await flushPromises()
            expect(w.text()).toContain('Path')
            expect(w.text()).toContain('Excludes')
            expect(w.text()).toContain('Symlinks')
            expect(w.text()).toContain('/mnt/music')
        })

        it('hides the Path, Excludes and Symlinks columns on phone but keeps the path under the name', async () => {
            tier.value = 'phone'
            scanFolders.current = [scanFolder({})]
            const w = mount(ScanFoldersPanel, {
                global: { plugins: [PrimeVue], directives: { tooltip: {} } }
            })
            await flushPromises()
            expect(w.text()).not.toContain('Path')
            expect(w.text()).not.toContain('Excludes')
            expect(w.text()).not.toContain('Symlinks')
            // The path that would sit in the hidden column still reaches the
            // phone admin, moved under the folder's name.
            expect(w.text()).toContain('/mnt/music')
            // Tracks and Status are not in the brief's hide list: they stay.
            expect(w.text()).toContain('Tracks')
            expect(w.text()).toContain('Available')
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
                    stubs: { ExecutionHistory: true, LogViewer: true, ScheduleDialog: true, SplitButton: splitButtonStub }
                }
            })
            expect(w.text()).toContain('Schedule')
            expect(w.find('.pi-calendar').exists()).toBe(true)
        })

        it('hides Schedule column but shows the calendar icon on phone', () => {
            tier.value = 'phone'
            const w = mount(TasksView, {
                global: {
                    plugins: [PrimeVue],
                    directives: { tooltip: {} },
                    stubs: { ExecutionHistory: true, LogViewer: true, ScheduleDialog: true, SplitButton: splitButtonStub }
                }
            })
            expect(w.text()).not.toContain('Schedule')
            expect(w.find('.pi-calendar').exists()).toBe(true)
            // Should still show Name and Actions
            expect(w.text()).toContain('Name')
            expect(w.text()).toContain('Actions')
        })
    })
})
