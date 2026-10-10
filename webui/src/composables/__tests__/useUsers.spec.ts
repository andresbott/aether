import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient, onlineManager } from '@tanstack/vue-query'
import type { CreateUserInput, UpdateUserInput } from '@/types/users'

const createUserMock = vi.fn()
const updateUserMock = vi.fn()
const getMeMock = vi.fn()

vi.mock('@/lib/api/Users', () => ({
    createUser: (...args: unknown[]) => createUserMock(...args),
    updateUser: (...args: unknown[]) => updateUserMock(...args),
    listUsers: vi.fn(),
    deleteUser: vi.fn(),
    getMe: (...args: unknown[]) => getMeMock(...args)
}))

const toastAdd = vi.hoisted(() => vi.fn())
vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: toastAdd }) }))

// A browser tab unless a case moves into the desktop app, which has a server.
const host = vi.hoisted(() => ({ desktopApp: false }))
vi.mock('@/lib/host', async () => {
    const { ref } = await import('vue')
    return {
        isDesktopApp: () => host.desktopApp,
        serverUrl: ref<string | null>('http://aether.local:8075')
    }
})

import { useCreateUser, useMe, useUpdateUser } from '@/composables/useUsers'

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

/** Mounts useMe in a vue-query context set up the way main.ts sets it up. */
function mountMe() {
    // main.ts retries every query three times; without the delay here the
    // browser's retries run out at once.
    const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: 3, retryDelay: 0 } }
    })
    let me!: ReturnType<typeof useMe>
    const Comp = defineComponent({
        setup() {
            me = useMe()
            return () => h('div')
        }
    })
    mount(Comp, { global: { plugins: [[VueQueryPlugin, { queryClient }]] } })
    return me
}

describe('the /me bootstrap', () => {
    beforeEach(() => {
        getMeMock.mockReset()
        getMeMock.mockRejectedValue(new Error('Network Error'))
        host.desktopApp = false
    })

    // In the app a failed /me brings the server step up, and Connect is the
    // retry. Three more attempts, each up to getMe's 10 s timeout, would hold
    // the window blank for most of a minute on an address that drops packets.
    it('asks a failed /me only once in the desktop app', async () => {
        host.desktopApp = true

        const me = mountMe()
        await flushPromises()

        expect(me.isError.value).toBe(true)
        expect(getMeMock).toHaveBeenCalledTimes(1)
    })

    it('keeps retrying a failed /me in the browser', async () => {
        const me = mountMe()

        await vi.waitFor(() => expect(me.isError.value).toBe(true))
        expect(getMeMock).toHaveBeenCalledTimes(4)
    })

    // The server step is up on the errored /me. Asked again when the network
    // comes back, /me would go pending and take the step away mid-typing.
    it('does not ask a failed /me again when the network comes back in the app', async () => {
        host.desktopApp = true
        const me = mountMe()
        await flushPromises()

        onlineManager.setOnline(false)
        onlineManager.setOnline(true)
        await flushPromises()

        expect(getMeMock).toHaveBeenCalledTimes(1)
        expect(me.isError.value).toBe(true)
    })

    // A tab opened offline shows nothing on its failed /me; the network coming
    // back is what brings the app up there.
    it('asks a failed /me again when the network comes back in the browser', async () => {
        const me = mountMe()
        await vi.waitFor(() => expect(me.isError.value).toBe(true))
        getMeMock.mockReturnValue(new Promise(() => {}))

        onlineManager.setOnline(false)
        onlineManager.setOnline(true)
        await flushPromises()

        expect(getMeMock).toHaveBeenCalledTimes(5)
    })
})
