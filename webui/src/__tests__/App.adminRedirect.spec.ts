import { describe, it, expect, vi, beforeEach } from 'vitest'
import { reactive, ref, nextTick } from 'vue'
import { mount } from '@vue/test-utils'

// The redirect under test reads the route's meta and the /me verdict, so both
// layouts and the login view are irrelevant here — stub them out.
vi.mock('@/layouts/PlayerLayout.vue', () => ({
    default: { name: 'PlayerLayout', template: '<div class="player-layout" />' }
}))
vi.mock('@/layouts/SettingsLayout.vue', () => ({
    default: { name: 'SettingsLayout', template: '<div class="settings-layout" />' }
}))
vi.mock('@/views/LoginView.vue', () => ({
    default: { name: 'LoginView', template: '<div class="login-view" />' }
}))

const route = reactive<{ meta: { layout?: string } }>({ meta: {} })
const replace = vi.fn()
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ replace })
}))

const isLoading = ref(false)
const needsLogin = ref(false)
const isAdmin = ref(false)
vi.mock('@/composables/useAuth', () => ({
    useAuth: () => ({ isLoading, needsLogin, isAdmin })
}))

import App from '@/App.vue'

beforeEach(() => {
    route.meta = {}
    isLoading.value = false
    needsLogin.value = false
    isAdmin.value = false
    replace.mockClear()
})

// The /settings area is admin-only. Nothing in a non-admin's UI links there,
// so this only fires on a typed URL — the backend 403s the data either way.
describe('App admin redirect', () => {
    it('sends a non-admin landing on a settings route home', async () => {
        route.meta = { layout: 'settings' }
        mount(App)
        await nextTick()
        expect(replace).toHaveBeenCalledWith('/')
    })

    it('leaves admins on settings routes alone', async () => {
        route.meta = { layout: 'settings' }
        isAdmin.value = true
        mount(App)
        await nextTick()
        expect(replace).not.toHaveBeenCalled()
    })

    it('does not redirect while /me is still loading', async () => {
        route.meta = { layout: 'settings' }
        isLoading.value = true
        mount(App)
        await nextTick()
        expect(replace).not.toHaveBeenCalled()
    })

    it('lets the login gate handle anonymous visitors instead of redirecting', async () => {
        route.meta = { layout: 'settings' }
        needsLogin.value = true
        const w = mount(App)
        await nextTick()
        expect(replace).not.toHaveBeenCalled()
        expect(w.find('.login-view').exists()).toBe(true)
    })

    it('redirects when the admin verdict arrives after the route change', async () => {
        route.meta = { layout: 'settings' }
        isLoading.value = true
        mount(App)
        await nextTick()
        expect(replace).not.toHaveBeenCalled()

        isLoading.value = false
        await nextTick()
        expect(replace).toHaveBeenCalledWith('/')
    })

    it('never redirects non-admins on ordinary routes', async () => {
        mount(App)
        await nextTick()
        expect(replace).not.toHaveBeenCalled()
    })
})
