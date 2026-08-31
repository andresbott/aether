import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { reactive, ref, type Ref } from 'vue'
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'
import ConfirmationService from 'primevue/confirmationservice'
import type { MeUser } from '@/types/users'

// A reactive route mock: replace() writes the new tab back onto route.params,
// so activeTab (a path-segment computed) actually switches panels on click.
const route = reactive({
    params: {} as Record<string, string>,
    query: {} as Record<string, string>
})
const replace = vi.fn((to: { path: string; query: Record<string, string> }) => {
    const tab = to.path.replace(/^\/user-settings\/?/, '')
    route.params = tab ? { tab } : {}
    route.query = to.query
})
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ replace })
}))

const currentUser: Ref<MeUser | null> = ref(null)
const authRequired = ref(false)
const logoutMutate = vi.fn()
vi.mock('@/composables/useAuth', () => ({
    useAuth: () => ({ currentUser, authRequired, logout: { mutate: logoutMutate } })
}))

vi.mock('@/composables/useTokens', () => ({
    useTokens: () => ({ data: ref([]) }),
    useCreateToken: () => ({ mutate: vi.fn(), isPending: ref(false) }),
    useRevokeToken: () => ({ mutate: vi.fn(), isPending: ref(false), variables: ref(undefined) })
}))

const changePassword = vi.fn()
vi.mock('@/lib/api/Auth', () => ({
    changePassword: (...args: unknown[]) => changePassword(...args)
}))

import UserSettingsView from '@/views/UserSettingsView.vue'

const mountView = () =>
    mount(UserSettingsView, {
        global: {
            plugins: [PrimeVue, ToastService, ConfirmationService],
            directives: { tooltip: {} },
            stubs: { teleport: true }
        },
        attachTo: document.body
    })

beforeEach(() => {
    route.params = {}
    route.query = {}
    currentUser.value = null
    authRequired.value = false
    changePassword.mockReset()
    logoutMutate.mockReset()
    replace.mockClear()
})

// Fill and submit the change-password form; returns the wrapper.
async function submitForm(
    w: ReturnType<typeof mountView>,
    current: string,
    next: string,
    confirm: string
) {
    await w.find('#current-password').setValue(current)
    await w.find('#new-password').setValue(next)
    await w.find('#confirm-password').setValue(confirm)
    await w.find('.change-password-form').trigger('submit')
    await w.vm.$nextTick()
}

describe('UserSettingsView change password', () => {
    it('shows an Account tab hosting the change-password form for a native user', () => {
        currentUser.value = { login: 'bob', role: 'user' }
        authRequired.value = true
        const w = mountView()
        const tabs = w.findAll('[role="tab"]').map((t) => t.text())
        expect(tabs).toContain('Account')
        // The form lives in the Account panel.
        expect(w.find('#panel-account .change-password-form').exists()).toBe(true)
    })

    it('switches to the Account panel when its tab is clicked', async () => {
        currentUser.value = { login: 'bob', role: 'user' }
        authRequired.value = true
        const w = mountView()
        const accountTab = w.findAll('[role="tab"]').find((t) => t.text() === 'Account')!
        await accountTab.trigger('click')
        const shown = w
            .findAll('[role="tabpanel"]')
            .filter((p) => (p.element as HTMLElement).style.display !== 'none')
            .map((p) => p.attributes('id'))
        expect(shown).toEqual(['panel-account'])
    })

    it('hides the Account tab and form in proxy-header mode (IdP owns credentials)', () => {
        // A signed-in user, but native login is not required: the credential
        // lives at the proxy, so there is nothing here to change.
        currentUser.value = { login: 'bob', role: 'user' }
        authRequired.value = false
        const w = mountView()
        expect(w.findAll('[role="tab"]').map((t) => t.text())).not.toContain('Account')
        expect(w.find('.change-password-form').exists()).toBe(false)
    })

    it('hides the Account tab and form when anonymous (auth method none)', () => {
        currentUser.value = null
        authRequired.value = false
        const w = mountView()
        expect(w.findAll('[role="tab"]').map((t) => t.text())).not.toContain('Account')
        expect(w.find('.change-password-form').exists()).toBe(false)
    })

    it('refuses to submit when the new password and confirmation differ', async () => {
        currentUser.value = { login: 'bob', role: 'user' }
        authRequired.value = true
        const w = mountView()
        await submitForm(w, 'secret', 'new-pw-one', 'new-pw-two')
        expect(changePassword).not.toHaveBeenCalled()
        expect(w.find('.change-password-form').text().toLowerCase()).toContain('match')
    })

    it('submits the current and new password, then signs the user out', async () => {
        currentUser.value = { login: 'bob', role: 'user' }
        authRequired.value = true
        changePassword.mockResolvedValue(undefined)
        const w = mountView()
        await submitForm(w, 'secret', 'brand-new-pw', 'brand-new-pw')
        expect(changePassword).toHaveBeenCalledWith('secret', 'brand-new-pw')
        await w.vm.$nextTick()
        // A successful change logs the user out so they re-authenticate.
        expect(logoutMutate).toHaveBeenCalled()
    })

    it('does not sign the user out when the current password is wrong', async () => {
        currentUser.value = { login: 'bob', role: 'user' }
        authRequired.value = true
        changePassword.mockRejectedValue({
            response: { status: 403, data: { detail: 'current password is incorrect' } }
        })
        const w = mountView()
        await submitForm(w, 'wrong', 'brand-new-pw', 'brand-new-pw')
        await w.vm.$nextTick()
        expect(logoutMutate).not.toHaveBeenCalled()
    })

    // The server answers a wrong current password with 403 (not 401), so the
    // form shows an error rather than the client treating it as a lost session.
    it('surfaces the server error when the current password is wrong', async () => {
        currentUser.value = { login: 'bob', role: 'user' }
        authRequired.value = true
        changePassword.mockRejectedValue({
            response: { status: 403, data: { detail: 'current password is incorrect' } }
        })
        const w = mountView()
        await submitForm(w, 'wrong', 'brand-new-pw', 'brand-new-pw')
        await w.vm.$nextTick()
        expect(w.find('.change-password-form').text()).toContain('current password is incorrect')
    })
})
