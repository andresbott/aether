import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import type { User } from '@/types/users'

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))
vi.mock('primevue/useconfirm', () => ({ useConfirm: () => ({ require: vi.fn() }) }))

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const users = vi.hoisted(() => ({ current: [] as any[] }))
// Shared mutation doubles so a test can drive .error and assert .reset.
const mutations = vi.hoisted(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return { create: null as any, update: null as any }
})

vi.mock('@/composables/useUsers', async () => {
    const { ref: vueRef } = await import('vue')
    mutations.create = { mutate: vi.fn(), isPending: vueRef(false), error: vueRef(null), reset: vi.fn() }
    mutations.update = { mutate: vi.fn(), isPending: vueRef(false), error: vueRef(null), reset: vi.fn() }
    return {
        useUsers: () => ({ data: vueRef(users.current), isLoading: vueRef(false) }),
        useCreateUser: () => mutations.create,
        useUpdateUser: () => mutations.update,
        useDeleteUser: () => ({ mutate: vi.fn(), isPending: vueRef(false), error: vueRef(null), reset: vi.fn() })
    }
})

import UsersPanel from '@/components/admin/UsersPanel.vue'
import UserDialog from '@/components/admin/UserDialog.vue'

beforeEach(() => {
    mutations.create.error.value = null
    mutations.update.error.value = null
    mutations.create.reset.mockClear()
    mutations.update.reset.mockClear()
})

const mountPanel = (list: User[]) => {
    users.current = list
    return mount(UsersPanel, {
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: { teleport: true, ConfirmDialog: true, UserDialog: true }
        }
    })
}

// A failed create/update leaves its error on the mutation; the panel forwards
// that to the dialog so the field-level message shows on the offending input,
// and clears any stale error when the dialog is (re)opened.
describe('UsersPanel forwards validation errors to the dialog', () => {
    const addBtn = (w: ReturnType<typeof mountPanel>) =>
        w.findAll('button').find((b) => b.text().includes('Add user'))!

    it('passes the create mutation error down to the dialog', async () => {
        const w = mountPanel([])
        await addBtn(w).trigger('click')
        mutations.create.error.value = {
            response: { status: 422, data: { errors: [{ pointer: '/login', detail: 'login already exists' }] } }
        }
        await flushPromises()
        expect(w.findComponent(UserDialog).props('error')).toBe(mutations.create.error.value)
    })

    it('resets both mutations when opening the dialog so no stale error shows', async () => {
        const w = mountPanel([])
        await addBtn(w).trigger('click')
        expect(mutations.create.reset).toHaveBeenCalled()
        expect(mutations.update.reset).toHaveBeenCalled()
    })
})
