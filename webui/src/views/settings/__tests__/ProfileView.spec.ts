import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import ProfileView from '@/views/settings/ProfileView.vue'
import { useTheme } from '@/composables/useTheme'

const THEME_CLASSES = ['dark-mode', 'theme-winamp', 'theme-crt']

// useTheme is a module singleton shared with the rest of the suite, so put the
// mode back where it started rather than leaking a hidden theme.
beforeEach(() => {
    useTheme().mode.value = 'auto'
})

afterEach(() => {
    useTheme().mode.value = 'auto'
    document.documentElement.classList.remove(...THEME_CLASSES)
})

describe('ProfileView', () => {
    it('renders the profile placeholder', () => {
        const w = mount(ProfileView)
        expect(w.text()).toContain('Profile')
        expect(w.text().toLowerCase()).toContain('placeholder')
    })

    it('offers only the three standard themes by default', () => {
        const labels = mount(ProfileView)
            .findAll('.p-selectbutton .p-togglebutton-label')
            .map((b) => b.text())
        expect(labels).toEqual(['Auto', 'Light', 'Dark'])
    })

    it('lists the hidden themes once they are unlocked', () => {
        useTheme().unlockHiddenThemes()
        const w = mount(ProfileView)
        const labels = w
            .findAll('.p-selectbutton .p-togglebutton-label')
            .map((b) => b.text())
        expect(labels).toEqual(['Auto', 'Light', 'Dark', 'Winamp', 'CRT'])
        expect(w.text()).toContain('Nice find')
    })
})
