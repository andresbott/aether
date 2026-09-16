import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import type { CreateUserInput, UpdateUserInput } from '@/types/users'

const createUserMock = vi.fn()
const updateUserMock = vi.fn()

vi.mock('@/lib/api/Users', () => ({
    createUser: (...args: unknown[]) => createUserMock(...args),
    updateUser: (...args: unknown[]) => updateUserMock(...args),
    listUsers: vi.fn(),
    deleteUser: vi.fn(),
    getMe: vi.fn()
}))

const toastAdd = vi.hoisted(() => vi.fn())
vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: toastAdd }) }))

import { useCreateUser, useUpdateUser } from '@/composables/useUsers'

const createInput: CreateUserInput = { login: 'bob', password: 'secret', enabled: true, role: 'user' }
const updateInput: UpdateUserInput = { enabled: true }

/** Mounts a mutation composable inside a real vue-query context and returns it. */
function mountMutation<T>(composable: () => T) {
    const queryClient = new QueryClient()
    let mutation!: T
    const Comp = defineComponent({
        setup() {
            mutation = composable()
            return () => h('div')
        }
    })
    mount(Comp, { global: { plugins: [[VueQueryPlugin, { queryClient }]] } })
    return { mutation }
}

beforeEach(() => {
    createUserMock.mockReset()
    updateUserMock.mockReset()
    toastAdd.mockReset()
})

// A 422 names the offending field in errors[]; the dialog renders that inline,
// so the composable must not also toast it. Other failures still toast.
describe('validation errors are left for the form, not toasted', () => {
    const fieldProblem = {
        response: {
            status: 422,
            data: { detail: 'login already exists', errors: [{ pointer: '/login', detail: 'login already exists' }] }
        }
    }

    it('useCreateUser does not toast a field-validation error', async () => {
        createUserMock.mockRejectedValue(fieldProblem)
        const { mutation } = mountMutation(useCreateUser)
        await mutation.mutateAsync(createInput).catch(() => {})
        expect(toastAdd).not.toHaveBeenCalled()
    })

    it('useUpdateUser does not toast a field-validation error', async () => {
        updateUserMock.mockRejectedValue(fieldProblem)
        const { mutation } = mountMutation(useUpdateUser)
        await mutation.mutateAsync({ id: 'uuid-1', input: updateInput }).catch(() => {})
        expect(toastAdd).not.toHaveBeenCalled()
    })

    it('still toasts a non-field failure such as a 500', async () => {
        createUserMock.mockRejectedValue({ response: { status: 500, data: { detail: 'boom' } } })
        const { mutation } = mountMutation(useCreateUser)
        await mutation.mutateAsync(createInput).catch(() => {})
        expect(toastAdd).toHaveBeenCalledWith(expect.objectContaining({ severity: 'error' }))
    })
})
