import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref, type Ref } from 'vue'
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'
import type { MeUser } from '@/types/users'
import type { ApiToken } from '@/types/tokens'

const currentUser: Ref<MeUser | null> = ref(null)
vi.mock('@/composables/useAuth', () => ({
    useAuth: () => ({ currentUser })
}))

const mockTokens: Ref<ApiToken[]> = ref([])
const mockCreateToken = vi.fn()
const mockRevokeToken = vi.fn()
vi.mock('@/composables/useTokens', () => ({
    useTokens: () => ({ data: mockTokens }),
    useCreateToken: () => ({
        mutate: mockCreateToken,
        isPending: ref(false)
    }),
    useRevokeToken: () => ({
        mutate: mockRevokeToken,
        isPending: ref(false),
        variables: ref(undefined)
    })
}))

import UserSettingsView from '@/views/UserSettingsView.vue'
import { useTheme } from '@/composables/useTheme'

const THEME_CLASSES = ['dark-mode', 'theme-winamp', 'theme-crt']

const mountView = () =>
    mount(UserSettingsView, {
        global: {
            plugins: [PrimeVue, ToastService],
            stubs: {
                teleport: true
            }
        },
        attachTo: document.body
    })

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

    it('shows the API tokens section for a logged-in native-mode user', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        const w = mountView()
        expect(w.find('.tokens-section').exists()).toBe(true)
        currentUser.value = null
    })

    it('hides the API tokens section with auth method none', () => {
        currentUser.value = null
        const w = mountView()
        expect(w.find('.tokens-section').exists()).toBe(false)
    })

    it('lists tokens with name and revoke affordance', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        mockTokens.value = [
            {
                tokenId: 't1',
                name: 'Symfonium',
                kind: 'client',
                createdAt: '2026-01-01T00:00:00Z'
            }
        ]
        const w = mountView()
        expect(w.text()).toContain('Symfonium')
        expect(w.find('.token-revoke').exists()).toBe(true)
        currentUser.value = null
        mockTokens.value = []
    })

    it('separates session and third-party tokens into their own groups', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        mockTokens.value = [
            {
                tokenId: 't1',
                name: 'aether-web',
                kind: 'session',
                createdAt: '2026-01-01T00:00:00Z'
            },
            {
                tokenId: 't2',
                name: 'Symfonium',
                kind: 'client',
                createdAt: '2026-01-01T00:00:00Z'
            }
        ]
        const w = mountView()
        const groups = w.findAll('.token-group')
        expect(groups).toHaveLength(2)
        expect(groups[0].find('h3').text()).toBe('Aether sessions')
        expect(groups[0].find('.token-count').text()).toContain('1 active Aether session')
        expect(groups[0].text()).toContain('aether-web')
        expect(groups[0].text()).not.toContain('Symfonium')
        expect(groups[1].find('h3').text()).toBe('Third-party tokens')
        expect(groups[1].text()).toContain('Symfonium')
        expect(groups[1].text()).not.toContain('aether-web')
        currentUser.value = null
        mockTokens.value = []
    })

    it('shows the plaintext exactly once after creation', async () => {
        currentUser.value = { login: 'alice', role: 'user' }
        const w = mountView()
        const nameInput = w.find('#token-name')
        await nameInput.setValue('MyToken')

        // Simulate successful creation
        mockCreateToken.mockImplementation((input: any, options: any) => {
            if (options?.onSuccess) {
                options.onSuccess({
                    tokenId: 't2',
                    name: 'MyToken',
                    createdAt: '2026-01-02T00:00:00Z',
                    token: 'aether_x_y'
                })
            }
        })

        await w.find('.token-create').trigger('submit')
        await w.vm.$nextTick()
        await w.vm.$nextTick() // Extra tick for dialog visibility

        const plaintextEl = w.find('.token-plaintext')
        expect(plaintextEl.exists()).toBe(true)
        expect(plaintextEl.text()).toContain('aether_x_y')
        currentUser.value = null
    })
})
