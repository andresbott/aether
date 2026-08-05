import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref, nextTick } from 'vue'
import PrimeVue from 'primevue/config'
import { sessionExpired } from '@/lib/authState'

const isPending = ref(false)
const mutateAsync = vi.fn()
vi.mock('@/composables/useAuth', () => ({
    useAuth: () => ({ login: { isPending, mutateAsync } })
}))

const routerReplace = vi.fn()
vi.mock('vue-router', () => ({
    useRouter: () => ({ replace: routerReplace })
}))

import LoginView from '@/views/LoginView.vue'

const mountView = () =>
    mount(LoginView, {
        global: { plugins: [PrimeVue] },
        attachTo: document.body
    })

async function fillAndSubmit(w: ReturnType<typeof mountView>, user: string, pw: string) {
    await w.find('#login-username').setValue(user)
    await w.find('#login-password').setValue(pw)
    await w.find('form').trigger('submit')
    await nextTick()
}

describe('LoginView', () => {
    beforeEach(() => {
        sessionExpired.value = false
        isPending.value = false
        mutateAsync.mockReset().mockResolvedValue({ done: true })
        routerReplace.mockReset()
    })

    it('disables the submit button until both fields are filled', async () => {
        const w = mountView()
        const submit = w.find('button[type="submit"]')
        expect(submit.attributes('disabled')).toBeDefined()
        await w.find('#login-username').setValue('alice')
        expect(submit.attributes('disabled')).toBeDefined()
        await w.find('#login-password').setValue('secret')
        expect(submit.attributes('disabled')).toBeUndefined()
    })

    it('submits trimmed username, password and the remember-me flag', async () => {
        const w = mountView()
        await w.find('#login-remember').setValue(true)
        await fillAndSubmit(w, '  alice  ', 'secret')
        expect(mutateAsync).toHaveBeenCalledWith({
            username: 'alice',
            password: 'secret',
            rememberMe: true
        })
    })

    // A fresh session always starts on the landing page — the URL the browser
    // sat on while logged out is deliberately not restored.
    it('redirects to the home page after a successful login', async () => {
        const w = mountView()
        await fillAndSubmit(w, 'alice', 'secret')
        await nextTick()
        expect(routerReplace).toHaveBeenCalledWith('/')
    })

    it('does not navigate when the login fails', async () => {
        mutateAsync.mockRejectedValue({ response: { status: 401 } })
        const w = mountView()
        await fillAndSubmit(w, 'alice', 'wrong')
        await nextTick()
        expect(routerReplace).not.toHaveBeenCalled()
    })

    it('defaults remember-me to off', async () => {
        const w = mountView()
        await fillAndSubmit(w, 'alice', 'secret')
        expect(mutateAsync).toHaveBeenCalledWith(
            expect.objectContaining({ rememberMe: false })
        )
    })

    // The server answers one uniform 401 for every credential-shaped failure,
    // so the view shows one uniform message and clears the password.
    it('shows a credentials error on 401 and clears the password', async () => {
        mutateAsync.mockRejectedValue({ response: { status: 401 } })
        const w = mountView()
        await fillAndSubmit(w, 'alice', 'wrong')
        await nextTick()
        expect(w.find('.login-error').text()).toBe('Wrong username or password.')
        expect((w.find('#login-password').element as HTMLInputElement).value).toBe('')
    })

    it('explains a session expiry when the flag is set', () => {
        sessionExpired.value = true
        expect(mountView().find('.login-expired').text()).toContain('session has expired')
    })

    it('shows no expiry note on a fresh visit', () => {
        expect(mountView().find('.login-expired').exists()).toBe(false)
    })
})
