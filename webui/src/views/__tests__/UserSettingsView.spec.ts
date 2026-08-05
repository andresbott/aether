import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref, type Ref } from 'vue'
import PrimeVue from 'primevue/config'
import type { MeUser } from '@/types/users'

const currentUser: Ref<MeUser | null> = ref(null)
vi.mock('@/composables/useAuth', () => ({
    useAuth: () => ({ currentUser })
}))

import UserSettingsView from '@/views/UserSettingsView.vue'
import { useTheme } from '@/composables/useTheme'

const THEME_CLASSES = ['dark-mode', 'theme-winamp', 'theme-crt']

const mountView = () => mount(UserSettingsView, { global: { plugins: [PrimeVue] } })

// useTheme is a module singleton shared with the rest of the suite, so put the
// mode back where it started rather than leaking a hidden theme.
beforeEach(() => {
    useTheme().mode.value = 'auto'
})

afterEach(() => {
    useTheme().mode.value = 'auto'
    document.documentElement.classList.remove(...THEME_CLASSES)
})

describe('UserSettingsView', () => {
    it('renders as a main content view with the scaffold title', () => {
        const w = mountView()
        expect(w.find('.content-scaffold').exists()).toBe(true)
        expect(w.find('.scaffold-title h1').text()).toBe('User settings')
    })

    it('shows the signed-in identity when a session exists', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        const w = mountView()
        expect(w.text()).toContain('Signed in as alice')
        currentUser.value = null
    })

    it('explains the no-login setup when anonymous', () => {
        const w = mountView()
        expect(w.text()).toContain('requires no login')
    })

    it('offers only the three standard themes by default', () => {
        const labels = mountView()
            .findAll('.p-selectbutton .p-togglebutton-label')
            .map((b) => b.text())
        expect(labels).toEqual(['Auto', 'Light', 'Dark'])
    })

    // The shortcuts reference moved to the About view.
    it('has no keyboard shortcuts section', () => {
        expect(mountView().text()).not.toContain('Keyboard shortcuts')
    })

    it('lists the hidden themes once they are unlocked', () => {
        useTheme().unlockHiddenThemes()
        const w = mountView()
        const labels = w
            .findAll('.p-selectbutton .p-togglebutton-label')
            .map((b) => b.text())
        expect(labels).toEqual(['Auto', 'Light', 'Dark', 'Winamp', 'CRT'])
        expect(w.text()).toContain('Nice find')
    })
})
