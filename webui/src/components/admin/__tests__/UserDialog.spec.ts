import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))

import UserDialog from '@/components/admin/UserDialog.vue'
import type { User, CreateUserInput, UpdateUserInput } from '@/types/users'

const baseUser: User = { id: 'uuid-1', login: 'alice', enabled: true, role: 'user' }

const mountDialog = (user: User | null) =>
    mount(UserDialog, {
        props: { visible: true, user, submitting: false },
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: { teleport: true }
        }
    })

function findButton(w: ReturnType<typeof mountDialog>, label: string) {
    return w.findAll('button').find((b) => b.text().includes(label))!
}

describe('UserDialog create mode', () => {
    it('disables Create until login and password are filled', async () => {
        const w = mountDialog(null)
        await flushPromises()
        expect(findButton(w, 'Create').attributes('disabled')).toBeDefined()

        await w.find('#user-login').setValue('bob')
        await w.find('#user-password').setValue('secret')
        expect(findButton(w, 'Create').attributes('disabled')).toBeUndefined()
    })

    it('emits create with login, password, enabled and the default user role', async () => {
        const w = mountDialog(null)
        await flushPromises()
        await w.find('#user-login').setValue('  bob ')
        await w.find('#user-password').setValue('secret')
        await findButton(w, 'Create').trigger('click')
        await flushPromises()
        const input = w.emitted('create')![0][0] as CreateUserInput
        expect(input).toEqual({ login: 'bob', password: 'secret', enabled: true, role: 'user' })
    })

    it('emits create with role admin when Admin is selected', async () => {
        const w = mountDialog(null)
        await flushPromises()
        await w.find('#user-login').setValue('root')
        await w.find('#user-password').setValue('secret')
        await findButton(w, 'Admin').trigger('click')
        await findButton(w, 'Create').trigger('click')
        await flushPromises()
        const input = w.emitted('create')![0][0] as CreateUserInput
        expect(input.role).toBe('admin')
    })
})

describe('UserDialog edit mode', () => {
    it('addresses the update by id and omits unchanged login, role and empty password', async () => {
        const w = mountDialog(baseUser)
        await flushPromises()

        await findButton(w, 'Save').trigger('click')
        await flushPromises()
        const payload = w.emitted('update')![0][0] as { id: string; input: UpdateUserInput }
        expect(payload.id).toBe('uuid-1')
        expect(payload.input.login).toBeUndefined()
        expect(payload.input.password).toBeUndefined()
        expect(payload.input.role).toBeUndefined()
        expect(payload.input.enabled).toBe(true)
    })

    it('includes a changed role in the update (promotion)', async () => {
        const w = mountDialog(baseUser)
        await flushPromises()
        await findButton(w, 'Admin').trigger('click')
        await findButton(w, 'Save').trigger('click')
        await flushPromises()
        const payload = w.emitted('update')![0][0] as { id: string; input: UpdateUserInput }
        expect(payload.input.role).toBe('admin')
    })

    it('includes a changed login in the update (rename)', async () => {
        const w = mountDialog(baseUser)
        await flushPromises()
        await w.find('#user-login').setValue('alice2')
        await findButton(w, 'Save').trigger('click')
        await flushPromises()
        const payload = w.emitted('update')![0][0] as { id: string; input: UpdateUserInput }
        expect(payload.id).toBe('uuid-1')
        expect(payload.input.login).toBe('alice2')
    })

    it('includes a newly typed password in the update', async () => {
        const w = mountDialog(baseUser)
        await flushPromises()
        await w.find('#user-password').setValue('new-pw')
        await findButton(w, 'Save').trigger('click')
        await flushPromises()
        const payload = w.emitted('update')![0][0] as { id: string; input: UpdateUserInput }
        expect(payload.input.password).toBe('new-pw')
    })
})
